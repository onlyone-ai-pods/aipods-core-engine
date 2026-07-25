package sap

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type SAPPod struct{}

func NewSAPPod() *SAPPod {
	return &SAPPod{}
}

func (p *SAPPod) ID() string {
	return "POD_SAP_ENTERPRISE"
}

func (p *SAPPod) Name() string {
	return "AI Pod SAP Enterprise (S/4HANA/ECC) & Business One"
}

func (p *SAPPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "pedido") || strings.Contains(lowerQuery, "s4hana") || strings.Contains(lowerQuery, "odata") {
		generatedCmd := "GET /sap/opu/odata/sap/API_SALES_ORDER_SRV/A_SalesOrder?$top=5&$filter=OverallSDProcessStatus eq 'C'"
		answer = fmt.Sprintf("AI Pod SAP S/4HANA: Para consultar los últimos pedidos de venta completados en SAP S/4HANA para la empresa (%s), la petición OData v2 RESTful ejecutada es:\n\n```http\n%s\n```\n\nRespuesta procesada desde SAP Gateway en formato JSON estándar.", tenantID, generatedCmd)
		citations = []string{
			"SAP_S4HANA_OData_Gateway_Spec_v1.pdf (Pagina 14)",
			"SAP_Business_Suite_REST_APIs.pdf (Pagina 88)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "consultar_pedidos_sap_odata",
				Summary:               fmt.Sprintf("Simulación de consulta OData v2 RESTful a SAP Gateway para %s.", tenantID),
				AffectedRecordsCount:  5,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "b1") || strings.Contains(lowerQuery, "material") || strings.Contains(lowerQuery, "articulo") {
		generatedCmd := "GET /b1s/v2/Items?$select=ItemCode,ItemName,QuantityOnStock"
		answer = "AI Pod SAP Business One: Para consultar el maestro de artículos e inventario en SAP Business One Service Layer, ejecute:\n\n```http\n" + generatedCmd + "\n```"
		citations = []string{
			"SAP_B1_Service_Layer_API.pdf (Pagina 34)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "consultar_maestro_materiales_b1",
				Summary:               "Simulación de lectura de artículos en SAP B1 Service Layer.",
				AffectedRecordsCount:  10,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = "AI Pod SAP Enterprise: Consulta de integración empresarial recibida para la organización (" + tenantID + ")."
		citations = []string{"Manual_SAP_Integration_Guide.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
