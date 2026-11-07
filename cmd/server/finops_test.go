package main

import (
	"context"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

func TestEstimateCost(t *testing.T) {
	usd, ok := estimateCost(store.LLMUsageTotal{Model: "claude-opus-5-5", InputTokens: 1_000_000, OutputTokens: 100_000, CacheReadTokens: 1_000_000})
	if !ok || roundUSD(usd) != 6.2 { // 4 + 2 + 0.20
		t.Errorf("opus 5.5 cost = %v %v, want 6.2", usd, ok)
	}
	if _, ok := estimateCost(store.LLMUsageTotal{Model: "modelo-desconocido"}); ok {
		t.Error("unknown model must not be priced")
	}
}

func TestFinOpsUsage(t *testing.T) {
	api := newUserAPI(t)
	registerFinOpsRoutes(api.r, api.st, api.guard)
	root := api.login("root@aipods.ar", "root-password-123")
	api.create(root, "tenant_acme", "jefa@acme.test", auth.RoleTenantAdmin)
	api.create(root, "tenant_acme", "ana@acme.test", auth.RoleCustomer)

	rec := recordLLMUsage(api.st)
	ctx := llm.WithTenant(context.Background(), "tenant_acme")
	rec(ctx, llm.Usage{TenantID: "tenant_acme", Model: "claude-opus-5-5", Purpose: "classify", InputTokens: 500_000, OutputTokens: 1000})
	rec(ctx, llm.Usage{TenantID: "tenant_acme", Model: "claude-opus-5-5", Purpose: "answer", InputTokens: 500_000, OutputTokens: 99_000})
	rec(ctx, llm.Usage{TenantID: "tenant_otro", Model: "modelo-nuevo", InputTokens: 10, OutputTokens: 5})
	rec(context.Background(), llm.Usage{Model: "claude-opus-5-5", InputTokens: 1})
	_ = api.st.RecordLLMUsage(context.Background(), store.LLMUsage{TenantID: "tenant_acme", Model: "claude-opus-5-5", InputTokens: 9_999_999, CreatedAt: time.Now().AddDate(0, 0, -40)})

	code, out := api.call("GET", "/api/v1/admin/finops/usage", root, nil)
	if code != 200 {
		t.Fatalf("platform_admin: %d %v", code, out)
	}
	byID := map[string]map[string]any{}
	for _, raw := range out["tenants"].([]any) {
		m := raw.(map[string]any)
		byID[m["tenant_id"].(string)] = m
	}
	if len(byID) != 3 || byID[unattributedTenant] == nil {
		t.Fatalf("want acme, otro and calls without tenant, got %v", out["tenants"])
	}
	acme := byID["tenant_acme"]
	// 1M input * $4 + 100k output * $20 = $6; the 40-day-old row is outside the window.
	if acme["requests"].(float64) != 2 || acme["estimated_cost_usd"].(float64) != 6 {
		t.Errorf("acme totals: %v", acme)
	}
	if u := byID["tenant_otro"]["unpriced_models"].([]any); len(u) != 1 || u[0] != "modelo-nuevo" {
		t.Errorf("unknown model must be listed as unpriced: %v", byID["tenant_otro"])
	}

	jefa := api.login("jefa@acme.test", "initial-pass-123")
	if code, _ := api.call("GET", "/api/v1/admin/finops/usage?tenant_id=tenant_otro", jefa, nil); code != 403 {
		t.Errorf("tenant_admin cannot read another tenant, got %d", code)
	}
	code, out = api.call("GET", "/api/v1/admin/finops/usage?days=90", jefa, nil)
	if code != 200 || len(out["tenants"].([]any)) != 1 {
		t.Fatalf("tenant_admin sees only its tenant: %d %v", code, out)
	}
	if code, _ := api.call("GET", "/api/v1/admin/finops/usage?days=0", root, nil); code != 400 {
		t.Errorf("days=0 must be rejected, got %d", code)
	}
	ana := api.login("ana@acme.test", "initial-pass-123")
	if code, _ := api.call("GET", "/api/v1/admin/finops/usage", ana, nil); code != 403 {
		t.Errorf("customer cannot read usage, got %d", code)
	}
}
