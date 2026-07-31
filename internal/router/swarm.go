package router

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type SwarmResultItem struct {
	PodID     string           `json:"pod_id"`
	Response  *pod.PodResponse `json:"response"`
	LatencyMs int64            `json:"latency_ms"`
	Error     string           `json:"error,omitempty"`
}

type SwarmResponse struct {
	SwarmExecutionID    string                     `json:"swarm_execution_id"`
	ExecutionTimeMs     int64                      `json:"execution_time_ms"`
	PodsInvolved        []string                   `json:"pods_involved"`
	SynthesizedResponse string                     `json:"synthesized_response"`
	PodDetails          map[string]SwarmResultItem `json:"pod_details"`
}

type SwarmOrchestrator struct {
	smartRouter *DynamicSmartRouter
}

func NewSwarmOrchestrator(smartRouter *DynamicSmartRouter) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		smartRouter: smartRouter,
	}
}

// ExecuteSwarm dispara la ejecución paralela en goroutines de múltiples AI Pods (SPEC-CORE-33)
func (s *SwarmOrchestrator) ExecuteSwarm(ctx context.Context, tenantID, query string, targetPods []string, dryRun bool) (*SwarmResponse, error) {
	startTime := time.Now()
	executionID := fmt.Sprintf("swm_%d", startTime.UnixNano()%1000000)

	if len(targetPods) == 0 {
		// Detección automática de Pods objetivo basados en palabras clave
		queryLower := strings.ToLower(query)
		if strings.Contains(queryLower, "afip") || strings.Contains(queryLower, "factura") || strings.Contains(queryLower, "arca") {
			targetPods = append(targetPods, "POD_AFIP_FISCAL")
		}
		if strings.Contains(queryLower, "odoo") || strings.Contains(queryLower, "billing") {
			targetPods = append(targetPods, "POD_ODOO_ENTERPRISE")
		}
		if strings.Contains(queryLower, "github") || strings.Contains(queryLower, "repo") {
			targetPods = append(targetPods, "POD_GITHUB_DEVOPS")
		}
		if len(targetPods) == 0 {
			targetPods = []string{"POD_AFIP_FISCAL", "POD_ODOO_ENTERPRISE"}
		}
	}

	resultsChan := make(chan SwarmResultItem, len(targetPods))
	var wg sync.WaitGroup

	// Disparo concurrente de goroutines por cada Pod objetivo
	for _, podID := range targetPods {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			pStart := time.Now()

			// Invocación a través del SmartRouter
			res, err := s.smartRouter.RouteAndExecute(ctx, tenantID, query, dryRun)
			pLatency := time.Since(pStart).Milliseconds()

			item := SwarmResultItem{
				PodID:     id,
				Response:  res,
				LatencyMs: pLatency,
			}
			if err != nil {
				item.Error = err.Error()
			}
			resultsChan <- item
		}(podID)
	}

	wg.Wait()
	close(resultsChan)

	// Consolidación de resultados
	podDetails := make(map[string]SwarmResultItem)
	var synthesisParts []string

	for item := range resultsChan {
		podDetails[item.PodID] = item
		if item.Response != nil {
			synthesisParts = append(synthesisParts, fmt.Sprintf("• **%s**: %s", item.PodID, item.Response.Answer))
		}
	}

	totalLatency := time.Since(startTime).Milliseconds()
	synthesizedText := fmt.Sprintf("🐝 **Enjambre Multi-Pod Ejecutado (%d ms):**\n\n%s", totalLatency, strings.Join(synthesisParts, "\n"))

	return &SwarmResponse{
		SwarmExecutionID:    executionID,
		ExecutionTimeMs:     totalLatency,
		PodsInvolved:        targetPods,
		SynthesizedResponse: synthesizedText,
		PodDetails:          podDetails,
	}, nil
}
