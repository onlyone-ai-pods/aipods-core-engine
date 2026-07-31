package resilience

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type SagaStatus string

const (
	SagaStatusInit          SagaStatus = "SAGA_INIT"
	SagaStatusAwaitingOTP   SagaStatus = "AWAITING_2FA_OTP"
	SagaStatusCompleted     SagaStatus = "COMPLETED"
	SagaStatusCompensated   SagaStatus = "COMPENSATED"
	SagaStatusFailed        SagaStatus = "FAILED"
)

type SagaTransaction struct {
	SagaID         string     `json:"saga_id"`
	TenantID       string     `json:"tenant_id"`
	ActionName     string     `json:"action_name"`
	IsCritical     bool       `json:"is_critical"`
	Status         SagaStatus `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	CompensatedAt  *time.Time `json:"compensated_at,omitempty"`
}

type SagaOrchestrator struct {
	mu           sync.RWMutex
	transactions map[string]*SagaTransaction
}

func NewSagaOrchestrator() *SagaOrchestrator {
	return &SagaOrchestrator{
		transactions: make(map[string]*SagaTransaction),
	}
}

// StartSaga inicia la transacción transaccional y aplica interrupción 2FA si la acción es crítica (SPEC-CORE-35)
func (s *SagaOrchestrator) StartSaga(tenantID, actionName string, isCritical bool) (*SagaTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sagaID := fmt.Sprintf("saga_%d", time.Now().UnixNano()%1000000)
	tx := &SagaTransaction{
		SagaID:     sagaID,
		TenantID:   tenantID,
		ActionName: actionName,
		IsCritical: isCritical,
		Status:     SagaStatusInit,
		CreatedAt:  time.Now(),
	}

	if isCritical {
		tx.Status = SagaStatusAwaitingOTP
	} else {
		now := time.Now()
		tx.Status = SagaStatusCompleted
		tx.ExecutedAt = &now
	}

	s.transactions[sagaID] = tx
	return tx, nil
}

// VerifyOTPAndExecute procesa el token 2FA de 6 dígitos para continuar o compensar la transacción Saga
func (s *SagaOrchestrator) VerifyOTPAndExecute(sagaID, otpCode string) (*SagaTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, exists := s.transactions[sagaID]
	if !exists {
		return nil, errors.New("saga transaction not found")
	}

	if tx.Status != SagaStatusAwaitingOTP {
		return nil, fmt.Errorf("invalid saga state for 2FA verification: %s", tx.Status)
	}

	now := time.Now()
	// Validación simulada de OTP 2FA (Acepta cualquier código de 6 dígitos)
	if len(otpCode) == 6 && otpCode != "000000" {
		tx.Status = SagaStatusCompleted
		tx.ExecutedAt = &now
		return tx, nil
	}

	// Ejecución de acción compensatoria (Rollback)
	tx.Status = SagaStatusCompensated
	tx.CompensatedAt = &now
	return tx, errors.New("invalid 2FA OTP code. Saga transaction compensated and rolled back")
}

// GetSaga retrieves the current state of a Saga transaction
func (s *SagaOrchestrator) GetSaga(sagaID string) (*SagaTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, exists := s.transactions[sagaID]
	if !exists {
		return nil, errors.New("saga transaction not found")
	}
	return tx, nil
}
