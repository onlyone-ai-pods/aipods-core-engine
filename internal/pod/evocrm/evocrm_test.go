package evocrm

import (
	"context"
	"strings"
	"testing"
)

func TestEvoCRMPod(t *testing.T) {
	pod := NewEvoCRMPod()

	if pod.ID() != "POD_EVOCRM_HELPDESK" {
		t.Fatalf("Expected ID POD_EVOCRM_HELPDESK, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: Webhook Payload Query with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Cómo configuro la URL de webhook de EvoCRM en Odoo Helpdesk?", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "evocrm/webhook") {
		t.Errorf("Expected webhook URL in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "validar_webhook_evocrm" {
		t.Errorf("Expected ActionName validar_webhook_evocrm, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: Anti-Spam Broadcast Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Cómo creo un envío masivo de WhatsApp a un grupo?", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "crear_campana_masiva_evocrm" {
		t.Errorf("Expected ActionName crear_campana_masiva_evocrm, got %s", res2.DryRunResult.ActionName)
	}
}
