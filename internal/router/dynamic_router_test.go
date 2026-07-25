package router

import (
	"context"
	"strings"
	"testing"
)

func TestDynamicSmartRouterRuntimeRegistration(t *testing.T) {
	router := NewDynamicSmartRouter()
	ctx := context.Background()

	// 1. Verify default static routing to GitHub DevOps Pod
	res1, err := router.RouteAndExecute(ctx, "tenant_acme", "Quiero un nuevo repo en GitHub", true)
	if err != nil {
		t.Fatalf("Static route 1 failed: %v", err)
	}
	if res1.PodID != "POD_GITHUB_DEVOPS" {
		t.Errorf("Expected POD_GITHUB_DEVOPS, got %s", res1.PodID)
	}

	// 2. Dynamically Register a NEW Custom Client Pod AT RUNTIME (No recompilation!)
	newCustomPod := DynamicPodConfig{
		PodID:       "POD_CUSTOM_LOGISTICS_SERVICE",
		Name:        "AI Pod Logística y Despachos Personalizado",
		TenantID:    "tenant_logistics_corp",
		EndpointURL: "http://localhost:8089/process",
		Keywords:    []string{"despacho", "seguimiento", "camion"},
		Status:      "ACTIVE",
	}

	router.RegisterDynamicPod(newCustomPod)

	// 3. Query the NEW dynamically registered Pod
	res2, err := router.RouteAndExecute(ctx, "tenant_logistics_corp", "Cuál es el estado del despacho del camion?", true)
	if err != nil {
		t.Fatalf("Dynamic route failed: %v", err)
	}
	if res2.PodID != "POD_CUSTOM_LOGISTICS_SERVICE" {
		t.Errorf("Expected POD_CUSTOM_LOGISTICS_SERVICE, got %s", res2.PodID)
	}
	if !strings.Contains(res2.Answer, "AI Pod Dinámico") {
		t.Errorf("Expected Dynamic Pod response, got: %s", res2.Answer)
	}
}
