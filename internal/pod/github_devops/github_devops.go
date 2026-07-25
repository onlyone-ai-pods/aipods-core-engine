package githubdevops

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type GitHubDevOpsPod struct{}

func NewGitHubDevOpsPod() *GitHubDevOpsPod {
	return &GitHubDevOpsPod{}
}

func (p *GitHubDevOpsPod) ID() string {
	return "POD_GITHUB_DEVOPS"
}

func (p *GitHubDevOpsPod) Name() string {
	return "AI Pod GitHub API & Odoo.sh DevOps Integrator"
}

func (p *GitHubDevOpsPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "repo") || strings.Contains(lowerQuery, "repositorio") || strings.Contains(lowerQuery, "github") {
		generatedCmd := fmt.Sprintf("gh repo create client-org/odoo-custom-%s --private --clone", tenantID)
		answer = fmt.Sprintf("AI Pod GitHub API: Para crear un nuevo repositorio privado para el módulo personalizado de Odoo de la empresa (%s), ejecute el comando en la GitHub CLI:\n\n```bash\n%s\n```\n\nPosteriormente, configure los secretos `ODOO_SH_SSH_KEY` en los GitHub Actions del repositorio para despliegue automático.", tenantID, generatedCmd)
		citations = []string{
			"Odoo_sh_PaaS_DevOps_Guide_v1.pdf (Pagina 8)",
			"GitHub_API_v3_Integration.pdf (Pagina 15)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "crear_repositorio_modulo_github",
				Summary:               fmt.Sprintf("Simulación de creación de repositorio privado en GitHub e integración CI/CD para %s.", tenantID),
				AffectedRecordsCount:  2,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "odoo.sh") || strings.Contains(lowerQuery, "despliegue") || strings.Contains(lowerQuery, "staging") {
		generatedCmd := "odoo-sh project build create --project-id prj-client-01 --branch staging"
		answer = "AI Pod Odoo.sh DevOps: Para vincular una nueva rama de staging en el proyecto de Odoo.sh PaaS, ejecute el comando:\n\n```bash\n" + generatedCmd + "\n```"
		citations = []string{
			"Odoo_sh_PaaS_DevOps_Guide_v1.pdf (Pagina 22)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "vincular_despliegue_odoo_sh",
				Summary:               "Simulación de creación de build de staging en Odoo.sh PaaS.",
				AffectedRecordsCount:  1,
				GeneratedCommand:      generatedCmd,
				RequiresHumanApproval: true,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = "AI Pod GitHub API & Odoo.sh DevOps: Consulta de automatización recibida para la organización (" + tenantID + ")."
		citations = []string{"Manual_DevOps_Odoo_SH.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
