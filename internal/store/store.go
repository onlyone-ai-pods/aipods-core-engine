// Package store persists tenants, users, approvals, vault secrets and the
// audit log (SPEC-CORE-67). Postgres is used when DATABASE_URL is set; the
// in-memory implementation is for development and tests.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrConflict is returned when a unique key already exists.
var ErrConflict = errors.New("store: already exists")

type Approval struct {
	Token           string     `json:"token"`
	PodID           string     `json:"pod_id"`
	ActionName      string     `json:"action_name"`
	Summary         string     `json:"summary"`
	Command         string     `json:"command"`
	TenantID        string     `json:"tenant_id"`
	Status          string     `json:"status"` // PENDING, APPROVED, REJECTED
	ExecutionResult string     `json:"execution_result,omitempty"`
	DecidedBy       string     `json:"decided_by,omitempty"`
	RequestedAt     time.Time  `json:"requested_at"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
}

type Secret struct {
	TenantID    string    `json:"tenant_id"`
	KeyName     string    `json:"key_name"`
	CipherText  string    `json:"-"`
	MaskedValue string    `json:"masked_value"`
	Algorithm   string    `json:"algorithm"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type User struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Roles        []string  `json:"roles"`
	Active       bool      `json:"active"`
	TokenVersion int       `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuditEvent struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	Outcome   string    `json:"outcome"`
	ClientIP  string    `json:"client_ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// LLMUsage is one model call's tokens (SPEC-CORE-69).
type LLMUsage struct {
	TenantID         string    `json:"tenant_id"`
	Model            string    `json:"model"`
	Purpose          string    `json:"purpose"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

// LLMUsageTotal sums the calls of one tenant and model.
type LLMUsageTotal struct {
	TenantID         string `json:"tenant_id"`
	Model            string `json:"model"`
	Requests         int64  `json:"requests"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
}

// Store is everything the core persists. tenantID "" in a List call means
// all tenants; callers must only pass "" for cross-tenant roles.
type Store interface {
	EnsureTenant(ctx context.Context, id, name string) error

	CreateUser(ctx context.Context, u User) error
	UserByEmail(ctx context.Context, email string) (User, error)
	CountUsers(ctx context.Context) (int, error)
	GetUser(ctx context.Context, id string) (User, error)
	// ListUsers returns users oldest first; tenantID "" means all tenants.
	ListUsers(ctx context.Context, tenantID string) ([]User, error)
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	SetUserActive(ctx context.Context, id string, active bool) error
	// SetUserRoles replaces the roles and bumps token_version, so tokens
	// issued with the old roles stop working.
	SetUserRoles(ctx context.Context, id string, roles []string) error

	AddApproval(ctx context.Context, a Approval) error
	ListApprovals(ctx context.Context, tenantID string) ([]Approval, error)
	GetApproval(ctx context.Context, token string) (Approval, error)
	// DecideApproval moves a PENDING approval to APPROVED or REJECTED.
	// It returns ErrConflict if the approval is no longer pending.
	DecideApproval(ctx context.Context, token, status, result, decidedBy string) (Approval, error)

	PutSecret(ctx context.Context, s Secret) (Secret, error)
	ListSecrets(ctx context.Context, tenantID string) ([]Secret, error)
	GetSecret(ctx context.Context, tenantID, keyName string) (Secret, error)

	AppendAudit(ctx context.Context, e AuditEvent) error
	ListAudit(ctx context.Context, tenantID string, limit int) ([]AuditEvent, error)

	RecordLLMUsage(ctx context.Context, u LLMUsage) error
	// SumLLMUsage totals calls since the given time by tenant and model,
	// ordered by tenant then model; tenantID "" means all tenants.
	SumLLMUsage(ctx context.Context, tenantID string, since time.Time) ([]LLMUsageTotal, error)

	Ping(ctx context.Context) error
	Name() string
	Close()
}
