package router

import (
	"context"
	"testing"
)

func TestSwarmOrchestratorParallelExecution(t *testing.T) {
	smartRouter := NewDynamicSmartRouter()
	swarm := NewSwarmOrchestrator(smartRouter)

	targetPods := []string{"POD_AFIP_FISCAL", "POD_ODOO_ENTERPRISE"}
	ctx := context.Background()

	// 1. Ejecutar Enjambre Concurrente
	res, err := swarm.ExecuteSwarm(ctx, "TENANT_DEMO_001", "Consulta AFIP y Odoo ERP", targetPods, true)
	if err != nil {
		t.Fatalf("Failed to execute Swarm Orchestrator: %v", err)
	}

	if len(res.PodsInvolved) != 2 {
		t.Errorf("Expected 2 pods involved, got %d", len(res.PodsInvolved))
	}

	if res.SwarmExecutionID == "" {
		t.Errorf("Expected valid swarm_execution_id, got empty")
	}

	if res.SynthesizedResponse == "" {
		t.Errorf("Expected non-empty synthesized response")
	}
}
