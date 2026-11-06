package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func testIssuer(t *testing.T) *Issuer {
	t.Helper()
	k, err := GenerateDevKey()
	if err != nil {
		t.Fatal(err)
	}
	return NewIssuer(k, time.Hour)
}

func TestIssueAndVerify(t *testing.T) {
	iss := testIssuer(t)
	tok, _, err := iss.Issue("u1", "ana@acme.com", "tenant_acme", []string{RoleCustomer})
	if err != nil {
		t.Fatal(err)
	}
	cl, err := iss.Verify(tok)
	if err != nil || cl.TenantID != "tenant_acme" || !cl.HasRole(RoleCustomer) {
		t.Fatalf("verify: %+v %v", cl, err)
	}

	// Token signed by another key is rejected.
	other := testIssuer(t)
	if _, err := other.Verify(tok); err == nil {
		t.Error("token from another key must be rejected")
	}
	// HS256 with the public key as secret (alg confusion) is rejected.
	hs, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{TenantID: "x", UserID: "y",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte("secret"))
	if _, err := iss.Verify(hs); err == nil {
		t.Error("HS256 token must be rejected")
	}
	// Expired token is rejected.
	expired := NewIssuer(iss.key, -time.Hour)
	expired.ttl = -time.Minute
	old, _, _ := expired.Issue("u1", "a", "t", nil)
	if _, err := iss.Verify(old); err == nil {
		t.Error("expired token must be rejected")
	}
}

func TestPasswordHashing(t *testing.T) {
	if _, err := HashPassword("corta"); err == nil {
		t.Error("short password must be rejected")
	}
	h, err := HashPassword("una-clave-bien-larga")
	if err != nil || !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("hash: %q %v", h, err)
	}
	if ok, _ := VerifyPassword("una-clave-bien-larga", h); !ok {
		t.Error("correct password rejected")
	}
	if ok, _ := VerifyPassword("otra-clave-bien-larga", h); ok {
		t.Error("wrong password accepted")
	}
}

func newRouter(g *Guard) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(g.Authenticate())
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/api/v1/chat", func(c *gin.Context) {
		tn, ok := g.ResolveTenant(c, c.Query("tenant_id"), "DEMO", false)
		if !ok {
			return
		}
		c.String(200, tn)
	})
	r.GET("/api/v1/admin/approvals", g.RequireRoles(RoleAuditor, RolePlatformAdmin), func(c *gin.Context) { c.String(200, "admin") })
	return r
}

func do(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGuardEnforce(t *testing.T) {
	iss := testIssuer(t)
	g := &Guard{Mode: ModeEnforce, Issuer: iss, Public: []string{"/healthz"}}
	r := newRouter(g)
	customer, _, _ := iss.Issue("u1", "ana@acme.com", "tenant_acme", []string{RoleCustomer})
	admin, _, _ := iss.Issue("u2", "root@aipods.ar", "GLOBAL", []string{RolePlatformAdmin})

	cases := []struct {
		name, path, token string
		code              int
		body              string
	}{
		{"public path", "/healthz", "", 200, "ok"},
		{"no token", "/api/v1/chat", "", 401, ""},
		{"bad token", "/api/v1/chat", "garbage", 401, ""},
		{"tenant from token", "/api/v1/chat", customer, 200, "tenant_acme"},
		{"same tenant in query", "/api/v1/chat?tenant_id=tenant_acme", customer, 200, "tenant_acme"},
		{"other tenant in query", "/api/v1/chat?tenant_id=tenant_otro", customer, 403, ""},
		{"platform admin may pick tenant", "/api/v1/chat?tenant_id=tenant_otro", admin, 200, "tenant_otro"},
		{"customer on admin route", "/api/v1/admin/approvals", customer, 403, ""},
		{"admin on admin route", "/api/v1/admin/approvals", admin, 200, "admin"},
	}
	for _, tc := range cases {
		w := do(r, tc.path, tc.token)
		if w.Code != tc.code || (tc.body != "" && w.Body.String() != tc.body) {
			t.Errorf("%s: got %d %q, want %d %q", tc.name, w.Code, w.Body.String(), tc.code, tc.body)
		}
	}
}

func TestGuardOffKeepsPrototypeBehaviour(t *testing.T) {
	g := &Guard{Mode: ModeOff}
	r := newRouter(g)
	if w := do(r, "/api/v1/chat?tenant_id=tenant_x", ""); w.Code != 200 || w.Body.String() != "tenant_x" {
		t.Errorf("off mode should use the requested tenant: %d %q", w.Code, w.Body.String())
	}
	if w := do(r, "/api/v1/chat", ""); w.Body.String() != "DEMO" {
		t.Errorf("off mode should use the fallback tenant, got %q", w.Body.String())
	}
	if w := do(r, "/api/v1/admin/approvals", ""); w.Code != 200 {
		t.Errorf("off mode should not check roles, got %d", w.Code)
	}
}
