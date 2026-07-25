package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

type DocumentChunk struct {
	ChunkID    string                 `json:"chunk_id"`
	TenantID   string                 `json:"tenant_id"`
	FileName   string                 `json:"file_name"`
	PageNumber int                    `json:"page_number"`
	Content    string                 `json:"content"`
	Embedding  []float32              `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

type PDFIngestor struct {
	chunkSize    int
	chunkOverlap int
	sanitizer    *security.FileSanitizer
}

func NewPDFIngestor(chunkSize, chunkOverlap int) *PDFIngestor {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 50
	}
	return &PDFIngestor{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		sanitizer:    security.NewFileSanitizer(),
	}
}

// IngestPDFText validates PDF magic bytes, sanitizes text anti-poisoning, and splits into chunks
func (p *PDFIngestor) IngestPDFText(ctx context.Context, tenantID, fileName string, rawBytes []byte) ([]DocumentChunk, error) {
	// 1. MANDATORY SECURITY GATE: Validate PDF Magic Bytes & Executable Objects (/JavaScript, /Launch)
	if err := p.sanitizer.ValidatePDFMagicBytes(rawBytes); err != nil {
		return nil, fmt.Errorf("security gate rejected file %s: %w", fileName, err)
	}

	// 2. MANDATORY SECURITY GATE: Anti-Prompt Injection & Zero-Width Sanitization
	sanitizationRes := p.sanitizer.SanitizeTextContent(string(rawBytes))
	cleanText := sanitizationRes.SanitizedContent

	words := strings.Fields(cleanText)
	if len(words) == 0 {
		return nil, fmt.Errorf("empty or unreadable text content in document %s", fileName)
	}

	var chunks []DocumentChunk
	pageNumber := 1

	for i := 0; i < len(words); i += (p.chunkSize - p.chunkOverlap) {
		end := i + p.chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunkText := strings.Join(words[i:end], " ")
		chunkID := fmt.Sprintf("chk_%s", uuid.New().String()[:8])

		chunks = append(chunks, DocumentChunk{
			ChunkID:    chunkID,
			TenantID:   tenantID,
			FileName:   fileName,
			PageNumber: pageNumber,
			Content:    chunkText,
			Metadata: map[string]interface{}{
				"source":           fileName,
				"tenant_id":        tenantID,
				"word_cnt":         end - i,
				"sanitized_threats": len(sanitizationRes.DetectedThreats),
			},
			CreatedAt: time.Now(),
		})

		// Increment page estimate every 250 words
		if (i / 250) + 1 > pageNumber {
			pageNumber = (i / 250) + 1
		}
	}

	return chunks, nil
}
