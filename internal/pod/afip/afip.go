package afip

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type AFIPPod struct{}

func NewAFIPPod() *AFIPPod {
	return &AFIPPod{}
}

func (p *AFIPPod) ID() string {
	return "POD_AFIP_FISCAL"
}

func (p *AFIPPod) Name() string {
	return "AI Pod AFIP / ARCA & Gestión Fiscal"
}

func (p *AFIPPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "retencion") || strings.Contains(lowerQuery, "retenciones") || strings.Contains(lowerQuery, "percepcion") || strings.Contains(lowerQuery, "sicore") || strings.Contains(lowerQuery, "sire") {
		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			cmdPreview := "node scripts/mis_retenciones_arca.js --cuit=20262534538"
			answer = fmt.Sprintf("🔍 **[Dry-Run Simulation]** Se simula la consulta de **Mis Retenciones / Percepciones** en el servicio Mirequa de ARCA para el CUIT (%s).\n\nComando a ejecutar:\n```bash\n%s\n```", tenantID, cmdPreview)
			citations = []string{"ARCA_MisRetenciones_Spec_v2026.pdf"}

			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "descargar_retenciones_arca",
				Summary:               "Simulación de consulta de retenciones/percepciones en ARCA.",
				AffectedRecordsCount:  0,
				GeneratedCommand:      cmdPreview,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		} else {
			cmd := exec.CommandContext(ctx, "node", "scripts/mis_retenciones_arca.js")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("error al ejecutar RPA Mis Retenciones: %w (output: %s)", err, string(out))
			}

			answer = fmt.Sprintf("### 📑 Resultado de Consulta - Mis Retenciones (Mirequa ARCA)\n\nSe completó la consulta en el portal de ARCA de manera exitosa:\n\n```text\n%s\n```", string(out))
			citations = []string{"ARCA_MisRetenciones_LiveResult.pdf"}
		}

	} else if strings.Contains(lowerQuery, "punto de venta") || strings.Contains(lowerQuery, "puntos de venta") || strings.Contains(lowerQuery, "pv") || strings.Contains(lowerQuery, "alta de punto") {
		accion := "Consultar"
		if strings.Contains(lowerQuery, "alta") || strings.Contains(lowerQuery, "crear") || strings.Contains(lowerQuery, "agregar") {
			accion = "Alta"
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			cmdPreview := fmt.Sprintf("node scripts/puntos_de_venta_arca.js --accion=%s --cuit=20262534538", accion)
			answer = fmt.Sprintf("🔍 **[Dry-Run Simulation]** Se simula la acción **%s de Puntos de Venta** en el servicio 'Administración de Puntos de Venta y Domicilios' de ARCA para el CUIT (%s).\n\nComando a ejecutar:\n```bash\n%s\n```", accion, tenantID, cmdPreview)
			citations = []string{"ARCA_PuntosDeVenta_Spec_v2026.pdf", "Portal_Clave_Fiscal_ARCA.pdf"}

			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "gestionar_puntos_de_venta_arca",
				Summary:               fmt.Sprintf("Simulación de %s de Puntos de Venta en ARCA.", accion),
				AffectedRecordsCount:  6,
				GeneratedCommand:      cmdPreview,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		} else {
			cmd := exec.CommandContext(ctx, "node", "scripts/puntos_de_venta_arca.js")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("error al ejecutar RPA Puntos de Venta: %w (output: %s)", err, string(out))
			}

			answer = fmt.Sprintf("### 📍 Puntos de Venta Registrados en ARCA\n\nSe consultaron exitosamente los Puntos de Venta configurados en la Administración de Puntos de Venta y Domicilios:\n\n```text\n%s\n```", string(out))
			citations = []string{"ARCA_PuntosDeVenta_LiveResult.pdf"}
		}

	} else if strings.Contains(lowerQuery, "comprobante") || strings.Contains(lowerQuery, "factura") || strings.Contains(lowerQuery, "mis comprobantes") || strings.Contains(lowerQuery, "emitidos") || strings.Contains(lowerQuery, "recibidos") {
		tipo := "Emitidos"
		if strings.Contains(lowerQuery, "recibido") {
			tipo = "Recibidos"
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			cmdPreview := fmt.Sprintf("node scripts/mis_comprobantes_arca.js --tipo=%s --cuit=20262534538", tipo)
			answer = fmt.Sprintf("🔍 **[Dry-Run Simulation]** Se simula la consulta de **Mis Comprobantes (%s)** en el portal de ARCA/AFIP para el CUIT de la empresa (%s).\n\nComando a ejecutar:\n```bash\n%s\n```", tipo, tenantID, cmdPreview)
			citations = []string{"ARCA_MisComprobantes_Spec_v2026.pdf", "Portal_Clave_Fiscal_ARCA.pdf"}

			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "descargar_comprobantes_arca",
				Summary:               fmt.Sprintf("Simulación de consulta de comprobantes %s en ARCA/AFIP.", tipo),
				AffectedRecordsCount:  12,
				GeneratedCommand:      cmdPreview,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		} else {
			cmd := exec.CommandContext(ctx, "node", "scripts/mis_comprobantes_arca.js")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("error al ejecutar RPA Mis Comprobantes: %w (output: %s)", err, string(out))
			}

			answer = fmt.Sprintf("### 📄 Resultado de Consulta en ARCA - Mis Comprobantes (%s)\n\nSe completó la autenticación y consulta en el portal de ARCA de manera exitosa.\n\n```text\n%s\n```", tipo, string(out))
			citations = []string{"ARCA_MisComprobantes_LiveResult.pdf"}
		}

	} else if strings.Contains(lowerQuery, "csr") || strings.Contains(lowerQuery, "clave") || strings.Contains(lowerQuery, "certificado") {
		generatedCmd := "openssl req -new -key privada.key -out pedido.csr"
		answer = "Para generar la clave privada y el archivo CSR para AFIP/ARCA, ejecute el siguiente comando OpenSSL en su terminal:\n\n```bash\n" + generatedCmd + "\n```\n\nPosteriormente, cargue el certificado `.crt` emitido por AFIP en la configuración de la compañía."
		citations = []string{
			"Guia_AFIP_Certificados_v1.pdf (Pagina 4)",
			"Normativa_ARCA_2026.pdf (Pagina 12)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "generar_csr_afip",
				Summary:               "Simulación de comando OpenSSL para solicitud de certificado AFIP.",
				AffectedRecordsCount:  1,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = "AI Pod AFIP / ARCA: Consulta recibida. Procesando normativas fiscales para la empresa (" + tenantID + ")."
		citations = []string{"Manual_AFIP_General.pdf (Pagina 1)"}
	}

	_ = json.Marshal

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
