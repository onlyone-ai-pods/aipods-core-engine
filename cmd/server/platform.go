package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// buildStore opens Postgres when DATABASE_URL is set (SPEC-CORE-67.R5).
// A configured database that cannot be reached is fatal: silently falling back
// to memory would lose approvals and secrets.
func buildStore(ctx context.Context, getenv func(string) string) (store.Store, error) {
	url := getenv("DATABASE_URL")
	if url == "" {
		log.Println("WARNING: DATABASE_URL not set; approvals, vault and audit live in memory and are lost on restart")
		return store.NewMemory(), nil
	}
	return store.OpenPostgres(ctx, url)
}

// buildGuard reads AUTH_MODE and AUTH_SIGNING_KEY (SPEC-CORE-67 §2, R4).
func buildGuard(getenv func(string) string) (*auth.Guard, error) {
	mode := auth.Mode(strings.ToLower(strings.TrimSpace(getenv("AUTH_MODE"))))
	if mode == "" {
		mode = auth.ModeOff
	}
	if mode != auth.ModeOff && mode != auth.ModeEnforce {
		return nil, fmt.Errorf("AUTH_MODE must be %q or %q", auth.ModeOff, auth.ModeEnforce)
	}

	var issuer *auth.Issuer
	if pemKey := getenv("AUTH_SIGNING_KEY"); pemKey != "" {
		key, err := auth.ParsePrivateKeyPEM(strings.ReplaceAll(pemKey, `\n`, "\n"))
		if err != nil {
			return nil, err
		}
		issuer = auth.NewIssuer(key, 8*time.Hour)
	} else {
		if mode == auth.ModeEnforce && strings.EqualFold(getenv("ENVIRONMENT"), "production") {
			return nil, errors.New("AUTH_SIGNING_KEY is required when AUTH_MODE=enforce in production")
		}
		log.Println("WARNING: AUTH_SIGNING_KEY not set; using an ephemeral key (tokens die on restart)")
		key, err := auth.GenerateDevKey()
		if err != nil {
			return nil, err
		}
		issuer = auth.NewIssuer(key, 8*time.Hour)
	}
	if mode == auth.ModeOff {
		log.Println("WARNING: AUTH_MODE=off; tenant_id is taken from each request (prototype mode)")
	}

	return &auth.Guard{
		Mode:   mode,
		Issuer: issuer,
		Public: []string{"/healthz", "/metrics", "/api/v1/auth/login", "/api/v1/sandbox/"},
	}, nil
}

// bootstrapAdmin creates the first platform_admin when the users table is empty.
func bootstrapAdmin(ctx context.Context, st store.Store, getenv func(string) string) error {
	email, password := strings.TrimSpace(getenv("AUTH_BOOTSTRAP_EMAIL")), getenv("AUTH_BOOTSTRAP_PASSWORD")
	if email == "" || password == "" {
		return nil
	}
	n, err := st.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.CreateUser(ctx, store.User{
		ID: uuid.NewString(), TenantID: "GLOBAL", Email: email, PasswordHash: hash,
		Roles: []string{auth.RolePlatformAdmin}, Active: true,
	}); err != nil {
		return err
	}
	log.Printf("auth: created bootstrap platform_admin %s", email)
	return nil
}

// auditor writes audit events and never fails the request because of them.
type auditor struct {
	st    store.Store
	guard *auth.Guard
}

func (a auditor) record(c *gin.Context, tenantID, action, target, outcome string) {
	a.recordAs(c, a.guard.Actor(c), tenantID, action, target, outcome)
}

