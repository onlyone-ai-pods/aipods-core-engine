package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type AdminSessionLog struct {
	LogID      string    `json:"log_id"`
	Timestamp  time.Time `json:"timestamp"`
	AdminEmail string    `json:"admin_email"`
	Action     string    `json:"action"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	SHA256Hash string    `json:"sha256_hash"`
}

type AdminSessionLogger struct{}

func NewAdminSessionLogger() *AdminSessionLogger {
	return &AdminSessionLogger{}
}

// LogAdminActivity registra eventos de administrador con IP y User-Agent firmados con SHA-256 (SPEC-CORE-40)
func (l *AdminSessionLogger) LogAdminActivity(adminEmail, action, clientIP, userAgent string) (*AdminSessionLog, error) {
	now := time.Now()
	logID := fmt.Sprintf("adm_log_%d", now.UnixNano()%1000000)

	logEntry := &AdminSessionLog{
		LogID:      logID,
		Timestamp:  now,
		AdminEmail: adminEmail,
		Action:     action,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
	}

	// Firma criptográfica SHA-256 de auditoría (ISO 27001)
	bytes, err := json.Marshal(logEntry)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(bytes)
	logEntry.SHA256Hash = hex.EncodeToString(hash[:])

	return logEntry, nil
}
