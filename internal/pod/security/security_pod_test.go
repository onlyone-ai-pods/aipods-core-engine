package securitypod

import (
	"context"
	"strings"
	"testing"
)

func TestCoreSecurityAuditPod(t *testing.T) {
	pod := NewCoreSecurityAuditPod()

	if pod.ID() != "POD_CORE_SECURITY_AUDIT" {
		t.Fatalf("Expected ID POD_CORE_SECURITY_AUDIT, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: Security & SOC2 Audit Log Query
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero consultar el reporte de auditoría de seguridad SOC2", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "SOC 2 Type II") {
		t.Errorf("Expected SOC 2 Type II in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "generar_reporte_auditoria_seguridad" {
		t.Errorf("Expected ActionName generar_reporte_auditoria_seguridad, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: Token Quota Control Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Verificar cuotas de consumo de tokens", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "monitorear_cuota_tokens" {
		t.Errorf("Expected ActionName monitorear_cuota_tokens, got %s", res2.DryRunResult.ActionName)
	}
}
