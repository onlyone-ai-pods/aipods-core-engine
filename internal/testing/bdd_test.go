package bddtesting

import (
	"context"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/events"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

func TestBDDFrameworkAndTieredEvals(t *testing.T) {
	ctx := context.Background()

	// Setup Dependencies
	smartRouter := router.NewDynamicSmartRouter()
	vectorStore := rag.NewVectorStore("localhost:6333", "aipods_vectors")
	semanticCache := rag.NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	ragService := rag.NewRAGIngestionService(vectorStore, semanticCache)
	sanitizer := security.NewFileSanitizer()
	dlq := events.NewDeadLetterQueue(3)
	eventBus := events.NewEventBus(dlq)

	bddRunner := NewBDDTestRunner(smartRouter, ragService, sanitizer, eventBus)
	converter := NewUserStoryConverter()

	// 1. Tier 1 Strict Policy Test: Smart Router Routing BDD Scenario
	res1, err := bddRunner.RunSmartRouterBDDScenario(ctx, "tenant_acme", "Quiero consultar los pedidos en SAP S/4HANA", "POD_SAP_ENTERPRISE")
	if err != nil || !res1.Passed {
		t.Fatalf("Tier 1 Smart Router BDD Scenario failed: %v, failure: %s", err, res1.FailureError)
	}

	// 2. Tier 1 Strict Policy Test: Malicious PDF Security Gate BDD Scenario
	maliciousPDF := []byte("%PDF-1.7\n/JavaScript (alert('attack'));")
	res2, err := bddRunner.RunFileSanitizerBDDScenario(ctx, maliciousPDF)
	if err != nil || !res2.Passed {
		t.Fatalf("Tier 1 Security Gate BDD Scenario failed: %v, failure: %s", err, res2.FailureError)
	}

	// 3. Tier 2 Client Rapid Sandbox Test: User Story to BDD Conversion < 2 seconds
	story := "Quiero que mi AI Pod responda dudas sobre la política de vacaciones"
	generated, err := converter.ConvertStoryToBDD(ctx, "tenant_acme", story)
	if err != nil || !generated.Passed {
		t.Fatalf("Tier 2 User Story BDD Conversion failed: %v", err)
	}
	if generated.ExecutionMs >= 2000 {
		t.Errorf("Expected conversion execution time < 2000ms, got %dms", generated.ExecutionMs)
	}
}
