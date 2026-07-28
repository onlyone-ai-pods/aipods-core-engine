package afip

import (
	"context"
	"strings"
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

	if res.PodID != "POD_AFIP_FISCAL" {
		t.Errorf("Expected PodID POD_AFIP_FISCAL, got: %s", res.PodID)
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

func TestAFIPPodComprobantesQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Quiero consultar mis comprobantes emitidos en AFIP", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "descargar_comprobantes_arca" {
		t.Errorf("Expected ActionName descargar_comprobantes_arca, got: %s", resDry.DryRunResult.ActionName)
	}

	if !strings.Contains(resDry.Answer, "Dry-Run Simulation") {
		t.Errorf("Expected dry-run answer header")
	}
}

func TestAFIPPodPuntosDeVentaQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Consulta mis puntos de venta en ARCA", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "gestionar_puntos_de_venta_arca" {
		t.Errorf("Expected ActionName gestionar_puntos_de_venta_arca, got: %s", resDry.DryRunResult.ActionName)
	}
}

func TestAFIPPodRetencionesQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Descargá mis retenciones sufridas de IVA y Ganancias en ARCA", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "descargar_retenciones_arca" {
		t.Errorf("Expected ActionName descargar_retenciones_arca, got: %s", resDry.DryRunResult.ActionName)
	}
}
