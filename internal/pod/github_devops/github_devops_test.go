package githubdevops

import (
	"context"
	"strings"
	"testing"
)

func TestGitHubDevOpsPod(t *testing.T) {
	pod := NewGitHubDevOpsPod()

	if pod.ID() != "POD_GITHUB_DEVOPS" {
		t.Fatalf("Expected ID POD_GITHUB_DEVOPS, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: GitHub Repo Creation with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero crear un nuevo repositorio en GitHub para mi módulo Odoo", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "gh repo create") {
		t.Errorf("Expected gh repo command in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "crear_repositorio_modulo_github" {
		t.Errorf("Expected ActionName crear_repositorio_modulo_github, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: Odoo.sh Deployment Linking with DryRun
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Vincular rama staging en odoo.sh", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "vincular_despliegue_odoo_sh" {
		t.Errorf("Expected ActionName vincular_despliegue_odoo_sh, got %s", res2.DryRunResult.ActionName)
	}
}
