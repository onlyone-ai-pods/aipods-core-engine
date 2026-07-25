package scm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type SCMPod struct{}

func NewSCMPod() *SCMPod {
	return &SCMPod{}
}

func (p *SCMPod) ID() string {
	return "POD_SCM_LOGISTICS"
}

func (p *SCMPod) Name() string {
	return "AI Pod Cadena de Suministros (SCM: WMS / MRP / Compras)"
}

func (p *SCMPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "wms") || strings.Contains(lowerQuery, "ruta") || strings.Contains(lowerQuery, "push") || strings.Contains(lowerQuery, "pull") || strings.Contains(lowerQuery, "deposito") {
		answer = fmt.Sprintf("AI Pod SCM & WMS: Para un flujo logístico de 3 pasos (Recepción -> Control de Calidad -> Stock) en la empresa (%s):\n1. Active 'Rutas de Varios Pasos' en Inventario > Ajustes.\n2. Configure las reglas Push/Pull asociando la ubicación virtual WH/Input a WH/QualityControl y finalmente WH/Stock.", tenantID)
		citations = []string{
			"Odoo_WMS_Push_Pull_Guide.pdf (Pagina 18)",
			"SCM_MultiStep_Routes.pdf (Pagina 44)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "configurar_rutas_wms",
				Summary:               fmt.Sprintf("Simulación de configuración de rutas Push/Pull de 3 pasos para %s.", tenantID),
				AffectedRecordsCount:  3,
				GeneratedCommand:      "stock.route.write([WH/Input -> WH/QualityControl -> WH/Stock])",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "mrp") || strings.Contains(lowerQuery, "bom") || strings.Contains(lowerQuery, "material") || strings.Contains(lowerQuery, "kit") || strings.Contains(lowerQuery, "fabricar") {
		answer = "AI Pod SCM & MRP: Para fabricar un kit promocional que no requiere orden de trabajo independiente:\n1. Utilice el tipo de Lista de Materiales 'Kit' (Phantom BoM).\n2. El kit no genera orden de producción; al venderse desglosa automáticamente sus componentes en el albarán de entrega."
		citations = []string{
			"Odoo_MRP_BoM_Manual.pdf (Pagina 22)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "recomendar_bom_mrp",
				Summary:               "Simulación de configuración de Lista de Materiales tipo Kit (Phantom BoM).",
				AffectedRecordsCount:  1,
				GeneratedCommand:      "mrp.bom.create(type='phantom')",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "landed") || strings.Contains(lowerQuery, "flete") || strings.Contains(lowerQuery, "costo") || strings.Contains(lowerQuery, "aduana") {
		answer = "AI Pod SCM & Compras: Para imputar gastos de flete marítimo o aranceles aduaneros (Landed Costs):\n1. Habilite 'Landed Costs' en Compras > Ajustes.\n2. Cree un tipo de producto 'Gastos de Destino' y elija el método de prorrateo: Igual, Por Cantidad, Por Valor Actual o Por Peso."
		citations = []string{
			"Odoo_Landed_Costs_Spec.pdf (Pagina 15)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "calcular_landed_costs",
				Summary:               "Simulación de prorrateo de costos de flete (Landed Costs).",
				AffectedRecordsCount:  1000,
				GeneratedCommand:      "stock.landed.cost.compute(split_method='by_quantity')",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod Cadena de Suministros (SCM): Asistencia en WMS, MRP y Compras activa para (%s).", tenantID)
		citations = []string{"Manual_SCM_Logistics.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
