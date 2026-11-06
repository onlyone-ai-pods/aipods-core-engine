package vault

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/store"
)

func TestNativeVaultAES256GCM(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	vm, err := NewNativeVaultManager("aipods_enterprise_aes256_key_32b", st)
	if err != nil {
		t.Fatalf("Failed to initialize NativeVaultManager: %v", err)
	}

	plainText := "mi_secreto_super_confidencial_2026"
	item, err := vm.StoreSecret(ctx, "tenant_acme", "TEST_SECRET_KEY", plainText)
	if err != nil {
		t.Fatalf("Failed to store secret: %v", err)
	}
	if item.Algorithm != "AES-256-GCM" || !strings.HasSuffix(item.MaskedValue, "2026") {
		t.Errorf("unexpected item %+v", item)
	}

	// Only cipher text reaches the store.
	stored, _ := st.GetSecret(ctx, "tenant_acme", "TEST_SECRET_KEY")
	if strings.Contains(stored.CipherText, plainText) || !strings.HasPrefix(stored.CipherText, "aes256gcm:v1:") {
		t.Fatalf("store must hold only cipher text, got %q", stored.CipherText)
	}

	revealed, err := vm.RevealSecret(ctx, "tenant_acme", "TEST_SECRET_KEY")
	if err != nil || revealed != plainText {
		t.Fatalf("reveal: %q %v", revealed, err)
	}

	if _, err := vm.RevealSecret(ctx, "tenant_otro", "TEST_SECRET_KEY"); !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("SECURITY: another tenant revealed the secret, err=%v", err)
	}

	// A new manager over the same store (= restart) still decrypts.
	vm2, _ := NewNativeVaultManager("aipods_enterprise_aes256_key_32b", st)
	if got, err := vm2.RevealSecret(ctx, "tenant_acme", "TEST_SECRET_KEY"); err != nil || got != plainText {
		t.Errorf("secret lost after restart: %q %v", got, err)
	}
}

func TestNativeVaultRejectsBadKey(t *testing.T) {
	if _, err := NewNativeVaultManager("short", store.NewMemory()); !errors.Is(err, ErrInvalidMaster) {
		t.Errorf("expected ErrInvalidMaster, got %v", err)
	}
}
