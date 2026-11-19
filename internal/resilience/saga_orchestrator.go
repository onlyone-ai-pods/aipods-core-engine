package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

type SagaStatus string

const (
	SagaStatusInit        SagaStatus = "SAGA_INIT"
	SagaStatusAwaitingOTP SagaStatus = store.SagaAwaitingOTP
	SagaStatusCompleted   SagaStatus = "COMPLETED"
	SagaStatusCompensated SagaStatus = "COMPENSATED"
	SagaStatusFailed      SagaStatus = "FAILED"
)

// ErrSagaNotFound is returned for an unknown saga ID.
var ErrSagaNotFound = errors.New("saga transaction not found")

type SagaTransaction struct {
	SagaID        string     `json:"saga_id"`
	TenantID      string     `json:"tenant_id"`
	ActionName    string     `json:"action_name"`
	IsCritical    bool       `json:"is_critical"`
	Status        SagaStatus `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	ExecutedAt    *time.Time `json:"executed_at,omitempty"`
	CompensatedAt *time.Time `json:"compensated_at,omitempty"`
}

// SagaStore is the part of store.Store the orchestrator needs.
type SagaStore interface {
	CreateSaga(ctx context.Context, s store.Saga) error
	GetSaga(ctx context.Context, sagaID string) (store.Saga, error)
	FinishSaga(ctx context.Context, sagaID, status string) (store.Saga, error)
}

// SagaOrchestrator keeps sagas in the store, so a saga started on one color
// can be verified on the other (SPEC-CORE-73.R9).
type SagaOrchestrator struct {
	st SagaStore
}

// NewSagaOrchestrator keeps sagas in process memory (tests and development).
func NewSagaOrchestrator() *SagaOrchestrator {
	return NewSagaOrchestratorWithStore(store.NewMemory())
}

func NewSagaOrchestratorWithStore(st SagaStore) *SagaOrchestrator {
	return &SagaOrchestrator{st: st}
}

func fromStore(s store.Saga) *SagaTransaction {
	return &SagaTransaction{
		SagaID: s.SagaID, TenantID: s.TenantID, ActionName: s.ActionName, IsCritical: s.IsCritical,
		Status: SagaStatus(s.Status), CreatedAt: s.CreatedAt, ExecutedAt: s.ExecutedAt, CompensatedAt: s.CompensatedAt,
	}
}

// StartSaga inicia la transacción transaccional y aplica interrupción 2FA si la acción es crítica (SPEC-CORE-35)
func (s *SagaOrchestrator) StartSaga(ctx context.Context, tenantID, actionName string, isCritical bool) (*SagaTransaction, error) {
	now := time.Now()
	saga := store.Saga{
		SagaID:     uuid.NewString(),
		TenantID:   tenantID,
		ActionName: actionName,
		IsCritical: isCritical,
		Status:     string(SagaStatusAwaitingOTP),
		CreatedAt:  now,
	}
	if !isCritical {
		saga.Status = string(SagaStatusCompleted)
		saga.ExecutedAt = &now
	}
	if err := s.st.CreateSaga(ctx, saga); err != nil {
		return nil, fmt.Errorf("saga: %w", err)
	}
	return fromStore(saga), nil
}

// VerifyOTPAndExecute procesa el token 2FA de 6 dígitos para continuar o
// compensar la transacción Saga. La transición es atómica en el store: de dos
// verificaciones concurrentes (aun desde colores distintos) solo una gana.
func (s *SagaOrchestrator) VerifyOTPAndExecute(ctx context.Context, sagaID, otpCode string) (*SagaTransaction, error) {
	// Validación simulada de OTP 2FA (Acepta cualquier código de 6 dígitos)
	valid := len(otpCode) == 6 && otpCode != "000000"
	target := SagaStatusCompensated
	if valid {
		target = SagaStatusCompleted
	}

	saga, err := s.st.FinishSaga(ctx, sagaID, string(target))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, ErrSagaNotFound
	case errors.Is(err, store.ErrConflict):
		return nil, fmt.Errorf("invalid saga state for 2FA verification: %s", saga.Status)
	case err != nil:
		return nil, fmt.Errorf("saga: %w", err)
	}
	tx := fromStore(saga)
	if !valid {
		// Ejecución de acción compensatoria (Rollback)
		return tx, errors.New("invalid 2FA OTP code. Saga transaction compensated and rolled back")
	}
	return tx, nil
}

// GetSaga retrieves the current state of a Saga transaction
func (s *SagaOrchestrator) GetSaga(ctx context.Context, sagaID string) (*SagaTransaction, error) {
	saga, err := s.st.GetSaga(ctx, sagaID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrSagaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("saga: %w", err)
	}
	return fromStore(saga), nil
}
