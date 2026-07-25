package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

type PlainTextIngestor struct {
	chunkSize    int
	chunkOverlap int
	sanitizer    *security.FileSanitizer
}

func NewPlainTextIngestor(chunkSize, chunkOverlap int) *PlainTextIngestor {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 50
	}
	return &PlainTextIngestor{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		sanitizer:    security.NewFileSanitizer(),
	}
}

// IngestPlainText parses plain text (.txt) documents into word-level overlapping chunks
func (p *PlainTextIngestor) IngestPlainText(ctx context.Context, tenantID, fileName, rawText string) ([]DocumentChunk, error) {
	// MANDATORY SECURITY GATE: Anti-Prompt Injection & Zero-Width Sanitization
	sanitizationRes := p.sanitizer.SanitizeTextContent(rawText)
	cleanText := sanitizationRes.SanitizedContent

	words := strings.Fields(cleanText)
	if len(words) == 0 {
		return nil, fmt.Errorf("empty plain text content in document %s", fileName)
	}

	var chunks []DocumentChunk

	for i := 0; i < len(words); i += (p.chunkSize - p.chunkOverlap) {
		end := i + p.chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunkText := strings.Join(words[i:end], " ")
		chunkID := fmt.Sprintf("chk_txt_%s", uuid.New().String()[:8])

		chunks = append(chunks, DocumentChunk{
			ChunkID:    chunkID,
			TenantID:   tenantID,
			FileName:   fileName,
			PageNumber: 1,
			Content:    chunkText,
			Metadata: map[string]interface{}{
				"source":    fileName,
				"tenant_id": tenantID,
				"word_cnt":  end - i,
				"doc_type":  "plaintext",
			},
			CreatedAt: time.Now(),
		})
	}

	return chunks, nil
}
