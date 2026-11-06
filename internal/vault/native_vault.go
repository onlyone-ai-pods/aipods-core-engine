package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"io"

	"github.com/martinllanos/only-ai-pods/internal/store"
)

var (
	ErrSecretNotFound = errors.New("secret key not found in Native Vault")
	ErrInvalidMaster  = errors.New("invalid master key length for AES-256")
)

// NativeVaultManager encrypts secrets with AES-256-GCM and persists only the
// cipher text, scoped by tenant (SPEC-CORE-27, SPEC-CORE-67.R8).
type NativeVaultManager struct {
	masterKey []byte
	store     store.Store
}

// NewNativeVaultManager requires an exact 32-byte key; there is no silent default.
func NewNativeVaultManager(masterKey string, st store.Store) (*NativeVaultManager, error) {
	if len(masterKey) != 32 {
		return nil, ErrInvalidMaster
	}
	if st == nil {
		return nil, errors.New("vault: store is required")
	}
	return &NativeVaultManager{masterKey: []byte(masterKey), store: st}, nil
}

// EncryptAES256GCM cifra un texto plano utilizando AES-256-GCM y retorna Base64
func (v *NativeVaultManager) EncryptAES256GCM(plainText string) (string, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherBytes := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return fmt.Sprintf("aes256gcm:v1:%s", base64.StdEncoding.EncodeToString(cipherBytes)), nil
}

// DecryptAES256GCM descifra un texto cifrado en Base64
func (v *NativeVaultManager) DecryptAES256GCM(cipherStr string) (string, error) {
	var encoded string
	_, err := fmt.Sscanf(cipherStr, "aes256gcm:v1:%s", &encoded)
	if err != nil {
		encoded = cipherStr
	}

	cipherBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherBytes) < nonceSize {
		return "", errors.New("ciphertext too short for GCM nonce")
	}

	nonce, ciphertext := cipherBytes[:nonceSize], cipherBytes[nonceSize:]
	plainBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plainBytes), nil
}

// StoreSecret encrypts and stores (or replaces) a tenant secret.
func (v *NativeVaultManager) StoreSecret(ctx context.Context, tenantID, keyName, plainText string) (store.Secret, error) {
	if tenantID == "" || keyName == "" {
		return store.Secret{}, errors.New("vault: tenant_id and key_name are required")
	}
	cipherText, err := v.EncryptAES256GCM(plainText)
	if err != nil {
		return store.Secret{}, err
	}
	masked := "••••••••••••"
	if r := []rune(plainText); len(r) > 8 {
		masked = "••••••••••••" + string(r[len(r)-4:])
	}
	return v.store.PutSecret(ctx, store.Secret{
		TenantID:    tenantID,
		KeyName:     keyName,
		CipherText:  cipherText,
		MaskedValue: masked,
		Algorithm:   "AES-256-GCM",
	})
}

// ListSecrets returns the tenant's secrets masked, never in clear.
func (v *NativeVaultManager) ListSecrets(ctx context.Context, tenantID string) ([]store.Secret, error) {
	return v.store.ListSecrets(ctx, tenantID)
}

// RevealSecret decrypts one tenant secret on demand.
func (v *NativeVaultManager) RevealSecret(ctx context.Context, tenantID, keyName string) (string, error) {
	item, err := v.store.GetSecret(ctx, tenantID, keyName)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrSecretNotFound
	}
	if err != nil {
		return "", err
	}
	return v.DecryptAES256GCM(item.CipherText)
}
