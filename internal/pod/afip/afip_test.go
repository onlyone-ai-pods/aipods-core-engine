package afip

import (
	"context"
	"testing"
)

func TestAFIPPodCSRQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	res, err := pod.ProcessQuery(ctx, tenantID, "Cómo genero mi archivo CSR para AFIP?", true)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if res.PodID != "POD_AFIP_FINANCE" {
		t.Errorf("Expected PodID POD_AFIP_FINANCE, got: %s", res.PodID)
	}

	if len(res.Citations) == 0 {
		t.Errorf("Expected citations, got 0")
	}

	if res.DryRunResult == nil || !res.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active (is_dry_run = true)")
	}

	if res.DryRunResult.GeneratedCommand != "openssl req -new -key privada.key -out pedido.csr" {
		t.Errorf("Unexpected generated command: %s", res.DryRunResult.GeneratedCommand)
	}
}
