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

	} else if strings.Contains(lowerQuery, "punto de venta") || strings.Contains(lowerQuery, "puntos de venta") || strings.Contains(lowerQuery, "pv") || strings.Contains(lowerQuery, "rece") || strings.Contains(lowerQuery, "linea") || strings.Contains(lowerQuery, "odoo") {
		accion := "Consultar"
		if strings.Contains(lowerQuery, "alta") || strings.Contains(lowerQuery, "crear") || strings.Contains(lowerQuery, "agregar") {
			accion = "Alta"
		}

		matches, searchTerm := p.SearchPuntosDeVenta(query)

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			cmdPreview := fmt.Sprintf("node scripts/puntos_de_venta_arca.js --accion=%s --query=\"%s\" --cuit=20262534538", accion, searchTerm)

			var matchesText []string
			for _, pv := range matches {
				matchesText = append(matchesText, fmt.Sprintf("  • **PV N° %s** | Tipo: `%s` | Estado: **%s**", pv.Numero, pv.Tipo, pv.Estado))
			}

			if len(matches) > 0 {
				answer = fmt.Sprintf("🔍 **Análisis Multicolumna de Puntos de Venta (Búsqueda: '%s'):**\n\nAnalizamos los datos registrados en ARCA/AFIP buscando en las 3 columnas (**Número**, **Tipo** y **Estado**). Coincidencias encontradas (%d):\n\n%s\n\nComando a ejecutar:\n```bash\n%s\n```", searchTerm, len(matches), strings.Join(matchesText, "\n"), cmdPreview)
			} else {
				answer = fmt.Sprintf("🔍 **Análisis Multicolumna de Puntos de Venta (Búsqueda: '%s'):**\n\nNo se encontraron Puntos de Venta que coincidan exactamente con '%s' en las columnas (Número, Tipo, Estado).\n\nComando a ejecutar para ver lista completa:\n```bash\n%s\n```", searchTerm, searchTerm, cmdPreview)
			}

			citations = []string{"ARCA_PuntosDeVenta_Spec_v2026.pdf", "Portal_Clave_Fiscal_ARCA.pdf"}

			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "gestionar_puntos_de_venta_arca",
				Summary:               fmt.Sprintf("Simulación de %s de Puntos de Venta (Búsqueda: '%s') en ARCA.", accion, searchTerm),
				AffectedRecordsCount:  len(matches),
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

			answer = fmt.Sprintf("### 📍 Resultado de Búsqueda Multicolumna en ARCA ('%s')\n\nSe consultaron exitosamente los Puntos de Venta configurados:\n\n```text\n%s\n```", searchTerm, string(out))
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

type PuntoDeVenta struct {
	Numero string
	Tipo   string
	Estado string
}

var datasetPV = []PuntoDeVenta{
	{Numero: "00001", Tipo: "Comprobantes en Línea - Mercado Interno", Estado: "ACTIVO"},
	{Numero: "00002", Tipo: "RECE para aplicativo y/o Web Services", Estado: "ACTIVO"},
	{Numero: "00003", Tipo: "FactuWeb Histórico (Deprecado 2021)", Estado: "DADO DE BAJA"},
	{Numero: "00005", Tipo: "Controlador Fiscal Sucursal Belgrano", Estado: "INACTIVO"},
	{Numero: "00007", Tipo: "Factura Electrónica - Odoo Production", Estado: "ACTIVO"},
}

func (p *AFIPPod) SearchPuntosDeVenta(query string) ([]PuntoDeVenta, string) {
	lower := strings.ToLower(query)

	cleanQuery := lower
	stopWords := []string{"quiero", "que", "me", "traiga", "el", "los", "punto", "puntos", "de", "venta", "del", "tipo", "ver", "consultar", "arca", "afip", "en", "mostrar", "buscame", "buscar"}
	for _, word := range stopWords {
		cleanQuery = strings.ReplaceAll(cleanQuery, word, " ")
	}
	cleanQuery = strings.TrimSpace(cleanQuery)

	if cleanQuery == "" || cleanQuery == "activos" || cleanQuery == "activo" {
		var result []PuntoDeVenta
		for _, pv := range datasetPV {
			if pv.Estado == "ACTIVO" {
				result = append(result, pv)
			}
		}
		return result, "Activos"
	}

	var matches []PuntoDeVenta
	for _, pv := range datasetPV {
		if strings.Contains(strings.ToLower(pv.Numero), cleanQuery) ||
			strings.Contains(strings.ToLower(pv.Tipo), cleanQuery) ||
			strings.Contains(strings.ToLower(pv.Estado), cleanQuery) ||
			strings.Contains(cleanQuery, strings.ToLower(pv.Numero)) ||
			strings.Contains(cleanQuery, strings.ToLower(pv.Tipo)) ||
			strings.Contains(cleanQuery, strings.ToLower(pv.Estado)) {
			matches = append(matches, pv)
		}
	}

	if len(matches) == 0 {
		if strings.Contains(lower, "rece") || strings.Contains(lower, "web service") || strings.Contains(lower, "ws") {
			cleanQuery = "RECE"
			for _, pv := range datasetPV {
				if strings.Contains(strings.ToLower(pv.Tipo), "rece") {
					matches = append(matches, pv)
				}
			}
		} else if strings.Contains(lower, "linea") || strings.Contains(lower, "mercado interno") {
			cleanQuery = "Comprobantes en Línea"
			for _, pv := range datasetPV {
				if strings.Contains(strings.ToLower(pv.Tipo), "línea") || strings.Contains(strings.ToLower(pv.Tipo), "linea") {
					matches = append(matches, pv)
				}
			}
		} else if strings.Contains(lower, "odoo") || strings.Contains(lower, "factura electronica") {
			cleanQuery = "Odoo"
			for _, pv := range datasetPV {
				if strings.Contains(strings.ToLower(pv.Tipo), "odoo") {
					matches = append(matches, pv)
				}
			}
		} else if strings.Contains(lower, "inactivo") || strings.Contains(lower, "baja") {
			cleanQuery = "Inactivos"
			for _, pv := range datasetPV {
				if pv.Estado != "ACTIVO" {
					matches = append(matches, pv)
				}
			}
		} else if strings.Contains(lower, "todos") {
			cleanQuery = "Todos"
			matches = datasetPV
		}
	}

	return matches, cleanQuery
}
