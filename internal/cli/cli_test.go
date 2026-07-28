package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/cli/audit"
	"github.com/martinllanos/only-ai-pods/internal/cli/register"
	"github.com/martinllanos/only-ai-pods/internal/cli/scaffold"
	"github.com/martinllanos/only-ai-pods/internal/cli/validator"
)

func TestCLIScaffoldAndValidator(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "aipods_cli_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	podName := "POD_TEST_SCRIBER"
	err = scaffold.GeneratePodBoilerplate(tempDir, podName, "go")
	if err != nil {
		t.Fatalf("failed to generate pod boilerplate: %v", err)
	}

	generatedPodPath := filepath.Join(tempDir, "pod_test_scriber")
	if _, err := os.Stat(filepath.Join(generatedPodPath, "pod.json")); os.IsNotExist(err) {
		t.Fatalf("pod.json missing in scaffolded directory")
	}

	report, err := validator.ValidatePodPipeline(generatedPodPath, true)
	if err != nil {
		t.Fatalf("validation pipeline error: %v", err)
	}

	if !report.AllPass {
		t.Fatalf("scaffolded pod failed validation pipeline")
	}
}

func TestCLIRegister(t *testing.T) {
	resp, err := register.RegisterDynamicPod("http://localhost:8080", "POD_TEST_SCRIBER", "http://localhost:9095", "test,mock")
	if err != nil {
		t.Fatalf("failed to register pod: %v", err)
	}

	if resp.PodID != "POD_TEST_SCRIBER" {
		t.Fatalf("expected PodID POD_TEST_SCRIBER, got %s", resp.PodID)
	}
}

func TestCLIAuditDossier(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "aipods_audit_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	summary, err := audit.GenerateAuditDossier(tempDir, "global", "GLOBAL")
	if err != nil {
		t.Fatalf("failed to generate audit dossier: %v", err)
	}

	if summary.SHA256Hash == "" {
		t.Fatalf("expected valid SHA256 hash")
	}

	valid, _, err := audit.VerifyDossierManifest(summary.ReportPath, summary.ManifestPath)
	if err != nil {
		t.Fatalf("failed to verify dossier manifest: %v", err)
	}

	if !valid {
		t.Fatalf("dossier manifest verification failed")
	}
}
