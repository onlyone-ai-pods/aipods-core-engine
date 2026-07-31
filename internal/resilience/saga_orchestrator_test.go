package resilience

import (
	"testing"
)

func TestSagaOrchestratorLifecycle(t *testing.T) {
	orchestrator := NewSagaOrchestrator()

	// 1. Iniciar Saga Crítica con Interrupción 2FA
	tx, err := orchestrator.StartSaga("TENANT_DEMO_001", "revocar_certificados_afip", true)
	if err != nil {
		t.Fatalf("Failed to start saga: %v", err)
	}

	if tx.Status != SagaStatusAwaitingOTP {
		t.Errorf("Expected status AWAITING_2FA_OTP, got %s", tx.Status)
	}

	// 2. Verificar OTP Válido (6 dígitos)
	completedTx, err := orchestrator.VerifyOTPAndExecute(tx.SagaID, "123456")
	if err != nil {
		t.Fatalf("Failed to verify valid OTP: %v", err)
	}

	if completedTx.Status != SagaStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", completedTx.Status)
	}

	// 3. Probar Rollback Compensatorio con OTP Inválido
	failTx, err := orchestrator.StartSaga("TENANT_DEMO_001", "eliminar_base_odoo", true)
	if err != nil {
		t.Fatalf("Failed to start second saga: %v", err)
	}

	compensatedTx, err := orchestrator.VerifyOTPAndExecute(failTx.SagaID, "000000")
	if err == nil {
		t.Errorf("Expected error for invalid OTP code")
	}

	if compensatedTx.Status != SagaStatusCompensated {
		t.Errorf("Expected status COMPENSATED, got %s", compensatedTx.Status)
	}
}
