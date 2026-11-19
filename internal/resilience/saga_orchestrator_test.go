package resilience

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/store"
)

func TestSagaOrchestratorLifecycle(t *testing.T) {
	orchestrator := NewSagaOrchestrator()

	// 1. Iniciar Saga Crítica con Interrupción 2FA
	tx, err := orchestrator.StartSaga(context.Background(), "TENANT_DEMO_001", "revocar_certificados_afip", true)
	if err != nil {
		t.Fatalf("Failed to start saga: %v", err)
	}

	if tx.Status != SagaStatusAwaitingOTP {
		t.Errorf("Expected status AWAITING_2FA_OTP, got %s", tx.Status)
	}

	// 2. Verificar OTP Válido (6 dígitos)
	completedTx, err := orchestrator.VerifyOTPAndExecute(context.Background(), tx.SagaID, "123456")
	if err != nil {
		t.Fatalf("Failed to verify valid OTP: %v", err)
	}

	if completedTx.Status != SagaStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", completedTx.Status)
	}

	// 3. Probar Rollback Compensatorio con OTP Inválido
	failTx, err := orchestrator.StartSaga(context.Background(), "TENANT_DEMO_001", "eliminar_base_odoo", true)
	if err != nil {
		t.Fatalf("Failed to start second saga: %v", err)
	}

	compensatedTx, err := orchestrator.VerifyOTPAndExecute(context.Background(), failTx.SagaID, "000000")
	if err == nil {
		t.Errorf("Expected error for invalid OTP code")
	}

	if compensatedTx.Status != SagaStatusCompensated {
		t.Errorf("Expected status COMPENSATED, got %s", compensatedTx.Status)
	}
}

func TestSPEC_CORE_73_R9_SagaIDIsUUID(t *testing.T) {
	tx, err := NewSagaOrchestrator().StartSaga(context.Background(), "t", "a", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.SagaID) != 36 || strings.Count(tx.SagaID, "-") != 4 {
		t.Fatalf("saga id %q is not a UUID", tx.SagaID)
	}
}

// A saga started by one instance (blue) is verified by another (green), and
// of two concurrent verifications only one wins.
func TestSPEC_CORE_73_R9_SagaIsSharedAndVerifiedOnce(t *testing.T) {
	for name, st := range sagaStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			blue, green := NewSagaOrchestratorWithStore(st), NewSagaOrchestratorWithStore(st)
			tx, err := blue.StartSaga(ctx, "TENANT_DEMO_001", "revocar_certificados_afip", true)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := green.GetSaga(ctx, tx.SagaID); err != nil || got.Status != SagaStatusAwaitingOTP {
				t.Fatalf("green does not see blue's saga: %+v %v", got, err)
			}

			var wins atomic.Int32
			var wg sync.WaitGroup
			for _, o := range []*SagaOrchestrator{blue, green, blue, green} {
				wg.Add(1)
				go func(o *SagaOrchestrator) {
					defer wg.Done()
					if _, err := o.VerifyOTPAndExecute(ctx, tx.SagaID, "123456"); err == nil {
						wins.Add(1)
					}
				}(o)
			}
			wg.Wait()
			if wins.Load() != 1 {
				t.Fatalf("%d verifications won, want exactly 1", wins.Load())
			}
			got, _ := blue.GetSaga(ctx, tx.SagaID)
			if got.Status != SagaStatusCompleted || got.ExecutedAt == nil {
				t.Fatalf("final saga = %+v", got)
			}
			if _, err := green.VerifyOTPAndExecute(ctx, "no-such-saga", "123456"); !errors.Is(err, ErrSagaNotFound) {
				t.Fatalf("unknown saga: %v", err)
			}
		})
	}
}

func sagaStores(t *testing.T) map[string]SagaStore {
	out := map[string]SagaStore{"memory": store.NewMemory()}
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		p, err := store.OpenPostgres(context.Background(), url)
		if err != nil {
			t.Fatalf("postgres: %v", err)
		}
		t.Cleanup(p.Close)
		out["postgres"] = p
	}
	return out
}
