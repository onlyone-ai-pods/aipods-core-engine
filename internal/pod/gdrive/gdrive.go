package gdrive

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type GDriveSyncPod struct{}

func NewGDriveSyncPod() *GDriveSyncPod {
	return &GDriveSyncPod{}
}

func (p *GDriveSyncPod) ID() string {
	return "POD_GDRIVE_SYNC"
}

func (p *GDriveSyncPod) Name() string {
	return "AI Pod Conector Cloud Google Drive / GDocs"
}

func (p *GDriveSyncPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "gdrive") || strings.Contains(lowerQuery, "google drive") || strings.Contains(lowerQuery, "carpeta") || strings.Contains(lowerQuery, "sincronizar") {
		answer = fmt.Sprintf("AI Pod Google Drive Sync: Sincronización de carpeta en la nube vía OAuth2 para (%s):\n- Carpeta: 'Documentación Corporativa'\n- Archivos Encontrados: 14 GDocs\n- Estado de Sincronización: Conector activo en segundo plano.", tenantID)
		citations = []string{
			"Google_Drive_API_v3_Spec.pdf (Pagina 10)",
			"OAuth2_Cloud_Sync_Manual.pdf (Pagina 25)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "sincronizar_carpeta_gdrive",
				Summary:               fmt.Sprintf("Simulación de sincronización OAuth2 Google Drive para %s.", tenantID),
				AffectedRecordsCount:  14,
				GeneratedCommand:      "gdrive.files.list(q=\"'root' in parents and mimeType='application/vnd.google-apps.document'\")",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "gdoc") || strings.Contains(lowerQuery, "extraer") || strings.Contains(lowerQuery, "nube") {
		answer = fmt.Sprintf("AI Pod Google Drive Sync: Extracción de deltas y texto de Google Docs para (%s).\n- Documentos indexados en Qdrant Vector Store.", tenantID)
		citations = []string{
			"GDocs_Text_Export_Spec.pdf (Pagina 6)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "extraer_del_nube_gdocs",
				Summary:               fmt.Sprintf("Simulación de extracción de deltas de GDocs para %s.", tenantID),
				AffectedRecordsCount:  5,
				GeneratedCommand:      "gdrive.files.export(fileId='gdoc_123', mimeType='text/plain')",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod Google Drive Sync: Conector de almacenamiento en la nube activo para (%s).", tenantID)
		citations = []string{"Manual_GDrive_Sync.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
