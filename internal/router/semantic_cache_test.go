package router

import (
	"context"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/vault"
)

// countingLLM counts every LLM call (classification, expansion and drafting).
type countingLLM struct{ calls int }

func (c *countingLLM) Model() string { return "fake-model" }

func (c *countingLLM) Complete(_ context.Context, req llm.Request) (string, error) {
	c.calls++
	if req.Effort == llm.EffortLow {
		return "POD_AFIP_FISCAL", nil
	}
	return "Respuesta redactada [Manual_ARCA.pdf (Pagina 1)].", nil
}

// ownerRetriever returns one chunk owned by the given tenant.
type ownerRetriever struct{ owner string }

func (o ownerRetriever) Search(context.Context, string, int) ([]rag.SearchResult, error) {
	return []rag.SearchResult{{
		Chunk:      rag.DocumentChunk{TenantID: o.owner, Content: "Texto del manual."},
		CitationID: "Manual_ARCA.pdf (Pagina 1)",
	}}, nil
}

func newCachedRouter(t *testing.T, docsOwner string) (*DynamicSmartRouter, *countingLLM) {
	t.Helper()
	v, err := vault.NewNativeVaultManager("0123456789abcdef0123456789abcdef", store.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	cache, err := rag.NewSemanticCacheManager(rag.NewMemoryCacheBackend(), v, time.Hour, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c := &countingLLM{}
	r := NewDynamicSmartRouter()
	r.SetLLM(c)
	r.SetRetriever(ownerRetriever{owner: docsOwner})
	r.SetCache(cache)
	return r, c
}

const generalQuestion = "explicame la normativa general de ganancias"

func TestSPEC_CORE_71_R1_RepeatedQueryDoesNotCallLLM(t *testing.T) {
	ctx := context.Background()
	r, c := newCachedRouter(t, rag.GlobalTenant)

	first, err := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached || first.DryRunResult != nil {
		t.Fatalf("first answer must come from the LLM, got cached=%t dryRun=%v", first.Cached, first.DryRunResult)
	}
	before := c.calls

	start := time.Now()
	second, err := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if c.calls != before {
		t.Errorf("expected 0 LLM calls on a repeated query, got %d", c.calls-before)
	}
	if !second.Cached || second.Answer != first.Answer {
		t.Errorf("expected the cached answer, got cached=%t", second.Cached)
	}
	if elapsed >= 10*time.Millisecond {
		t.Errorf("cache hit took %s, expected < 10ms", elapsed)
	}
}

func TestSPEC_CORE_71_R2_GlobalSourcesAreSharedWithOtherTenants(t *testing.T) {
	ctx := context.Background()
	r, c := newCachedRouter(t, rag.GlobalTenant)
	if _, err := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true); err != nil {
		t.Fatal(err)
	}
	before := c.calls
	res, err := r.RouteAndExecute(ctx, "tenant_b", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cached || res.CacheScope != rag.CacheScopeGlobal || c.calls != before {
		t.Errorf("tenant_b should reuse the GLOBAL answer: cached=%t scope=%s llmCalls=%d", res.Cached, res.CacheScope, c.calls-before)
	}
}

func TestSPEC_CORE_71_R3_PrivateSourcesStayInTenant(t *testing.T) {
	ctx := context.Background()
	r, _ := newCachedRouter(t, "tenant_a")
	if _, err := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true); err != nil {
		t.Fatal(err)
	}
	res, err := r.RouteAndExecute(ctx, "tenant_b", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Cached {
		t.Fatal("SECURITY VIOLATION: an answer built from tenant_a documents was served to tenant_b")
	}
	own, _ := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true)
	if !own.Cached || own.CacheScope != rag.CacheScopeTenant {
		t.Errorf("tenant_a should hit its TENANT entry, got cached=%t scope=%s", own.Cached, own.CacheScope)
	}
}

func TestSPEC_CORE_71_R3_ActionsAreNeverCached(t *testing.T) {
	ctx := context.Background()
	r, c := newCachedRouter(t, rag.GlobalTenant)
	q := "descargar mis retenciones sicore"
	if _, err := r.RouteAndExecute(ctx, "tenant_a", q, true); err != nil {
		t.Fatal(err)
	}
	before := c.calls
	res, err := r.RouteAndExecute(ctx, "tenant_a", q, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Cached || c.calls == before {
		t.Error("an answer with a dry-run action must not be cached")
	}
}

// leakyLLM drafts an answer that names the tenant: it must stay private.
type leakyLLM struct{ countingLLM }

func (l *leakyLLM) Complete(ctx context.Context, req llm.Request) (string, error) {
	if req.Effort == llm.EffortLow {
		return l.countingLLM.Complete(ctx, req)
	}
	return "Para la empresa TENANT_A la normativa dice...", nil
}

func TestSPEC_CORE_71_R3_AnswerNamingTheTenantStaysPrivate(t *testing.T) {
	ctx := context.Background()
	r, _ := newCachedRouter(t, rag.GlobalTenant)
	r.SetLLM(&leakyLLM{})
	if _, err := r.RouteAndExecute(ctx, "tenant_a", generalQuestion, true); err != nil {
		t.Fatal(err)
	}
	res, err := r.RouteAndExecute(ctx, "tenant_b", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Cached {
		t.Fatal("SECURITY VIOLATION: an answer naming tenant_a was shared through GLOBAL")
	}
}
