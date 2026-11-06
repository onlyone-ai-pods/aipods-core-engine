package main

import (
	"strings"
	"testing"
)

func TestResolveVaultMasterKey(t *testing.T) {
	valid := strings.Repeat("ab", 32)

	key, dev, err := resolveVaultMasterKey(valid, "production")
	if err != nil || dev || len(key) != 32 || key[0] != 0xab {
		t.Fatalf("valid key: got len=%d dev=%v err=%v", len(key), dev, err)
	}

	if _, _, err := resolveVaultMasterKey("", "production"); err == nil {
		t.Fatal("expected error for missing key in production")
	}
	if _, _, err := resolveVaultMasterKey("not-hex", "production"); err == nil {
		t.Fatal("expected error for non-hex key")
	}
	if _, _, err := resolveVaultMasterKey(strings.Repeat("ab", 16), "development"); err == nil {
		t.Fatal("expected error for short key")
	}

	key, dev, err = resolveVaultMasterKey("", "development")
	if err != nil || !dev || len(key) != 32 {
		t.Fatalf("dev fallback: got len=%d dev=%v err=%v", len(key), dev, err)
	}
}
