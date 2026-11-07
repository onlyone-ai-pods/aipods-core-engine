package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// minApprovalTokenLen rejects the old 8-hex-char tokens, which were guessable.
const minApprovalTokenLen = len("dryrun_") + 36

// registerApprovalStatusRoute lets whoever requested an action see whether it
// was approved (SPEC-CORE-67.R14). The token is the capability: it is only
// shown to the requester and has 122 random bits. The route is public so the
// anonymous sandbox works too, and it returns only the decision, not the
// command or the tenant.
func registerApprovalStatusRoute(r *gin.Engine, st store.Store) {
	r.GET("/api/v1/approvals/:token", func(c *gin.Context) {
		token := c.Param("token")
		if len(token) < minApprovalTokenLen || !strings.HasPrefix(token, "dryrun_") {
			c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
			return
		}
		a, err := st.GetApproval(c.Request.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read approval"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"token":            a.Token,
			"status":           a.Status,
			"execution_result": a.ExecutionResult,
			"decided_at":       a.DecidedAt,
		})
	})
}

// newEngine builds the gin engine: release mode in production unless GIN_MODE
// says otherwise, and only private networks trusted to set X-Forwarded-For
// (Traefik runs on the Docker network), so the audit log records the real
// client IP and a client cannot spoof it.
func newEngine(getenv func(string) string) (*gin.Engine, error) {
	if getenv("GIN_MODE") == "" && strings.EqualFold(getenv("ENVIRONMENT"), "production") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	proxies := []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	if v := strings.TrimSpace(getenv("TRUSTED_PROXIES")); v != "" {
		proxies = nil
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				proxies = append(proxies, p)
			}
		}
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		return nil, err
	}
	return r, nil
}
