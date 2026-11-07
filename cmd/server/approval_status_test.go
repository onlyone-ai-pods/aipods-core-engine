package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

func TestApprovalStatusRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, _ := auth.GenerateDevKey()
	st := store.NewMemory()
	guard := &auth.Guard{Mode: auth.ModeEnforce, Issuer: auth.NewIssuer(key, 0), Public: []string{"/api/v1/approvals/"}}
	r := gin.New()
	r.Use(guard.Authenticate())
	registerApprovalStatusRoute(r, st)

	tok := pod.NewApprovalToken()
	ctx := context.Background()
	if err := st.AddApproval(ctx, store.Approval{Token: tok, TenantID: "tenant_acme", PodID: "POD_AFIP_FISCAL", ActionName: "a",
		Summary: "s", Command: "node secret-script.js --cuit=1", Status: "PENDING", RequestedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	legacy := "dryrun_1a2b3c4d"
	_ = st.AddApproval(ctx, store.Approval{Token: legacy, TenantID: "t", PodID: "p", ActionName: "a", Summary: "s", Command: "c", Status: "PENDING", RequestedAt: time.Now()})

	get := func(token string) (int, map[string]any) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/approvals/"+token, nil))
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	code, out := get(tok)
	if code != 200 || out["status"] != "PENDING" {
		t.Fatalf("anonymous requester with the token reads the status: %d %v", code, out)
	}
	for _, leaked := range []string{"command", "tenant_id", "summary", "decided_by"} {
		if _, ok := out[leaked]; ok {
			t.Errorf("SECURITY: status response leaks %q", leaked)
		}
	}
	if _, err := st.DecideApproval(ctx, tok, "APPROVED", "resultado", "admin@x"); err != nil {
		t.Fatal(err)
	}
	if code, out := get(tok); code != 200 || out["status"] != "APPROVED" || out["execution_result"] != "resultado" {
		t.Errorf("decided approval: %d %v", code, out)
	}
	if code, _ := get(legacy); code != 404 {
		t.Errorf("SECURITY: short legacy tokens are guessable and must not be readable, got %d", code)
	}
	if code, _ := get("dryrun_" + strings.Repeat("0", 36)); code != 404 {
		t.Errorf("unknown token: want 404, got %d", code)
	}
}

func TestNewApprovalTokenIsLongAndUnique(t *testing.T) {
	a, b := pod.NewApprovalToken(), pod.NewApprovalToken()
	if a == b || len(a) < minApprovalTokenLen || !strings.HasPrefix(a, "dryrun_") {
		t.Errorf("weak or repeated tokens: %q %q", a, b)
	}
}

func TestNewEngine(t *testing.T) {
	defer gin.SetMode(gin.TestMode)
	env := map[string]string{"ENVIRONMENT": "production"}
	if _, err := newEngine(func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	if gin.Mode() != gin.ReleaseMode {
		t.Errorf("production should run gin in release mode, got %s", gin.Mode())
	}

	gin.SetMode(gin.TestMode)
	env = map[string]string{"TRUSTED_PROXIES": "10.1.0.0/16"}
	r, err := newEngine(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	var ip string
	r.GET("/ip", func(c *gin.Context) { ip = c.ClientIP() })
	req := httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = "203.0.113.9:1234" // not a trusted proxy
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if ip != "203.0.113.9" {
		t.Errorf("SECURITY: untrusted client spoofed its IP via X-Forwarded-For: %s", ip)
	}
	req = httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = "10.1.2.3:1234" // Traefik
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if ip != "198.51.100.7" {
		t.Errorf("trusted proxy should forward the client IP, got %s", ip)
	}

	env = map[string]string{"TRUSTED_PROXIES": "not-an-ip"}
	if _, err := newEngine(func(k string) string { return env[k] }); err == nil {
		t.Error("an invalid TRUSTED_PROXIES must fail at startup")
	}
}
