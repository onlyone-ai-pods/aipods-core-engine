package router

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
)

// modelLLM records the model of every call and fails the models in down.
type modelLLM struct {
	mu    sync.Mutex
	calls []string // "<kind>:<model>", kind = classify | expand | answer
	down  map[string]error
}

func (m *modelLLM) Model() string { return "claude-opus-5-5" }

func (m *modelLLM) Complete(_ context.Context, req llm.Request) (string, error) {
	kind := "answer"
	switch {
	case req.MaxTokens == 2000:
		kind = "classify"
	case req.MaxTokens == 1000:
		kind = "expand"
	}
	m.mu.Lock()
	m.calls = append(m.calls, kind+":"+req.Model)
	err := m.down[req.Model]
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	switch kind {
	case "classify":
		return "POD_AFIP_FISCAL", nil
	case "expand":
		return "Factura E", nil
	}
	return "Respuesta redactada [Manual_ARCA.pdf (Pagina 1)].", nil
}

type saturated struct{}

func (saturated) Error() string   { return "529 overloaded" }
func (saturated) HTTPStatus() int { return 529 }

// lexicalOwner returns one chunk owned by the given tenant.
type lexicalOwner struct{ owner string }

func (l lexicalOwner) SearchLexical(context.Context, []string, int) ([]rag.SearchResult, error) {
	return []rag.SearchResult{{
		Chunk:      rag.DocumentChunk{TenantID: l.owner, Content: "Texto del manual."},
		CitationID: "Manual_ARCA.pdf (Pagina 1)",
	}}, nil
}

func newModelRouter(t *testing.T, rt *llm.Runtime, resolver ModelResolver) (*DynamicSmartRouter, *modelLLM, *countingResolver) {
	t.Helper()
	r, _ := newCachedRouter(t, rag.GlobalTenant)
	m := &modelLLM{down: map[string]error{}}
	r.SetLLM(m)
	r.SetRetriever(nil)
	r.SetLexicalRetriever(lexicalOwner{owner: rag.GlobalTenant})
	cr := &countingResolver{inner: resolver, defaults: rt.Defaults}
	r.SetLLMRuntime(rt, cr.resolve)
	return r, m, cr
}

type countingResolver struct {
	mu       sync.Mutex
	n        int
	inner    ModelResolver
	defaults llm.Models
}

func (c *countingResolver) resolve(ctx context.Context, tenantID string) llm.Models {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	if c.inner != nil {
		return c.inner(ctx, tenantID)
	}
	return c.defaults
}

func TestSPEC_CORE_74_R1_EachTaskUsesItsModel(t *testing.T) {
	rt := &llm.Runtime{Defaults: llm.Models{Classify: "claude-haiku-5-5", Expand: "claude-haiku-5-5"}}
	r, m, _ := newModelRouter(t, rt, nil)

	res, err := r.RouteAndExecute(context.Background(), "TENANT_A", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"classify:claude-haiku-5-5", "expand:claude-haiku-5-5", "answer:claude-opus-5-5"}
	if !reflect.DeepEqual(m.calls, want) {
		t.Fatalf("calls = %v, want %v", m.calls, want)
	}
	if res.Model != "claude-opus-5-5" {
		t.Fatalf("model = %q", res.Model)
	}
}

func TestSPEC_CORE_74_R1_WithoutRuntimeNothingChanges(t *testing.T) {
	r, _ := newCachedRouter(t, rag.GlobalTenant)
	m := &modelLLM{}
	r.SetLLM(m)
	res, err := r.RouteAndExecute(context.Background(), "TENANT_A", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.calls {
		if c != "classify:" && c != "answer:" {
			t.Fatalf("calls = %v: no per-call model expected without a runtime", m.calls)
		}
	}
	if res.Model != "claude-opus-5-5" {
		t.Fatalf("model = %q, want the base client's", res.Model)
	}
}

func TestSPEC_CORE_74_R6_TenantModelsOverridePlatform(t *testing.T) {
	rt := &llm.Runtime{Defaults: llm.Models{Classify: "claude-haiku-5-5"}}
	resolver := func(_ context.Context, tenantID string) llm.Models {
		if tenantID == "ACME" {
			return llm.Models{Classify: "claude-haiku-5-5", Answer: "claude-sonnet-5-5"}
		}
		return rt.Defaults
	}
	r, m, _ := newModelRouter(t, rt, resolver)

	acme, err := r.RouteAndExecute(context.Background(), "ACME", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if acme.Model != "claude-sonnet-5-5" {
		t.Fatalf("ACME model = %q", acme.Model)
	}
	m.calls = nil
	other, err := r.RouteAndExecute(context.Background(), "OTHER", "otra consulta sobre ganancias", true)
	if err != nil {
		t.Fatal(err)
	}
	if other.Model != "claude-opus-5-5" {
		t.Fatalf("other tenant model = %q, want the platform's", other.Model)
	}
}

func TestSPEC_CORE_74_R7_TenantConfigIsReadOncePerMissAndNeverOnHit(t *testing.T) {
	rt := &llm.Runtime{}
	r, _, cr := newModelRouter(t, rt, nil)
	ctx := context.Background()

	if _, err := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true); err != nil {
		t.Fatal(err)
	}
	if cr.n != 1 {
		t.Fatalf("cache miss read the tenant config %d times, want 1", cr.n)
	}
	hit, err := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true)
	if err != nil || !hit.Cached {
		t.Fatalf("second call must be a cache hit: cached=%v err=%v", hit != nil && hit.Cached, err)
	}
	if cr.n != 1 {
		t.Fatalf("cache hit read the tenant config (%d reads)", cr.n)
	}
}

