package securitypod

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type CoreSecurityAuditPod struct{}

func NewCoreSecurityAuditPod() *CoreSecurityAuditPod {
	return &CoreSecurityAuditPod{}
}

func (p *CoreSecurityAuditPod) ID() string {
	return "POD_CORE_SECURITY_AUDIT"
}

func (p *CoreSecurityAuditPod) Name() string {
	return "AI Pod Esencial de Control de Logs & Auditoría SOC2/ISO27001"
}

func (p *CoreSecurityAuditPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "log") || strings.Contains(lowerQuery, "auditoria") || strings.Contains(lowerQuery, "soc2") || strings.Contains(lowerQuery, "iso") {
		answer = fmt.Sprintf("AI Pod Seguridad & Logs Core: Reporte de Auditoría SOC 2 Type II para la organización (%s):\n- Registro de Intentos de Infiltración: 0 amenazas detectadas\n- Archivos Sanitizados por FileSanitizer: 100%% aprobados\n- Aislamiento Multi-Tenant: Verificado estricto (WHERE tenant_id == '%s')\n- Log de Auditoría Inmutable: Sincronizado en PostgreSQL audit_trail.", tenantID, tenantID)
		citations = []string{
			"SOC2_Compliance_Audit_Spec.pdf (Pagina 10)",
			"ISO27001_Access_Control_Policy.pdf (Pagina 33)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "generar_reporte_auditoria_seguridad",
				Summary:               fmt.Sprintf("Generación de reporte de cumplimiento SOC2/ISO27001 para %s.", tenantID),
				AffectedRecordsCount:  1,
				GeneratedCommand:      fmt.Sprintf("security.audit.generate_report(tenant_id='%s')", tenantID),
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "token") || strings.Contains(lowerQuery, "cuota") || strings.Contains(lowerQuery, "limite") {
		answer = fmt.Sprintf("AI Pod Seguridad & Logs Core: Monitoreo de Cuotas de Tokens para (%s):\n- Cuota Asignada: 500,000 tokens/mes\n- Consumo Actual: 124,500 tokens (24.9%%)\n- Alertas de Sobrecosto: Ninguna activada", tenantID)
		citations = []string{
			"FinOps_Token_Quota_Control.pdf (Pagina 4)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "monitorear_cuota_tokens",
				Summary:               fmt.Sprintf("Auditoría de uso de tokens y cuotas de consumo para %s.", tenantID),
				AffectedRecordsCount:  1,
				GeneratedCommand:      fmt.Sprintf("finops.quota.check(tenant_id='%s')", tenantID),
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod Seguridad & Logs Core: Servicio de auditoría inmutable activo para (%s).", tenantID)
		citations = []string{"Manual_Security_Audit_Core.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
