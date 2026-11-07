package llm

import "context"

// Usage is the token count of one model call, attributed to a tenant
// (SPEC-CORE-69). Model is the model that served the call, which can differ
// from the requested one when a server-side fallback answered.
type Usage struct {
	TenantID         string
	Model            string
	Purpose          string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

type tenantKey struct{}

// WithTenant marks ctx so model calls made with it are billed to tenantID.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// TenantFrom returns the tenant set by WithTenant, or "" if none.
func TenantFrom(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey{}).(string)
	return id
}
