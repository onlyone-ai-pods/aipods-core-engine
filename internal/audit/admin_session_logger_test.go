package audit

import (
	"testing"
)

func TestAdminSessionLoggerIPAndUserAgent(t *testing.T) {
	logger := NewAdminSessionLogger()

	log, err := logger.LogAdminActivity(
		"admin.consultant@acmecorp.com",
		"ADMIN_LOGIN_SUCCESS",
		"190.210.45.12",
		"Mozilla/5.0 (Tablet; iPad 8inch)",
	)

	if err != nil {
		t.Fatalf("Failed to log admin activity: %v", err)
	}

	if log.ClientIP != "190.210.45.12" {
		t.Errorf("Expected ClientIP 190.210.45.12, got %s", log.ClientIP)
	}

	if log.SHA256Hash == "" {
		t.Errorf("Expected non-empty SHA256Hash signature")
	}
}
