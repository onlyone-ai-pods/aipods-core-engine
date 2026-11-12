package router

import (
	"context"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/rag"
)

// spyCache counts what reaches the semantic cache.
type spyCache struct {
	ResponseCache
	stores int
	global []bool
}

func (s *spyCache) StoreResponse(ctx context.Context, tenantID, query string, resp *pod.PodResponse, global bool) {
	s.stores++
	s.global = append(s.global, global)
	s.ResponseCache.StoreResponse(ctx, tenantID, query, resp, global)
}

func withSpy(t *testing.T, r *DynamicSmartRouter) *spyCache {
	t.Helper()
	spy := &spyCache{ResponseCache: r.cache}
	r.SetCache(spy)
	return spy
}

const informationalQuestion = "¿Cuáles son las categorías del Monotributo vigentes?"

// SPEC-CORE-72.R2: an informational AFIP query with GLOBAL sources is stored
// in the GLOBAL level, and another tenant gets it from the cache.
func TestSPEC_CORE_72_R2_InformationalQueryIsSharedThroughGlobalCache(t *testing.T) {
	ctx := context.Background()
	r, c := newCachedRouter(t, rag.GlobalTenant)
	spy := withSpy(t, r)

	first, err := r.RouteAndExecute(ctx, "tenant_a", informationalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.PodID != "POD_AFIP_FISCAL" || first.DryRunResult != nil {
		t.Fatalf("informational query must reach AFIP without action: pod %s, dryRun %+v", first.PodID, first.DryRunResult)
	}
	if spy.stores != 1 || !spy.global[0] {
		t.Fatalf("expected one GLOBAL store, got stores=%d global=%v", spy.stores, spy.global)
	}
	before := c.calls
	res, err := r.RouteAndExecute(ctx, "tenant_b", informationalQuestion, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cached || res.CacheScope != rag.CacheScopeGlobal || c.calls != before {
		t.Errorf("tenant_b should reuse the GLOBAL answer: cached=%t scope=%s llmCalls=%d", res.Cached, res.CacheScope, c.calls-before)
	}
}

// SPEC-CORE-72.R3: an operational AFIP query keeps the dry-run with approval
// and never reaches the cache.
func TestSPEC_CORE_72_R3_OperationalQueryIsNotCached(t *testing.T) {
	ctx := context.Background()
	r, c := newCachedRouter(t, rag.GlobalTenant)
	spy := withSpy(t, r)
	const q = "Consultá mi estado de cuenta del monotributo"

	for i := 0; i < 2; i++ {
		before := c.calls
		res, err := r.RouteAndExecute(ctx, "tenant_a", q, true)
		if err != nil {
			t.Fatal(err)
		}
		dr := res.DryRunResult
		if dr == nil || !dr.RequiresHumanApproval || dr.ApprovalToken == "" {
			t.Fatalf("call %d: operational query must keep the approval dry-run, got %+v", i, dr)
		}
		if res.Cached || c.calls == before {
			t.Errorf("call %d: operational answer must not come from the cache", i)
		}
	}
	if spy.stores != 0 {
		t.Errorf("operational answers must not be stored, got %d stores", spy.stores)
	}
}

// SPEC-CORE-72.R4: the router reports the pod's intent to the metrics hook.
func TestSPEC_CORE_72_R4_RouterReportsAFIPIntent(t *testing.T) {
	r := NewDynamicSmartRouter()
	got := map[string]int{}
	r.SetIntentHook(func(podID, intent string) { got[podID+":"+intent]++ })
	for _, q := range []string{informationalQuestion, "Descargá mis comprobantes recibidos de octubre"} {
		if _, err := r.RouteAndExecute(context.Background(), "tenant_a", q, true); err != nil {
			t.Fatal(err)
		}
	}
	if got["POD_AFIP_FISCAL:informational"] != 1 || got["POD_AFIP_FISCAL:operational"] != 1 {
		t.Errorf("intents = %v", got)
	}
}

type topKLexical struct{ topK int }

func (f *topKLexical) SearchLexical(_ context.Context, _ []string, topK int) ([]rag.SearchResult, error) {
	f.topK = topK
	return nil, nil
}

type topKVector struct{ topK int }

func (f *topKVector) Search(_ context.Context, _ string, topK int) ([]rag.SearchResult, error) {
	f.topK = topK
	return nil, nil
}

// SPEC-CORE-72.R6: lexical and vector searches receive the configured topK
// (default 10).
func TestSPEC_CORE_72_R6_SearchUsesConfiguredTopK(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  int // 0 = not set
		want int
	}{
		{"default", 0, DefaultRAGTopK},
		{"configured", 8, 8},
		{"out of range", 50, DefaultRAGTopK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &expanderLLM{expanded: "Monotributo", expandOK: true}
			vec, lex, fallbacks := &topKVector{}, &topKLexical{}, 0
			r := newDualRouter(c, vec, lex, &fallbacks)
			if tc.set != 0 {
				r.SetTopK(tc.set)
			}
			ctx := context.Background()
			if _, err := r.RouteAndExecute(ctx, "tenant_default", "categorías monotributo", true); err != nil {
				t.Fatal(err)
			}
			if _, err := r.RouteAndExecute(ctx, "tenant_vector", "categorías monotributo", true); err != nil {
				t.Fatal(err)
			}
			if lex.topK != tc.want || vec.topK != tc.want {
				t.Errorf("lexical topK %d, vector topK %d, want %d", lex.topK, vec.topK, tc.want)
			}
		})
	}
}

func TestSPEC_CORE_72_R6_ParseRAGTopK(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 10, true}, {"8", 8, true}, {" 20 ", 20, true}, {"1", 1, true},
		{"50", 10, false}, {"0", 10, false}, {"-3", 10, false}, {"diez", 10, false},
	} {
		if got, ok := ParseRAGTopK(tc.in); got != tc.want || ok != tc.wantOK {
			t.Errorf("ParseRAGTopK(%q) = %d, %t; want %d, %t", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