func (a auditor) recordAs(c *gin.Context, actor, tenantID, action, target, outcome string) {
	err := a.st.AppendAudit(c.Request.Context(), store.AuditEvent{
		TenantID: tenantID, Actor: actor, Action: action, Target: target, Outcome: outcome,
		ClientIP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		log.Printf("audit: failed to record %s: %v", action, err)
	}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CreateUserRequest struct {
	TenantID   string   `json:"tenant_id" binding:"required"`
	TenantName string   `json:"tenant_name"`
	Email      string   `json:"email" binding:"required"`
	Password   string   `json:"password" binding:"required"`
	Roles      []string `json:"roles" binding:"required"`
}

func registerAuthRoutes(r *gin.Engine, st store.Store, guard *auth.Guard, audit auditor) {
	// Login (SPEC-CORE-67.R4). Same error for unknown user and wrong password.
	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := strings.TrimSpace(req.Email)
		u, err := st.UserByEmail(c.Request.Context(), email)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login unavailable"})
			return
		}
		ok := false
		if err == nil && u.Active {
			ok, _ = auth.VerifyPassword(req.Password, u.PasswordHash)
		} else {
			auth.VerifyDummy(req.Password)
		}
		if !ok {
			audit.recordAs(c, email, "GLOBAL", "auth.login", email, "denied")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciales inválidas"})
			return
		}
		token, exp, err := guard.Issuer.Issue(u.ID, u.Email, u.TenantID, u.Roles)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue token"})
			return
		}
		audit.recordAs(c, u.Email, u.TenantID, "auth.login", u.Email, "success")
		c.JSON(http.StatusOK, gin.H{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_at":   exp,
			"user":         gin.H{"id": u.ID, "email": u.Email, "tenant_id": u.TenantID, "roles": u.Roles},
		})
	})

	r.GET("/api/v1/auth/me", func(c *gin.Context) {
		cl := auth.ClaimsFrom(c)
		if cl == nil {
			c.JSON(http.StatusOK, gin.H{"auth_mode": guard.Mode, "authenticated": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"auth_mode": guard.Mode, "authenticated": true,
			"user": gin.H{"id": cl.UserID, "email": cl.Email, "tenant_id": cl.TenantID, "roles": cl.Roles}})
	})

	// User management: platform_admin for any tenant, tenant_admin for their own.
	r.POST("/api/v1/admin/users", guard.RequireRoles(auth.RolePlatformAdmin, auth.RoleTenantAdmin), func(c *gin.Context) {
		if !guard.Enforced() {
			c.JSON(http.StatusForbidden, gin.H{"error": "user management requires AUTH_MODE=enforce"})
			return
		}
		var req CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "", false)
		if !ok {
			return
		}
		cl := auth.ClaimsFrom(c)
		for _, role := range req.Roles {
			if !auth.ValidRole(role) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown role " + role})
				return
			}
			// Only platform admins can grant staff roles.
			if (role == auth.RolePlatformAdmin || role == auth.RoleAuditor) && !cl.HasRole(auth.RolePlatformAdmin) {
				c.JSON(http.StatusForbidden, gin.H{"error": "cannot grant role " + role})
				return
			}
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		name := req.TenantName
		if name == "" {
			name = tenantID
		}
		if cl.HasRole(auth.RolePlatformAdmin) {
			if err := st.EnsureTenant(c.Request.Context(), tenantID, name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		u := store.User{ID: uuid.NewString(), TenantID: tenantID, Email: strings.TrimSpace(req.Email),
			PasswordHash: hash, Roles: req.Roles, Active: true}
		switch err := st.CreateUser(c.Request.Context(), u); {
		case errors.Is(err, store.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tenant"})
			return
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		audit.record(c, tenantID, "user.create", u.Email, "success")
		c.JSON(http.StatusCreated, gin.H{"user": gin.H{"id": u.ID, "email": u.Email, "tenant_id": u.TenantID, "roles": u.Roles}})
	})

	r.GET("/api/v1/admin/audit/log", guard.RequireRoles(auth.RoleAuditor, auth.RolePlatformAdmin), func(c *gin.Context) {
		tenantID := c.Query("tenant_id")
		events, err := st.ListAudit(c.Request.Context(), tenantID, 200)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "OK", "events": events})
	})
}

func outcome(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func databaseStatus(ctx context.Context, st store.Store) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := st.Ping(ctx); err != nil {
		return st.Name() + ":down"
	}
	return st.Name()
}

// recordApproval persists a pending human approval produced by a pod dry-run.
func recordApproval(c *gin.Context, st store.Store, res *pod.PodResponse, tenantID string) {
	dr := res.DryRunResult
	if dr == nil || !dr.RequiresHumanApproval || dr.ApprovalToken == "" {
		return
	}
	err := st.AddApproval(c.Request.Context(), store.Approval{
		Token: dr.ApprovalToken, PodID: res.PodID, ActionName: dr.ActionName, Summary: dr.Summary,
		Command: dr.GeneratedCommand, TenantID: tenantID, Status: "PENDING", RequestedAt: time.Now(),
	})
	if err != nil {
		log.Printf("approvals: failed to store %s: %v", dr.ApprovalToken, err)
	}
}
