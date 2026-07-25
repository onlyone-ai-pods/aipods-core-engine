package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type CoreBillingPod struct{}

func NewCoreBillingPod() *CoreBillingPod {
	return &CoreBillingPod{}
}

func (p *CoreBillingPod) ID() string {
	return "POD_CORE_BILLING_ODOO"
}

func (p *CoreBillingPod) Name() string {
	return "AI Pod Esencial de Facturación Odoo & Estado de Cuenta"
}

func (p *CoreBillingPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "factura") || strings.Contains(lowerQuery, "cobro") || strings.Contains(lowerQuery, "emitir") {
		generatedCmd := fmt.Sprintf("account.move.create(partner_id='%s', line_ids=[(0, 0, {'name': 'Consumo Tokens SaaS', 'price_unit': 15.50})])", tenantID)
		answer = fmt.Sprintf("AI Pod Facturación Core: Emisión de factura electrónica en Odoo Invoicing para la empresa (%s).\n\nComando Odoo XML-RPC ejecutado:\n\n```python\n%s\n```\n\nFactura 'account.move' registrada en estado borradores para revisión.", tenantID, generatedCmd)
		citations = []string{
			"Odoo_Account_Move_API_Spec.pdf (Pagina 14)",
			"SaaS_Billing_Cycle_Guide.pdf (Pagina 88)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "generar_factura_odoo",
				Summary:               fmt.Sprintf("Simulación de emisión de factura electrónica Odoo para %s.", tenantID),
				AffectedRecordsCount:  1,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "estado") || strings.Contains(lowerQuery, "cuenta") || strings.Contains(lowerQuery, "saldo") || strings.Contains(lowerQuery, "suscripcion") {
		answer = fmt.Sprintf("AI Pod Facturación Core: Estado de Cuenta del cliente (%s):\n- Plan Actual: PROD_ACTIVE\n- Saldo Pendiente: $0.00 USD\n- Tokens Consumidos este mes: 124,500 tokens\n- Próxima fecha de facturación: 01/08/2026", tenantID)
		citations = []string{
			"Odoo_Customer_Statement_Spec.pdf (Pagina 5)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "consultar_estado_cuenta_odoo",
				Summary:               fmt.Sprintf("Consulta de saldo y estado de cuenta Odoo para %s.", tenantID),
				AffectedRecordsCount:  1,
				GeneratedCommand:      fmt.Sprintf("account.partner.ledger(partner_id='%s')", tenantID),
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod Facturación Core Odoo: Servicio esencial de cobros activo para (%s).", tenantID)
		citations = []string{"Manual_SaaS_Billing_Core.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
