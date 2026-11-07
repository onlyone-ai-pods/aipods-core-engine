package tenant

import (
	"context"
	"errors"
)

type contextKey string

const TenantIDKey contextKey = "tenant_id"

var ErrMissingTenantID = errors.New("missing mandatory tenant_id in request context")

// WithTenantID injects tenant_id into context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// FromContext extracts tenant_id from context
func FromContext(ctx context.Context) (string, error) {
	tenantID, ok := ctx.Value(TenantIDKey).(string)
	if !ok || tenantID == "" {
		return "", ErrMissingTenantID
	}
	return tenantID, nil
}

// RAGStrategy is how the router finds a tenant's documents (SPEC-CORE-70.R3).
type RAGStrategy string

const (
	// RAGQueryExpansion (default): Claude rewrites the query into formal ARCA
	// terms and the chunks are found with Spanish full-text search.
	RAGQueryExpansion RAGStrategy = "query_expansion"
	// RAGExternalEmbeddings: vector search with a SaaS embedding API, falling
	// back to RAGQueryExpansion when that API fails (R4).
	RAGExternalEmbeddings RAGStrategy = "external_embeddings"
)

// DefaultRAGStrategy applies to tenants without an explicit choice.
const DefaultRAGStrategy = RAGQueryExpansion

// ValidRAGStrategy reports whether s is a known strategy.
func ValidRAGStrategy(s string) bool {
	return s == string(RAGQueryExpansion) || s == string(RAGExternalEmbeddings)
}
