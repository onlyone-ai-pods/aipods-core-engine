package billing

import (
	"context"
	"strings"
	"testing"
)

func TestCoreBillingPod(t *testing.T) {
	pod := NewCoreBillingPod()

	if pod.ID() != "POD_CORE_BILLING_ODOO" {
		t.Fatalf("Expected ID POD_CORE_BILLING_ODOO, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: Odoo Invoice Generation Query with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero emitir la factura del consumo mensual en Odoo", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "account.move") {
		t.Errorf("Expected account.move in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "generar_factura_odoo" {
		t.Errorf("Expected ActionName generar_factura_odoo, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: Customer Account Statement Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Consultar estado de cuenta y saldo pendiente", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "consultar_estado_cuenta_odoo" {
		t.Errorf("Expected ActionName consultar_estado_cuenta_odoo, got %s", res2.DryRunResult.ActionName)
	}
}
