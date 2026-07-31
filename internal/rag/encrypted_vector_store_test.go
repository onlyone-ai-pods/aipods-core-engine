package rag

import (
	"math"
	"testing"
)

func TestEncryptedVectorStoreLifecycle(t *testing.T) {
	key := "aipods_enterprise_aes256_key_32b"
	store, err := NewEncryptedVectorStore(key)
	if err != nil {
		t.Fatalf("Failed to initialize EncryptedVectorStore: %v", err)
	}

	// 1. Crear vector de prueba de 1536 dimensiones
	sampleVec := make([]float32, 1536)
	for i := 0; i < 1536; i++ {
		sampleVec[i] = float32(i) * 0.00123
	}

	// 2. Cifrar Vector
	encryptedBytes, err := store.EncryptVectorFloat32(sampleVec)
	if err != nil {
		t.Fatalf("Failed to encrypt vector: %v", err)
	}

	if len(encryptedBytes) == 0 {
		t.Errorf("Expected non-empty encrypted bytes")
	}

	// 3. Descifrar Vector
	decryptedVec, err := store.DecryptVectorFloat32(encryptedBytes)
	if err != nil {
		t.Fatalf("Failed to decrypt vector: %v", err)
	}

	if len(decryptedVec) != len(sampleVec) {
		t.Errorf("Expected vector length %d, got %d", len(sampleVec), len(decryptedVec))
	}

	// 4. Comparar precisión flotante
	for i := 0; i < len(sampleVec); i++ {
		if math.Abs(float64(decryptedVec[i]-sampleVec[i])) > 1e-6 {
			t.Errorf("Mismatch at index %d: expected %f, got %f", i, sampleVec[i], decryptedVec[i])
		}
	}
}
