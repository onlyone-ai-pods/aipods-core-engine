package router

import (
	"context"
	"strings"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/pod/afip"
)

type SmartRouter struct {
	pods map[string]pod.BaseAIPod
}

func NewSmartRouter() *SmartRouter {
	afipPod := afip.NewAFIPPod()
	return &SmartRouter{
		pods: map[string]pod.BaseAIPod{
			afipPod.ID(): afipPod,
		},
	}
}

func (r *SmartRouter) RouteAndExecute(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	// Default to AFIP Pod for Sprint 1 MVP
	targetPodID := "POD_AFIP_FINANCE"

	if strings.Contains(lowerQuery, "afip") || strings.Contains(lowerQuery, "csr") || strings.Contains(lowerQuery, "factura") || strings.Contains(lowerQuery, "balance") {
		targetPodID = "POD_AFIP_FINANCE"
	}

	selectedPod, ok := r.pods[targetPodID]
	if !ok {
		selectedPod = r.pods["POD_AFIP_FINANCE"]
	}

	return selectedPod.ProcessQuery(ctx, tenantID, query, dryRun)
}
