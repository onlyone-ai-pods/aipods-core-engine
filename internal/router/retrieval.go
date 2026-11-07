package router

import (
	"context"
	"log"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// Dual RAG strategy per tenant (SPEC-CORE-70.R3/R4).

// LexicalRetriever finds chunks by words (implemented by rag.RAGIngestionService).
type LexicalRetriever interface {
	SearchLexical(ctx context.Context, terms []string, topK int) ([]rag.SearchResult, error)
}

// StrategyResolver returns a tenant's RAG strategy.
type StrategyResolver func(ctx context.Context, tenantID string) tenant.RAGStrategy

// Labels reported in PodResponse.RAGStrategy.
const (
	ragUsedExpansion = string(tenant.RAGQueryExpansion)
	ragUsedVector    = string(tenant.RAGExternalEmbeddings)
	ragUsedFallback  = "query_expansion_fallback"
)

const (
	// vectorSearchTimeout bounds the wait for the embeddings API, so a slow
	// provider degrades to lexical search instead of stalling the chat.
	vectorSearchTimeout = 8 * time.Second
	breakerFailures     = 3
	breakerReset        = 60 * time.Second
)

// retrieval holds what retrieve needs, read under the router lock.
type retrieval struct {
	vector   Retriever
	lexical  LexicalRetriever
	strategy StrategyResolver
}

// SetLexicalRetriever enables query expansion + lexical search.
func (r *DynamicSmartRouter) SetLexicalRetriever(l LexicalRetriever) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lexical = l
}

// SetStrategyResolver sets how the router learns each tenant's RAG strategy.
// Without one every tenant uses tenant.DefaultRAGStrategy.
func (r *DynamicSmartRouter) SetStrategyResolver(s StrategyResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = s
}

// SetFallbackHook is called each time external embeddings degrade to lexical
// search (used for the aipods_rag_fallback_total metric).
func (r *DynamicSmartRouter) SetFallbackHook(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onFallback = f
}

// retrieve finds the tenant's documents with the tenant's strategy and
// reports which one was used. It never fails: on errors it logs and returns
// fewer (or no) documents, so the answer is still drafted.
func (r *DynamicSmartRouter) retrieve(ctx context.Context, client llm.Client, deps retrieval, tenantID, query string) ([]rag.SearchResult, string) {
	tctx := tenant.WithTenantID(ctx, tenantID)
	strategy := tenant.DefaultRAGStrategy
	if deps.strategy != nil {
		strategy = deps.strategy(ctx, tenantID)
	}

	used := ragUsedExpansion
	if strategy == tenant.RAGExternalEmbeddings {
		hits, err := r.vectorSearch(tctx, deps.vector, query)
		if err == nil {
			return hits, ragUsedVector
		}
		log.Printf("[router] external embeddings failed for tenant %s, falling back to query expansion: %v", tenantID, err)
		r.mu.RLock()
		hook := r.onFallback
		r.mu.RUnlock()
		if hook != nil {
			hook()
		}
		used = ragUsedFallback
	}

	if deps.lexical == nil {
		// No lexical index (prototype setups): plain vector search, as before SPEC-CORE-70.
		if deps.vector == nil || used == ragUsedFallback {
			return nil, used
		}
		hits, err := deps.vector.Search(tctx, query, ragTopK)
		if err != nil {
			log.Printf("[router] RAG search failed for tenant %s: %v", tenantID, err)
		}
		return hits, ragUsedVector
	}

	terms := []string{query}
	if client != nil {
		expanded, err := llm.ExpandQuery(ctx, client, query)
		if err != nil {
			log.Printf("[router] query expansion failed for tenant %s, searching the raw query: %v", tenantID, err)
		}
		terms = append(terms, expanded...)
	}
	hits, err := deps.lexical.SearchLexical(tctx, terms, ragTopK)
	if err != nil {
		log.Printf("[router] lexical search failed for tenant %s: %v", tenantID, err)
	}
	return hits, used
}

// vectorSearch calls the embeddings-backed store behind a circuit breaker and
// a timeout.
func (r *DynamicSmartRouter) vectorSearch(ctx context.Context, vector Retriever, query string) ([]rag.SearchResult, error) {
	if vector == nil {
		return nil, rag.ErrNoVectorStore
	}
	if !r.vectorBreaker.AllowRequest() {
		return nil, ErrCircuitBreakerOpen
	}
	vctx, cancel := context.WithTimeout(ctx, vectorSearchTimeout)
	defer cancel()
	hits, err := vector.Search(vctx, query, ragTopK)
	if err != nil {
		r.vectorBreaker.RecordFailure()
		return nil, err
	}
	r.vectorBreaker.RecordSuccess()
	return hits, nil
}
