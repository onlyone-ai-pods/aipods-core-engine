package router

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// expanderLLM answers the classifier, the query expander and the drafter.
type expanderLLM struct {
	mu       sync.Mutex
	expanded string
	expandOK bool
	prompts  []string
}

func (e *expanderLLM) Model() string { return "fake-model" }

func (e *expanderLLM) Complete(_ context.Context, req llm.Request) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prompts = append(e.prompts, req.User)
	switch {
	case strings.Contains(req.System, "clasificador"):
		return "POD_AFIP_FISCAL", nil
	case strings.Contains(req.System, "términos técnicos"):
		if !e.expandOK {
			return "", errors.New("expander down")
		}
		return e.expanded, nil
	default:
		return "respuesta redactada", nil
	}
}

func (e *expanderLLM) lastPrompt() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.prompts[len(e.prompts)-1]
}

type fakeVector struct {
	calls int
	err   error
}

func (f *fakeVector) Search(ctx context.Context, _ string, _ int) ([]rag.SearchResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return []rag.SearchResult{{Chunk: rag.DocumentChunk{Content: "vector hit"}, CitationID: "Vector.pdf (Pagina 1)"}}, nil
}

type fakeLexical struct {
	calls    int
	gotTerms []string
	tenant   string
}

func (f *fakeLexical) SearchLexical(ctx context.Context, terms []string, _ int) ([]rag.SearchResult, error) {
	f.calls++
	f.gotTerms = terms
	f.tenant, _ = tenant.FromContext(ctx)
	return []rag.SearchResult{{Chunk: rag.DocumentChunk{Content: "Comprobantes clase E"}, CitationID: "ARCA_Factura_E.pdf (Pagina 3)"}}, nil
}

func strategies(m map[string]tenant.RAGStrategy) StrategyResolver {
	return func(_ context.Context, tenantID string) tenant.RAGStrategy {
		if s, ok := m[tenantID]; ok {
			return s
		}
		return tenant.DefaultRAGStrategy
	}
}

func newDualRouter(c llm.Client, vec Retriever, lex LexicalRetriever, fallbacks *int) *DynamicSmartRouter {
	r := NewDynamicSmartRouter()
	r.SetLLM(c)
	r.SetRetriever(vec)
	r.SetLexicalRetriever(lex)
	r.SetStrategyResolver(strategies(map[string]tenant.RAGStrategy{"tenant_vector": tenant.RAGExternalEmbeddings}))
	r.SetFallbackHook(func() { *fallbacks++ })
	return r
}

// SPEC-CORE-70.R3: the router dispatches by tenant_id. R1: the lexical search
// receives the original query plus Claude's expansion.
func TestRouterDispatchesRAGStrategyByTenant(t *testing.T) {
	c := &expanderLLM{expanded: "Factura E\nExportación de servicios", expandOK: true}
	vec, lex, fallbacks := &fakeVector{}, &fakeLexical{}, 0
	r := newDualRouter(c, vec, lex, &fallbacks)

	res, err := r.RouteAndExecute(context.Background(), "tenant_default", "cómo cobro laburo afuera", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RAGStrategy != "query_expansion" || vec.calls != 0 || lex.calls != 1 || lex.tenant != "tenant_default" {
		t.Fatalf("default tenant: strategy %q, vector %d, lexical %d (%s)", res.RAGStrategy, vec.calls, lex.calls, lex.tenant)
	}
	want := []string{"cómo cobro laburo afuera", "Factura E", "Exportación de servicios"}
	if strings.Join(lex.gotTerms, "|") != strings.Join(want, "|") {
		t.Errorf("terms = %q", lex.gotTerms)
	}
	if !strings.Contains(c.lastPrompt(), "ARCA_Factura_E.pdf (Pagina 3)") {
		t.Error("lexical hits must reach the drafting prompt")
	}

	res, err = r.RouteAndExecute(context.Background(), "tenant_vector", "cómo cobro laburo afuera", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RAGStrategy != "external_embeddings" || vec.calls != 1 || lex.calls != 1 {
		t.Fatalf("vector tenant: strategy %q, vector %d, lexical %d", res.RAGStrategy, vec.calls, lex.calls)
	}
	if len(res.Citations) != 1 || res.Citations[0] != "Vector.pdf (Pagina 1)" || fallbacks != 0 {
		t.Errorf("citations %v, fallbacks %d", res.Citations, fallbacks)
	}
}

// SPEC-CORE-70.R4: a network failure of the embeddings API degrades to query
// expansion + lexical search, and the user still gets a cited answer.
func TestRouterFallsBackWhenEmbeddingsAPIFails(t *testing.T) {
	c := &expanderLLM{expanded: "Factura E", expandOK: true}
	vec := &fakeVector{err: errors.New("voyage embed: dial tcp: connection refused")}
	lex, fallbacks := &fakeLexical{}, 0
	r := newDualRouter(c, vec, lex, &fallbacks)

	for i := 0; i < breakerFailures+2; i++ {
		res, err := r.RouteAndExecute(context.Background(), "tenant_vector", "cómo cobro laburo afuera", true)
		if err != nil {
			t.Fatalf("the chat must not fail: %v", err)
		}
		if res.RAGStrategy != "query_expansion_fallback" || res.Answer != "respuesta redactada" {
			t.Fatalf("call %d: strategy %q answer %q", i, res.RAGStrategy, res.Answer)
		}
		if len(res.Citations) != 1 || res.Citations[0] != "ARCA_Factura_E.pdf (Pagina 3)" {
			t.Fatalf("call %d: citations %v", i, res.Citations)
		}
	}
	if fallbacks != breakerFailures+2 || lex.calls != breakerFailures+2 {
		t.Errorf("fallbacks %d, lexical calls %d", fallbacks, lex.calls)
	}
	// After breakerFailures errors the breaker opens: the API is not called again.
	if vec.calls != breakerFailures {
		t.Errorf("vector calls = %d, want %d (circuit breaker)", vec.calls, breakerFailures)
	}
}

// If the expander also fails, the raw query is still searched.
func TestRouterSearchesRawQueryWhenExpansionFails(t *testing.T) {
	c := &expanderLLM{expandOK: false}
	lex, fallbacks := &fakeLexical{}, 0
	r := newDualRouter(c, nil, lex, &fallbacks)

	res, err := r.RouteAndExecute(context.Background(), "tenant_vector", "monotributo", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RAGStrategy != "query_expansion_fallback" || fallbacks != 1 {
		t.Errorf("no vector store configured must fall back: %q, %d", res.RAGStrategy, fallbacks)
	}
	if len(lex.gotTerms) != 1 || lex.gotTerms[0] != "monotributo" {
		t.Errorf("terms = %q", lex.gotTerms)
	}
}
