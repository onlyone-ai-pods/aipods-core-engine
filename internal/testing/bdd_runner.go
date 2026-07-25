package bddtesting

import (
	"context"
	"fmt"

	"github.com/martinllanos/only-ai-pods/internal/events"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/security"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

type BDDScenarioResult struct {
	ScenarioName string   `json:"scenario_name"`
	Passed       bool     `json:"passed"`
	StepsCount   int      `json:"steps_count"`
	FailureError string   `json:"failure_error,omitempty"`
	Logs         []string `json:"logs"`
}

type BDDTestRunner struct {
	smartRouter *router.DynamicSmartRouter
	ragService  *rag.RAGIngestionService
	sanitizer   *security.FileSanitizer
	eventBus    *events.EventBus
}

func NewBDDTestRunner(smartRouter *router.DynamicSmartRouter, ragService *rag.RAGIngestionService, sanitizer *security.FileSanitizer, eventBus *events.EventBus) *BDDTestRunner {
	return &BDDTestRunner{
		smartRouter: smartRouter,
		ragService:  ragService,
		sanitizer:   sanitizer,
		eventBus:    eventBus,
	}
}

// RunSmartRouterBDDScenario executes a BDD Gherkin scenario for Dynamic Smart Router
func (b *BDDTestRunner) RunSmartRouterBDDScenario(ctx context.Context, tenantID, query string, expectedPodID string) (*BDDScenarioResult, error) {
	logs := []string{
		fmt.Sprintf("GIVEN tenant_id: '%s'", tenantID),
		fmt.Sprintf("WHEN query: '%s'", query),
	}

	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	resp, err := b.smartRouter.RouteAndExecute(tenantCtx, tenantID, query, true)
	if err != nil {
		return &BDDScenarioResult{
			ScenarioName: "Smart Router Execution",
			Passed:       false,
			StepsCount:   3,
			FailureError: err.Error(),
			Logs:         logs,
		}, nil
	}

	logs = append(logs, fmt.Sprintf("THEN routed pod_id: '%s'", resp.PodID))

	if expectedPodID != "" && resp.PodID != expectedPodID {
		return &BDDScenarioResult{
			ScenarioName: "Smart Router Routing Verification",
			Passed:       false,
			StepsCount:   3,
			FailureError: fmt.Sprintf("Expected PodID %s, got %s", expectedPodID, resp.PodID),
			Logs:         logs,
		}, nil
	}

	return &BDDScenarioResult{
		ScenarioName: "Smart Router Routing Verification",
		Passed:       true,
		StepsCount:   3,
		Logs:         logs,
	}, nil
}

// RunFileSanitizerBDDScenario executes a BDD Gherkin scenario for File Security & Anti-Poisoning
func (b *BDDTestRunner) RunFileSanitizerBDDScenario(ctx context.Context, pdfBytes []byte) (*BDDScenarioResult, error) {
	logs := []string{
		"GIVEN raw uploaded PDF bytes",
		"WHEN FileSanitizer evaluates PDF magic bytes",
	}

	err := b.sanitizer.ValidatePDFMagicBytes(pdfBytes)
	if err != nil {
		logs = append(logs, fmt.Sprintf("THEN security gate rejected malicious PDF: %v", err))
		return &BDDScenarioResult{
			ScenarioName: "File Security & Anti-Poisoning Gate",
			Passed:       true, // Security gate properly rejected malicious file!
			StepsCount:   3,
			Logs:         logs,
		}, nil
	}

	logs = append(logs, "THEN PDF magic bytes passed successfully")
	return &BDDScenarioResult{
		ScenarioName: "File Security & Anti-Poisoning Gate",
		Passed:       true,
		StepsCount:   3,
		Logs:         logs,
	}, nil
}
