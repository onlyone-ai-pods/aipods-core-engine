package afip

import (
	"context"
	"strings"

	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type AFIPPod struct{}

func NewAFIPPod() *AFIPPod {
	return &AFIPPod{}
}

func (p *AFIPPod) ID() string {
	return "POD_AFIP_FINANCE"
}

func (p *AFIPPod) Name() string {
	return "AI Pod AFIP / ARCA & Balances Financieros"
}

func (p *AFIPPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "csr") || strings.Contains(lowerQuery, "clave") || strings.Contains(lowerQuery, "certificado") {
		generatedCmd := "openssl req -new -key privada.key -out pedido.csr"
		answer = "Para generar la clave privada y el archivo CSR para AFIP/ARCA, ejecute el siguiente comando OpenSSL en su terminal:\n\n```bash\n" + generatedCmd + "\n```\n\nPosteriormente, cargue el certificado `.crt` emitido por AFIP en la configuración de la compañía."
		citations = []string{
			"Guia_AFIP_Certificados_v1.pdf (Pagina 4)",
			"Normativa_ARCA_2026.pdf (Pagina 12)",
		}

		if dryRun {
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "generar_csr_afip",
				Summary:               "Simulación de comando OpenSSL para solicitud de certificado AFIP.",
				AffectedRecordsCount:  1,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: true,
				ApprovalToken:         "dryrun_tok_afip_8899a",
			}
		}
	} else {
		answer = "AI Pod AFIP / ARCA: Consulta recibida. Procesando normativas fiscales para la empresa (" + tenantID + ")."
		citations = []string{"Manual_AFIP_General.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
