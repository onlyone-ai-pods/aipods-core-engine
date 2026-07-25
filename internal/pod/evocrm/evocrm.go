package evocrm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type EvoCRMPod struct{}

func NewEvoCRMPod() *EvoCRMPod {
	return &EvoCRMPod{}
}

func (p *EvoCRMPod) ID() string {
	return "POD_EVOCRM_HELPDESK"
}

func (p *EvoCRMPod) Name() string {
	return "AI Pod EvoCRM & Helpdesk Omnicanal"
}

func (p *EvoCRMPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "webhook") || strings.Contains(lowerQuery, "payload") || strings.Contains(lowerQuery, "integracion") {
		sampleWebhookJSON := `{\n  "event": "messages.upsert",\n  "instance": "instance_prod_01",\n  "data": {\n    "key": { "remoteJid": "5491155550000@s.whatsapp.net", "fromMe": false, "id": "MSG_12345" },\n    "message": { "conversation": "Consulta de soporte técnico sobre factura" }\n  }\n}`
		answer = fmt.Sprintf("AI Pod EvoCRM & Helpdesk: Para integrar EvoCRM con Odoo Helpdesk para la organización (%s), configure la URL base `https://app.aipods.io/evocrm/webhook`.\n\nPayload JSON estándar de validación:\n\n```json\n%s\n```", tenantID, sampleWebhookJSON)
		citations = []string{
			"EvoCRM_WhatsApp_API_Spec.pdf (Pagina 12)",
			"Odoo_Helpdesk_Integration_Guide.pdf (Pagina 45)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "validar_webhook_evocrm",
				Summary:               fmt.Sprintf("Simulación de prueba de Webhook EvoCRM WhatsApp para %s.", tenantID),
				AffectedRecordsCount:  1,
				GeneratedCommand:      "POST https://app.aipods.io/evocrm/webhook (200 OK)",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "masivo") || strings.Contains(lowerQuery, "spam") || strings.Contains(lowerQuery, "grupo") {
		answer = "AI Pod EvoCRM: Para envíos masivos por WhatsApp:\n1. Acceda al módulo EvoCRM > Campañas > Nuevo Envío Masivo.\n2. Seleccione la lista de contactos segmentada.\n\n⚠️ ADVERTENCIA ANTI-SPAM: Respete los límites de velocidad de Meta WhatsApp Cloud API (máximo 80 msgs/minuto por línea) para evitar bloqueos de número."
		citations = []string{
			"Meta_WhatsApp_Business_Policy.pdf (Pagina 8)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "crear_campana_masiva_evocrm",
				Summary:               "Simulación de validación anti-spam para campaña masiva WhatsApp.",
				AffectedRecordsCount:  50,
				GeneratedCommand:      "POST /api/v1/evocrm/broadcasts/validate (RateLimit: 80/min)",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod EvoCRM & Helpdesk: Asistencia omnicanal activa para la empresa (%s).", tenantID)
		citations = []string{"Manual_EvoCRM_Helpdesk.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
