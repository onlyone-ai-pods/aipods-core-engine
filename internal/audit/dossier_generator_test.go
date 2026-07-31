package audit

import (
	"testing"
)

func TestDossierGeneratorLifecycle(t *testing.T) {
	generator := NewDossierGenerator()

	// 1. Generar Dossier
	dossier, err := generator.GenerateDossier("TENANT_DEMO_001")
	if err != nil {
		t.Fatalf("Failed to generate ISO/SOC2 dossier: %v", err)
	}

	if dossier.SHA256DossierSignature == "" {
		t.Errorf("Expected valid SHA-256 signature, got empty string")
	}

	if dossier.SecurityAudit.GosecVulnerabilities != 0 {
		t.Errorf("Expected 0 gosec vulnerabilities, got %d", dossier.SecurityAudit.GosecVulnerabilities)
	}

	// 2. Verificar firma válida
	valid := generator.VerifyDossierSignature(dossier)
	if !valid {
		t.Errorf("Expected dossier signature to be valid")
	}
}
