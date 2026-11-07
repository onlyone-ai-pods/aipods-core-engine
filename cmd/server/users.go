package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// User administration and password changes (SPEC-CORE-67.R11-R13).

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

type SetRolesRequest struct {
	Roles []string `json:"roles" binding:"required"`
}

type SetActiveRequest struct {
	Active *bool `json:"active" binding:"required"`
}

var (
	errHiddenUser = errors.New("user not found")
	errStaffUser  = errors.New("only a platform_admin can manage staff users")
)

// canManage says whether the caller may change the target user.
// A tenant_admin only sees users of its own tenant (others look like 404)
// and cannot touch staff accounts (platform_admin, auditor).
func canManage(cl *auth.Claims, target store.User) error {
	if cl.HasRole(auth.RolePlatformAdmin) {
		return nil
	}
	if !cl.HasRole(auth.RoleTenantAdmin) || target.TenantID != cl.TenantID {
		return errHiddenUser
	}
	for _, r := range target.Roles {
		if r == auth.RolePlatformAdmin || r == auth.RoleAuditor {
			return errStaffUser
		}
	}
	return nil
}

func requireEnforced(guard *auth.Guard, c *gin.Context) bool {
	if !guard.Enforced() {
		c.JSON(http.StatusForbidden, gin.H{"error": "user management requires AUTH_MODE=enforce"})
		return false
	}
	return true
}

func registerUserRoutes(r *gin.Engine, st store.Store, guard *auth.Guard, audit auditor) {
	// Any signed-in user changes its own password (R11).
	r.POST("/api/v1/auth/password", func(c *gin.Context) {
		if !requireEnforced(guard, c) {
			return
		}
		var req ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cl := auth.ClaimsFrom(c)
		u, err := st.GetUser(c.Request.Context(), cl.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load user"})
			return
		}
		if ok, _ := auth.VerifyPassword(req.CurrentPassword, u.PasswordHash); !ok {
			audit.record(c, u.TenantID, "auth.password_change", u.Email, "denied")
			// 400, not 401: a wrong current password must not end the session.
			c.JSON(http.StatusBadRequest, gin.H{"error": "la contraseña actual no es correcta"})
			return
		}
		if req.NewPassword == req.CurrentPassword {
			c.JSON(http.StatusBadRequest, gin.H{"error": "la contraseña nueva tiene que ser distinta"})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := st.UpdateUserPassword(c.Request.Context(), u.ID, hash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update password"})
			return
		}
		audit.record(c, u.TenantID, "auth.password_change", u.Email, "success")
		// The change revoked every token of this user, including the one in use
		// (R15): hand back a fresh one so this session stays open.
		u, err = st.GetUser(c.Request.Context(), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load user"})
			return
		}
		token, exp, err := guard.Issuer.Issue(u.ID, u.Email, u.TenantID, u.Roles, u.TokenVersion)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not issue token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":       "OK",
			"access_token": token,
			"token_type":   "Bearer",
			"expires_at":   exp,
			"user":         gin.H{"id": u.ID, "email": u.Email, "tenant_id": u.TenantID, "roles": u.Roles},
		})
	})

	// List users (R12): platform_admin and auditor see every tenant, tenant_admin its own.
	r.GET("/api/v1/admin/users", guard.RequireRoles(auth.RolePlatformAdmin, auth.RoleAuditor, auth.RoleTenantAdmin), func(c *gin.Context) {
		if !requireEnforced(guard, c) {
			return
		}
		tenantID := c.Query("tenant_id")
		if !guard.CrossTenant(c) {
			var ok bool
			if tenantID, ok = guard.ResolveTenant(c, tenantID, "", false); !ok {
				return
			}
		}
		users, err := st.ListUsers(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "OK", "users": users})
	})

	manage := guard.RequireRoles(auth.RolePlatformAdmin, auth.RoleTenantAdmin)

	// loadTarget returns the user an admin acts on, or writes the error.
	loadTarget := func(c *gin.Context) (store.User, bool) {
		cl := auth.ClaimsFrom(c)
		if c.Param("id") == cl.UserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "usá Cambiar contraseña para tu propio usuario; no podés desactivarte"})
			return store.User{}, false
		}
		u, err := st.GetUser(c.Request.Context(), c.Param("id"))
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errHiddenUser.Error()})
			return store.User{}, false
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return store.User{}, false
		}
		switch err := canManage(cl, u); {
		case errors.Is(err, errHiddenUser):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return store.User{}, false
		case err != nil:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return store.User{}, false
		}
		return u, true
	}

	// Activate or deactivate a user (R13). A deactivated user cannot log in
	// and its open sessions stop working (Guard.UserState).
	r.PATCH("/api/v1/admin/users/:id", manage, func(c *gin.Context) {
		if !requireEnforced(guard, c) {
			return
		}
		var req SetActiveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		u, ok := loadTarget(c)
		if !ok {
			return
		}
		if err := st.SetUserActive(c.Request.Context(), u.ID, *req.Active); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		action := "user.deactivate"
		if *req.Active {
			action = "user.activate"
		}
		audit.record(c, u.TenantID, action, u.Email, "success")
		u.Active = *req.Active
		c.JSON(http.StatusOK, gin.H{"status": "OK", "user": u})
	})

	// An admin sets a new password for someone who forgot theirs (R13).
	r.POST("/api/v1/admin/users/:id/password", manage, func(c *gin.Context) {
		if !requireEnforced(guard, c) {
			return
		}
		var req ResetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		u, ok := loadTarget(c)
		if !ok {
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := st.UpdateUserPassword(c.Request.Context(), u.ID, hash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		audit.record(c, u.TenantID, "user.password_reset", u.Email, "success")
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	// Change a user's roles (R16). A tenant_admin can only hand out customer
	// and tenant_admin. The user's open sessions end because the roles live
	// in the token.
	r.PATCH("/api/v1/admin/users/:id/roles", manage, func(c *gin.Context) {
		if !requireEnforced(guard, c) {
			return
		}
		var req SetRolesRequest
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Roles) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "roles must list at least one role"})
			return
		}
		cl := auth.ClaimsFrom(c)
		for _, role := range req.Roles {
			if !auth.ValidRole(role) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown role " + role})
				return
			}
			if (role == auth.RolePlatformAdmin || role == auth.RoleAuditor) && !cl.HasRole(auth.RolePlatformAdmin) {
				c.JSON(http.StatusForbidden, gin.H{"error": "cannot grant role " + role})
				return
			}
		}
		u, ok := loadTarget(c)
		if !ok {
			return
		}
		if err := st.SetUserRoles(c.Request.Context(), u.ID, req.Roles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		audit.record(c, u.TenantID, "user.roles", u.Email+" -> "+strings.Join(req.Roles, ","), "success")
		u.Roles = req.Roles
		c.JSON(http.StatusOK, gin.H{"status": "OK", "user": u})
	})
}
