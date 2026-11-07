package main

import (
	"context"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// SPEC-CORE-70.R3: only a platform_admin changes a tenant's RAG strategy,
// the change is audited and the router resolver sees it.
func TestRAGStrategyAPI(t *testing.T) {
	api := newUserAPI(t)
	root := api.login("root@aipods.ar", "root-password-123")
	api.create(root, "tenant_acme", "jefa@acme.test", auth.RoleTenantAdmin)
	jefa := api.login("jefa@acme.test", "initial-pass-123")

	const path = "/api/v1/admin/tenants/tenant_acme/rag-strategy"
	if code, out := api.call("GET", path, root, nil); code != 200 || out["strategy"] != "query_expansion" {
		t.Fatalf("default: %d %v", code, out)
	}
	if code, _ := api.call("PUT", path, jefa, map[string]string{"strategy": "external_embeddings"}); code != 403 {
		t.Errorf("tenant_admin must get 403, got %d", code)
	}
	if code, _ := api.call("PUT", path, root, map[string]string{"strategy": "bogus"}); code != 400 {
		t.Errorf("invalid strategy: want 400, got %d", code)
	}
	if code, _ := api.call("PUT", "/api/v1/admin/tenants/NOPE/rag-strategy", root, map[string]string{"strategy": "query_expansion"}); code != 404 {
		t.Errorf("unknown tenant: want 404, got %d", code)
	}
	if code, out := api.call("PUT", path, root, map[string]string{"strategy": "external_embeddings"}); code != 200 {
		t.Fatalf("set: %d %v", code, out)
	}

	resolve := storeStrategyResolver(api.st)
	if got := resolve(context.Background(), "tenant_acme"); got != tenant.RAGExternalEmbeddings {
		t.Errorf("resolver = %q", got)
	}
	if got := resolve(context.Background(), "TENANT_DEMO_001"); got != tenant.DefaultRAGStrategy {
		t.Errorf("tenant without row = %q", got)
	}

	events, err := api.st.ListAudit(context.Background(), "tenant_acme", 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == "rag_strategy.set" && e.Target == "external_embeddings" && e.Outcome == "success" {
			found = true
		}
	}
	if !found {
		t.Errorf("strategy change not audited: %+v", events)
	}
}
