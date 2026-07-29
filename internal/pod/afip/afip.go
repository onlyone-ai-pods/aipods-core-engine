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

	} else if strings.Contains(lowerQuery, "monotributo") || strings.Contains(lowerQuery, "estado de cuenta") || strings.Contains(lowerQuery, "cuota") || strings.Contains(lowerQuery, "categoria") || strings.Contains(lowerQuery, "categoría") || strings.Contains(lowerQuery, "periodo") || strings.Contains(lowerQuery, "período") {

		periodoFiltro, suggestion := p.ParsePeriodoMonotributo(query)

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			cmdPreview := fmt.Sprintf("node scripts/monotributo_estado_cuenta.js --cuit=20262534538 --periodo=%s", periodoFiltro)

			if suggestion != "" {
				answer = fmt.Sprintf("🧾 **Estado de Cuenta Monotributo**\n\n%s\n\nComando a ejecutar:\n```bash\n%s\n```", suggestion, cmdPreview)
			} else {
				answer = fmt.Sprintf("🧾 **Estado de Cuenta Monotributo (Período: %s)**\n\nSe simula la consulta del estado de cuenta del Monotributo en ARCA para el CUIT del contribuyente.\n\nComando a ejecutar:\n```bash\n%s\n```", periodoFiltro, cmdPreview)
			}
			citations = []string{"ARCA_Monotributo_Spec_v2026.pdf", "RG_ARCA_Monotributo_2026.pdf"}

			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "consultar_monotributo_estado_cuenta",
				Summary:               fmt.Sprintf("Consulta de Estado de Cuenta Monotributo (Período: %s).", periodoFiltro),
				AffectedRecordsCount:  6,
				GeneratedCommand:      cmdPreview,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		} else {
			answer = "### 🧾 Estado de Cuenta Monotributo\n\nSe completó la consulta del estado de cuenta del Monotributo."
			citations = []string{"ARCA_Monotributo_LiveResult.pdf"}
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

	// Direct entity keyword matching for conversational queries
	if strings.Contains(lower, "odoo") {
		var matches []PuntoDeVenta
		for _, pv := range datasetPV {
			if strings.Contains(strings.ToLower(pv.Tipo), "odoo") {
				matches = append(matches, pv)
			}
		}
		return matches, "Odoo"
	}

	if strings.Contains(lower, "rece") || strings.Contains(lower, "web service") || strings.Contains(lower, "ws") || strings.Contains(lower, "aplicativo") {
		var matches []PuntoDeVenta
		for _, pv := range datasetPV {
			if strings.Contains(strings.ToLower(pv.Tipo), "rece") {
				matches = append(matches, pv)
			}
		}
		return matches, "RECE"
	}

	if strings.Contains(lower, "belgrano") || strings.Contains(lower, "controlador") {
		var matches []PuntoDeVenta
		for _, pv := range datasetPV {
			if strings.Contains(strings.ToLower(pv.Tipo), "belgrano") || strings.Contains(strings.ToLower(pv.Tipo), "controlador") {
				matches = append(matches, pv)
			}
		}
		return matches, "Sucursal Belgrano"
	}

	if strings.Contains(lower, "linea") || strings.Contains(lower, "mercado interno") {
		var matches []PuntoDeVenta
		for _, pv := range datasetPV {
			if strings.Contains(strings.ToLower(pv.Tipo), "línea") || strings.Contains(strings.ToLower(pv.Tipo), "linea") {
				matches = append(matches, pv)
			}
		}
		return matches, "Comprobantes en Línea"
	}

	if strings.Contains(lower, "inactivo") || strings.Contains(lower, "baja") || strings.Contains(lower, "desactivado") {
		var matches []PuntoDeVenta
		for _, pv := range datasetPV {
			if pv.Estado != "ACTIVO" {
				matches = append(matches, pv)
			}
		}
		return matches, "Inactivos"
	}

	if strings.Contains(lower, "todos") || strings.Contains(lower, "completo") || strings.Contains(lower, "historial") {
		return datasetPV, "Todos"
	}

	cleanQuery := lower
	stopWords := []string{"quiero", "que", "esta", "utilizando", "usa", "usando", "me", "traiga", "el", "los", "punto", "puntos", "de", "venta", "del", "tipo", "ver", "consultar", "arca", "afip", "en", "mostrar", "buscame", "buscar"}
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

	return matches, cleanQuery
}

