package gdrive

import (
	"context"
	"strings"
	"testing"
)

func TestGDriveSyncPod(t *testing.T) {
	pod := NewGDriveSyncPod()

	if pod.ID() != "POD_GDRIVE_SYNC" {
		t.Fatalf("Expected ID POD_GDRIVE_SYNC, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: Google Drive Sync Query with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero sincronizar la carpeta de Google Drive con mis documentos", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "Google Drive Sync") {
		t.Errorf("Expected Google Drive Sync in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "sincronizar_carpeta_gdrive" {
		t.Errorf("Expected ActionName sincronizar_carpeta_gdrive, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: GDocs Extraction Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Extraer texto de los archivos gdoc de la nube", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "extraer_del_nube_gdocs" {
		t.Errorf("Expected ActionName extraer_del_nube_gdocs, got %s", res2.DryRunResult.ActionName)
	}
}
