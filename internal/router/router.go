package router

import (
	"context"
	"strings"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/pod/afip"
	githubdevops "github.com/martinllanos/only-ai-pods/internal/pod/github_devops"
)

type SmartRouter struct {
	pods map[string]pod.BaseAIPod
}

func NewSmartRouter() *SmartRouter {
	router := &SmartRouter{
		pods: make(map[string]pod.BaseAIPod),
	}

	// Register Default Pods
	router.RegisterPod(afip.NewAFIPPod())
	router.RegisterPod(githubdevops.NewGitHubDevOpsPod())

	return router
}

func (r *SmartRouter) RegisterPod(p pod.BaseAIPod) {
	r.pods[p.ID()] = p
}

func (r *SmartRouter) RouteAndExecute(ctx context.Context, tenantID, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var targetPod pod.BaseAIPod

	if strings.Contains(lowerQuery, "github") || strings.Contains(lowerQuery, "odoo.sh") || strings.Contains(lowerQuery, "repo") || strings.Contains(lowerQuery, "despliegue") {
		targetPod = r.pods["POD_GITHUB_DEVOPS"]
	} else {
		// Default To AFIP/Fiscal Pod
		targetPod = r.pods["POD_AFIP_FISCAL"]
	}

	return targetPod.ProcessQuery(ctx, tenantID, query, dryRun)
}