func TestSPEC_CORE_74_R13_ResponseReportsTheFallbackModel(t *testing.T) {
	var hops []string
	rt := &llm.Runtime{
		Defaults:   llm.Models{Fallback: []string{"claude-sonnet-5-5", "claude-haiku-5-5"}},
		OnFallback: func(task llm.Task, from, to string) { hops = append(hops, string(task)+":"+from+">"+to) },
	}
	r, m, _ := newModelRouter(t, rt, nil)
	m.down["claude-opus-5-5"] = saturated{}

	res, err := r.RouteAndExecute(context.Background(), "TENANT_A", generalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "claude-sonnet-5-5" {
		t.Fatalf("model = %q, want the fallback that answered", res.Model)
	}
	want := []string{
		"classify:claude-opus-5-5>claude-sonnet-5-5",
		"expand:claude-opus-5-5>claude-sonnet-5-5",
		"answer:claude-opus-5-5>claude-sonnet-5-5",
	}
	if !reflect.DeepEqual(hops, want) {
		t.Fatalf("hops = %v", hops)
	}
}

func TestSPEC_CORE_74_R9_ExhaustedChainKeepsThePodAnswer(t *testing.T) {
	rt := &llm.Runtime{Defaults: llm.Models{Fallback: []string{"claude-haiku-5-5"}}}
	r, m, _ := newModelRouter(t, rt, nil)
	m.down["claude-opus-5-5"] = saturated{}
	m.down["claude-haiku-5-5"] = errors.New("connection reset")

	res, err := r.RouteAndExecute(context.Background(), "TENANT_A", generalQuestion, true)
	if err != nil || res == nil {
		t.Fatalf("the chat must still answer: res=%v err=%v", res, err)
	}
	if res.Model != "" || res.Answer == "" {
		t.Fatalf("want the pod's own answer without a model, got model=%q", res.Model)
	}
}

// R14: GLOBAL only takes answers drafted by the platform's answer model.
func TestSPEC_CORE_74_R14_GlobalCacheOnlyFromPlatformModel(t *testing.T) {
	ctx := context.Background()

	t.Run("platform model goes to GLOBAL", func(t *testing.T) {
		r, _, _ := newModelRouter(t, &llm.Runtime{}, nil)
		if _, err := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true); err != nil {
			t.Fatal(err)
		}
		hit, _ := r.RouteAndExecute(ctx, "TENANT_B", generalQuestion, true)
		if !hit.Cached || hit.CacheScope != rag.CacheScopeGlobal {
			t.Fatalf("other tenant: cached=%v scope=%q, want a GLOBAL hit", hit.Cached, hit.CacheScope)
		}
	})

	t.Run("tenant's own model stays in the tenant", func(t *testing.T) {
		rt := &llm.Runtime{}
		resolver := func(_ context.Context, tenantID string) llm.Models {
			if tenantID == "TENANT_A" {
				return llm.Models{Answer: "claude-haiku-5-5"}
			}
			return rt.Defaults
		}
		r, _, _ := newModelRouter(t, rt, resolver)
		if _, err := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true); err != nil {
			t.Fatal(err)
		}
		other, _ := r.RouteAndExecute(ctx, "TENANT_B", generalQuestion, true)
		if other.Cached {
			t.Fatal("an answer drafted by a tenant's own model leaked into GLOBAL")
		}
		own, _ := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true)
		if !own.Cached || own.CacheScope != rag.CacheScopeTenant || own.Model != "claude-haiku-5-5" {
			t.Fatalf("own tenant: cached=%v scope=%q model=%q", own.Cached, own.CacheScope, own.Model)
		}
	})

	t.Run("fallback answer stays in the tenant", func(t *testing.T) {
		rt := &llm.Runtime{Defaults: llm.Models{Fallback: []string{"claude-sonnet-5-5"}}}
		r, m, _ := newModelRouter(t, rt, nil)
		m.down["claude-opus-5-5"] = saturated{}
		if _, err := r.RouteAndExecute(ctx, "TENANT_A", generalQuestion, true); err != nil {
			t.Fatal(err)
		}
		delete(m.down, "claude-opus-5-5")
		other, _ := r.RouteAndExecute(ctx, "TENANT_B", generalQuestion, true)
		if other.Cached {
			t.Fatal("an answer drafted by a fallback model leaked into GLOBAL")
		}
	})
}
