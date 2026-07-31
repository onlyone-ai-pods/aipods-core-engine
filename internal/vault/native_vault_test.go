package vault

import (
	"testing"
)

func TestNativeVaultAES256GCM(t *testing.T) {
	vm, err := NewNativeVaultManager("aipods_enterprise_aes256_key_32b")
	if err != nil {
		t.Fatalf("Failed to initialize NativeVaultManager: %v", err)
	}

	secretKey := "TEST_SECRET_KEY"
	plainText := "mi_secreto_super_confidencial_2026"

	// 1. Cifrar y Almacenar
	item, err := vm.StoreSecret(secretKey, plainText)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}

	if item.Algorithm != "AES-256-GCM" {
		t.Errorf("Expected algorithm AES-256-GCM, got %s", item.Algorithm)
	}

	// 2. Descifrar en Vivo
	revealed, err := vm.RevealSecret(secretKey)
	if err != nil {
		t.Fatalf("Failed to reveal secret: %v", err)
	}

	if revealed != plainText {
		t.Errorf("Expected revealed plainText '%s', got '%s'", plainText, revealed)
	}
}
