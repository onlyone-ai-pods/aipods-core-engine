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

// BuildMultiTenantFilter returns SQL/Qdrant metadata filter invariant
func BuildMultiTenantFilter(tenantID string) string {
	return "WHERE (tenant_id = '" + tenantID + "' OR tenant_id = 'GLOBAL') AND status = 'ACTIVE'"
}
