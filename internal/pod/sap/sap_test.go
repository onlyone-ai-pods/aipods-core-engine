package sap

import (
	"context"
	"strings"
	"testing"
)

func TestSAPPod(t *testing.T) {
	pod := NewSAPPod()

	if pod.ID() != "POD_SAP_ENTERPRISE" {
		t.Fatalf("Expected ID POD_SAP_ENTERPRISE, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: SAP S/4HANA OData Query with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero consultar los pedidos de venta en SAP S/4HANA vía OData", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "API_SALES_ORDER_SRV") {
		t.Errorf("Expected OData sales order API in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "consultar_pedidos_sap_odata" {
		t.Errorf("Expected ActionName consultar_pedidos_sap_odata, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: SAP Business One Service Layer Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Consultar articulos en SAP B1", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "consultar_maestro_materiales_b1" {
		t.Errorf("Expected ActionName consultar_maestro_materiales_b1, got %s", res2.DryRunResult.ActionName)
	}
}