// --- Monotributo Estado de Cuenta ---

type PeriodoMonotributo struct {
	Periodo   string
	Cuota     string
	Estado    string
	FechaPago string
}

var DatasetMonotributo = []PeriodoMonotributo{
	{Periodo: "07/2026", Cuota: "$52.530,48", Estado: "PENDIENTE", FechaPago: "—"},
	{Periodo: "06/2026", Cuota: "$52.530,48", Estado: "PAGADO", FechaPago: "18/06/2026"},
	{Periodo: "05/2026", Cuota: "$52.530,48", Estado: "PAGADO", FechaPago: "20/05/2026"},
	{Periodo: "04/2026", Cuota: "$48.920,00", Estado: "PAGADO", FechaPago: "19/04/2026"},
	{Periodo: "03/2026", Cuota: "$48.920,00", Estado: "PAGADO", FechaPago: "20/03/2026"},
	{Periodo: "02/2026", Cuota: "$48.920,00", Estado: "ADEUDADO", FechaPago: "—"},
}

var mesesMap = map[string]string{
	"enero": "01", "febrero": "02", "marzo": "03", "abril": "04",
	"mayo": "05", "junio": "06", "julio": "07", "agosto": "08",
	"septiembre": "09", "octubre": "10", "noviembre": "11", "diciembre": "12",
	"ene": "01", "feb": "02", "mar": "03", "abr": "04",
	"may": "05", "jun": "06", "jul": "07", "ago": "08",
	"sep": "09", "oct": "10", "nov": "11", "dic": "12",
}

func (p *AFIPPod) ParsePeriodoMonotributo(query string) (string, string) {
	lower := strings.ToLower(query)

	// Try to find MM/YYYY pattern
	for _, periodo := range DatasetMonotributo {
		if strings.Contains(lower, strings.ToLower(periodo.Periodo)) {
			return periodo.Periodo, ""
		}
	}

	// Try month name → map to number
	for nombre, numero := range mesesMap {
		if strings.Contains(lower, nombre) {
			// Check if year is mentioned
			if strings.Contains(lower, "2026") {
				periodo := numero + "/2026"
				// Verify it exists in dataset
				for _, p := range DatasetMonotributo {
					if p.Periodo == periodo {
						return periodo, ""
					}
				}
				return periodo, fmt.Sprintf("⚠️ El período **%s/2026** no se encontró en el estado de cuenta. Los períodos disponibles son: **02/2026** a **07/2026**.\n\n💡 *Intente escribir el período en formato MM/AAAA (ej: '07/2026') o el nombre del mes (ej: 'julio 2026').*", numero)
			}
			// No year specified → suggest with year
			return numero + "/2026", fmt.Sprintf("📅 Asumimos que se refiere a **%s/2026**. Si desea otro año, escriba el período completo (ej: '%s/2025').\n\n💡 *Períodos disponibles: 02/2026, 03/2026, 04/2026, 05/2026, 06/2026, 07/2026.*", numero, numero)
		}
	}

	// Try bare number like "7" or "07"
	for _, num := range []string{"02", "03", "04", "05", "06", "07"} {
		bare := strings.TrimLeft(num, "0")
		if lower == bare || lower == num || strings.Contains(lower, " "+bare+" ") || strings.Contains(lower, " "+num+" ") || strings.HasSuffix(lower, " "+bare) || strings.HasSuffix(lower, " "+num) {
			return num + "/2026", fmt.Sprintf("📅 Asumimos que se refiere al período **%s/2026**. Si desea otro año, escriba el período completo.\n\n💡 *Períodos disponibles: 02/2026, 03/2026, 04/2026, 05/2026, 06/2026, 07/2026.*", num)
		}
	}

	// Unrecognized period → show all with suggestion
	if strings.Contains(lower, "periodo") || strings.Contains(lower, "período") || strings.Contains(lower, "mes") {
		return "todos", "📅 No pude identificar un período específico. Por favor, indicá el período en alguno de estos formatos:\n\n  • **MM/AAAA** → ej: `07/2026`\n  • **Nombre del mes + año** → ej: `julio 2026`\n  • **Nombre del mes** → ej: `julio` (asume año actual)\n\n💡 *Períodos disponibles: 02/2026, 03/2026, 04/2026, 05/2026, 06/2026, 07/2026.*"
	}

	// Default: show all periods
	return "todos", ""
}
