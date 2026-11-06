package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// devVaultMasterKey is only used outside production when VAULT_MASTER_KEY is unset.
const devVaultMasterKey = "aipods_dev_only_aes256_key_32by!"

// resolveVaultMasterKey returns the 32-byte AES-256 master key (SPEC-CORE-65).
// VAULT_MASTER_KEY must be 64 hex characters. In production a missing or invalid
// key is an error; elsewhere a missing key falls back to a fixed dev key.
func resolveVaultMasterKey(raw, environment string) (key string, usedDevKey bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if strings.EqualFold(environment, "production") {
			return "", false, fmt.Errorf("VAULT_MASTER_KEY is required when ENVIRONMENT=production")
		}
		return devVaultMasterKey, true, nil
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return "", false, fmt.Errorf("VAULT_MASTER_KEY must be 64 hex characters (32 bytes)")
	}
	return string(decoded), false, nil
}
