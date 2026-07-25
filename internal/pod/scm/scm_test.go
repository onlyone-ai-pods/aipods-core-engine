package scm

import (
	"context"
	"strings"
	"testing"
)

func TestSCMPod(t *testing.T) {
	pod := NewSCMPod()

	if pod.ID() != "POD_SCM_LOGISTICS" {
		t.Fatalf("Expected ID POD_SCM_LOGISTICS, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: WMS Multi-Step Routes Query
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Cómo configuro una ruta de 3 pasos en WMS de Odoo?", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "Rutas de Varios Pasos") {
		t.Errorf("Expected Rutas de Varios Pasos in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "configurar_rutas_wms" {
		t.Errorf("Expected ActionName configurar_rutas_wms, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: MRP Phantom BoM Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Qué tipo de lista de materiales BoM uso para un kit?", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "recomendar_bom_mrp" {
		t.Errorf("Expected ActionName recomendar_bom_mrp, got %s", res2.DryRunResult.ActionName)
	}

	// Test Case 3: Landed Costs Query
	res3, err := pod.ProcessQuery(ctx, "tenant_acme", "Cómo imputo gastos de flete marítimo con landed costs?", true)
	if err != nil {
		t.Fatalf("Test Case 3 failed: %v", err)
	}
	if res3.DryRunResult.ActionName != "calcular_landed_costs" {
		t.Errorf("Expected ActionName calcular_landed_costs, got %s", res3.DryRunResult.ActionName)
	}
}
