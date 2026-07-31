package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"io"
	"sync"
	"time"
)

var (
	ErrSecretNotFound = errors.New("secret key not found in Native Vault")
	ErrInvalidMaster  = errors.New("invalid master key length for AES-256")
)

type VaultSecretItem struct {
	KeyName     string    `json:"key_name"`
	CipherText  string    `json:"cipher_text"`
	MaskedValue string    `json:"masked_value"`
	Algorithm   string    `json:"algorithm"`
	CreatedAt   time.Time `json:"created_at"`
}

type NativeVaultManager struct {
	mu        sync.RWMutex
	masterKey []byte
	secrets   map[string]*VaultSecretItem
	plainMap  map[string]string // En memoria RAM efímera para pruebas
}

func NewNativeVaultManager(masterKeyHex string) (*NativeVaultManager, error) {
	// Clave por defecto AES-256 de 32 bytes exactos
	key := []byte("aipods_enterprise_aes256_key_32b")
	if len(masterKeyHex) == 32 {
		key = []byte(masterKeyHex)
	}

	vm := &NativeVaultManager{
		masterKey: key,
		secrets:   make(map[string]*VaultSecretItem),
		plainMap:  make(map[string]string),
	}

	// Cargar secretos mock iniciales cifrados con AES-256
	_, _ = vm.StoreSecret("AFIP_ARCA_CERTIFICATE", "-----BEGIN CERTIFICATE-----\nMIIF...ARCA2026\n-----END CERTIFICATE-----")
	_, _ = vm.StoreSecret("ODOO_ENTERPRISE_API_KEY", "odoo_live_api_9918237465012938")
	_, _ = vm.StoreSecret("GITHUB_DEVOPS_PAT_TOKEN", "ghp_LiveProductionDevOpsToken2026")

	return vm, nil
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

// StoreSecret cifra y almacena un nuevo secreto
func (v *NativeVaultManager) StoreSecret(keyName, plainText string) (*VaultSecretItem, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	cipherText, err := v.EncryptAES256GCM(plainText)
	if err != nil {
		return nil, err
	}

	masked := "••••••••••••"
	if len(plainText) > 4 {
		masked = fmt.Sprintf("••••••••••••%s", plainText[len(plainText)-4:])
	}

	item := &VaultSecretItem{
		KeyName:     keyName,
		CipherText:  cipherText,
		MaskedValue: masked,
		Algorithm:   "AES-256-GCM",
		CreatedAt:   time.Now(),
	}

	v.secrets[keyName] = item
	v.plainMap[keyName] = plainText
	return item, nil
}

// ListSecrets lista todos los secretos enmascarados (sin revelar plano)
func (v *NativeVaultManager) ListSecrets() []*VaultSecretItem {
	v.mu.RLock()
	defer v.mu.RUnlock()

	list := make([]*VaultSecretItem, 0, len(v.secrets))
	for _, item := range v.secrets {
		list = append(list, item)
	}
	return list
}

// RevealSecret descifra el valor efímero en memoria RAM
func (v *NativeVaultManager) RevealSecret(keyName string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	item, exists := v.secrets[keyName]
	if !exists {
		return "", ErrSecretNotFound
	}

	// Decrypt on demand
	return v.DecryptAES256GCM(item.CipherText)
}
