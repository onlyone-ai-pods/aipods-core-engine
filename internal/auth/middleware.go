package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// Mode controls whether authentication is enforced (SPEC-CORE-67 §2).
type Mode string

const (
	ModeOff     Mode = "off"     // prototype behaviour: tenant comes from the request
	ModeEnforce Mode = "enforce" // tenant and roles come from the JWT
)

const claimsKey = "aipods_claims"

// Guard applies authentication and authorization to gin routes.
type Guard struct {
	Mode   Mode
	Issuer *Issuer
	// Public paths skip authentication (exact match or prefix ending in "/").
	Public []string
	// UserState, when set, is checked on every authenticated request: a
	// deactivated user (SPEC-CORE-67.R13) or a token older than the user's last
	// password change (R15) loses access before the token expires.
	UserState func(ctx context.Context, userID string) (active bool, tokenVersion int, err error)
}

func (g *Guard) Enforced() bool { return g.Mode == ModeEnforce }

func (g *Guard) isPublic(path string) bool {
	for _, p := range g.Public {
		if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}

// Authenticate validates the Bearer token on every non-public route when enforced.
func (g *Guard) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !g.Enforced() || g.isPublic(c.Request.URL.Path) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuthHeader.Error()})
			return
		}
		claims, err := g.Issuer.Verify(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidToken.Error()})
			return
		}
		if g.UserState != nil {
			active, version, err := g.UserState(c.Request.Context(), claims.UserID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "could not verify user"})
				return
			}
			if !active || version != claims.TokenVersion {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidToken.Error()})
				return
			}
		}
		c.Set(claimsKey, claims)
		c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), claims.TenantID))
		c.Next()
	}
}

// ClaimsFrom returns the authenticated claims, or nil.
func ClaimsFrom(c *gin.Context) *Claims {
	if v, ok := c.Get(claimsKey); ok {
		if cl, ok := v.(*Claims); ok {
			return cl
		}
	}
	return nil
}

// RequireRoles allows the request only if the user has one of the roles.
// It is a no-op when authentication is not enforced.
func (g *Guard) RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !g.Enforced() {
			c.Next()
			return
		}
		cl := ClaimsFrom(c)
		if cl == nil || !cl.HasRole(roles...) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
			return
		}
		c.Next()
	}
}

// ResolveTenant returns the tenant a request may act on (SPEC-CORE-67.R2).
//   - not enforced: the requested tenant, or fallback if empty.
//   - enforced: the token's tenant; a different requested tenant is only
//     allowed for platform_admin (and auditor when allowAuditor is true).
//
// ok=false means the handler must stop: a 403 has already been written.
func (g *Guard) ResolveTenant(c *gin.Context, requested, fallback string, allowAuditor bool) (string, bool) {
	if !g.Enforced() {
		if requested == "" {
			return fallback, true
		}
		return requested, true
	}
	cl := ClaimsFrom(c)
	if cl == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuthHeader.Error()})
		return "", false
	}
	if requested == "" || requested == cl.TenantID {
		return cl.TenantID, true
	}
	if cl.HasRole(RolePlatformAdmin) || (allowAuditor && cl.HasRole(RoleAuditor)) {
		return requested, true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant_id does not match the authenticated tenant"})
	return "", false
}

// CrossTenant reports whether the caller may see every tenant (list endpoints).
func (g *Guard) CrossTenant(c *gin.Context) bool {
	if !g.Enforced() {
		return true
	}
	cl := ClaimsFrom(c)
	return cl != nil && cl.HasRole(RolePlatformAdmin, RoleAuditor)
}

// Actor names the caller for the audit log.
func (g *Guard) Actor(c *gin.Context) string {
	if cl := ClaimsFrom(c); cl != nil {
		return cl.Email
	}
	return "anonymous"
}
