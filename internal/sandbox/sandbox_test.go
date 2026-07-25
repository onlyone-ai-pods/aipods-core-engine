package sandbox

import (
	"context"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/router"
)

func TestSandboxSessionManager(t *testing.T) {
	smartRouter := router.NewSmartRouter()
	mgr := NewSessionManager(smartRouter)

	session := mgr.CreateEphemeralSession("manual_afip_test.pdf")
	if session.SessionID == "" {
		t.Fatalf("Expected valid SessionID, got empty")
	}

	ctx := context.Background()

	// Query 1
	res1, s1, err := mgr.ExecuteSandboxQuery(ctx, session.SessionID, "Cómo genero mi archivo CSR para AFIP?")
	if err != nil || res1 == nil {
		t.Fatalf("Query 1 failed: %v", err)
	}
	if s1.QueryCount != 1 {
		t.Errorf("Expected QueryCount = 1, got %d", s1.QueryCount)
	}

	// Query 2
	_, s2, err := mgr.ExecuteSandboxQuery(ctx, session.SessionID, "Consulta 2 de prueba")
	if err != nil {
		t.Fatalf("Query 2 failed: %v", err)
	}
	if s2.QueryCount != 2 {
		t.Errorf("Expected QueryCount = 2, got %d", s2.QueryCount)
	}

	// Query 3
	_, s3, err := mgr.ExecuteSandboxQuery(ctx, session.SessionID, "Consulta 3 de prueba")
	if err != nil {
		t.Fatalf("Query 3 failed: %v", err)
	}
	if s3.QueryCount != 3 {
		t.Errorf("Expected QueryCount = 3, got %d", s3.QueryCount)
	}

	// Query 4 (Must fail with ErrMaxQueriesExceeded)
	_, _, err = mgr.ExecuteSandboxQuery(ctx, session.SessionID, "Consulta 4 no permitida")
	if err != ErrMaxQueriesExceeded {
		t.Errorf("Expected ErrMaxQueriesExceeded, got: %v", err)
	}
}
