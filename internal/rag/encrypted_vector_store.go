package rag

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

type EncryptedVectorStore struct {
	secretKey []byte
}

func NewEncryptedVectorStore(key32Bytes string) (*EncryptedVectorStore, error) {
	if len(key32Bytes) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes for AES-256")
	}
	return &EncryptedVectorStore{
		secretKey: []byte(key32Bytes),
	}, nil
}

// EncryptVectorFloat32 convierte []float32 a bytes cifrados AES-256-GCM (SPEC-CORE-34)
func (e *EncryptedVectorStore) EncryptVectorFloat32(vec []float32) ([]byte, error) {
	if len(vec) == 0 {
		return nil, errors.New("empty vector cannot be encrypted")
	}

	// 1. Serializar []float32 a buffer de bytes binario
	buf := make([]byte, len(vec)*4)
	for i, f := range vec {
		bits := math.Float32bits(f)
		binary.BigEndian.PutUint32(buf[i*4:], bits)
	}

	// 2. Inicializar cifrador AES-GCM
	block, err := aes.NewCipher(e.secretKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Cifrar el buffer binario
	ciphertext := aesGCM.Seal(nonce, nonce, buf, nil)
	return ciphertext, nil
}

// DecryptVectorFloat32 descifra el payload binario a []float32
func (e *EncryptedVectorStore) DecryptVectorFloat32(ciphertext []byte) ([]float32, error) {
	block, err := aes.NewCipher(e.secretKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plainBytes, err := aesGCM.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, err
	}

	if len(plainBytes)%4 != 0 {
		return nil, errors.New("invalid byte length for float32 array")
	}

	// Reconstruir []float32
	vecLen := len(plainBytes) / 4
	vec := make([]float32, vecLen)
	for i := 0; i < vecLen; i++ {
		bits := binary.BigEndian.Uint32(plainBytes[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}

	return vec, nil
}
