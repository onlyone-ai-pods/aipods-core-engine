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

// ErrInvalid is returned when a value is not one of the allowed ones.
var ErrInvalid = errors.New("store: invalid value")

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

// Saga is a 2FA saga of SPEC-CORE-35, shared by both colors (SPEC-CORE-73.R9).
type Saga struct {
	SagaID        string
	TenantID      string
	ActionName    string
	IsCritical    bool
	Status        string
	CreatedAt     time.Time
	ExecutedAt    *time.Time
	CompensatedAt *time.Time
}

// SagaAwaitingOTP is the only state FinishSaga moves a saga out of.
const SagaAwaitingOTP = "AWAITING_2FA_OTP"

// DynamicPod is a pod registered at runtime, loaded by every instance
// (SPEC-CORE-73.R10).
type DynamicPod struct {
	PodID       string
	Name        string
	TenantID    string
	EndpointURL string
	Keywords    []string
	Status      string
	UpdatedAt   time.Time
}

// TenantLLMModels is a tenant's own LLM configuration (SPEC-CORE-74.R5). An
// empty task model inherits the platform's. FallbackSet false inherits the
// platform's fallback chain; true with an empty Fallback means no chain.
type TenantLLMModels struct {
	Classify    string
	Expand      string
	Answer      string
	Fallback    []string
	FallbackSet bool
}

// Store is everything the core persists. tenantID "" in a List call means
// all tenants; callers must only pass "" for cross-tenant roles.
type Store interface {
	EnsureTenant(ctx context.Context, id, name string) error
	// GetRAGStrategy returns the tenant's RAG strategy (SPEC-CORE-70.R3), or
	// ErrNotFound when the tenant has no row.
	GetRAGStrategy(ctx context.Context, tenantID string) (string, error)
	// SetRAGStrategy returns ErrInvalid for an unknown strategy and
	// ErrNotFound for an unknown tenant.
	SetRAGStrategy(ctx context.Context, tenantID, strategy string) error
	// GetLLMModels returns the tenant's own LLM models (SPEC-CORE-74.R5), or
	// ErrNotFound when the tenant has no row.
	GetLLMModels(ctx context.Context, tenantID string) (TenantLLMModels, error)
	// SetLLMModels replaces them; ErrNotFound for an unknown tenant. Model
	// names are validated by the caller against the allowed list.
	SetLLMModels(ctx context.Context, tenantID string, m TenantLLMModels) error

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

	CreateSaga(ctx context.Context, s Saga) error
	GetSaga(ctx context.Context, sagaID string) (Saga, error)
	// FinishSaga moves a saga from AWAITING_2FA_OTP to status in one atomic
	// step and stamps executed_at (COMPLETED) or compensated_at (any other).
	// It returns ErrConflict with the current saga if it was not awaiting.
	FinishSaga(ctx context.Context, sagaID, status string) (Saga, error)

	// UpsertDynamicPod creates or replaces a pod by PodID.
	UpsertDynamicPod(ctx context.Context, p DynamicPod) error
	ListDynamicPods(ctx context.Context) ([]DynamicPod, error)

	AppendAudit(ctx context.Context, e AuditEvent) error
	ListAudit(ctx context.Context, tenantID string, limit int) ([]AuditEvent, error)

	Ping(ctx context.Context) error
	Name() string
	Close()
}
