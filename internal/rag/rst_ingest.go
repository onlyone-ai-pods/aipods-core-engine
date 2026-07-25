package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

type RSTIngestor struct {
	sanitizer *security.FileSanitizer
}

func NewRSTIngestor() *RSTIngestor {
	return &RSTIngestor{
		sanitizer: security.NewFileSanitizer(),
	}
}

// IngestRSTText parses reStructuredText (.rst) documents by section underlines (===, ---)
func (r *RSTIngestor) IngestRSTText(ctx context.Context, tenantID, fileName, rawText string) ([]DocumentChunk, error) {
	// MANDATORY SECURITY GATE: Anti-Prompt Injection & Zero-Width Sanitization
	sanitizationRes := r.sanitizer.SanitizeTextContent(rawText)
	cleanText := sanitizationRes.SanitizedContent

	lines := strings.Split(cleanText, "\n")
	var chunks []DocumentChunk
	var currentBuffer []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		currentBuffer = append(currentBuffer, line)

		// Check if next line is a reST section underline (=== or ---)
		if i+1 < len(lines) && isRSTUnderline(lines[i+1]) {
			chunkText := strings.TrimSpace(strings.Join(currentBuffer, "\n"))
			if len(chunkText) > 0 {
				chunkID := fmt.Sprintf("chk_rst_%s", uuid.New().String()[:8])
				chunks = append(chunks, DocumentChunk{
					ChunkID:    chunkID,
					TenantID:   tenantID,
					FileName:   fileName,
					PageNumber: 1,
					Content:    chunkText,
					Metadata: map[string]interface{}{
						"source":    fileName,
						"tenant_id": tenantID,
						"doc_type":  "restructuredtext",
					},
					CreatedAt: time.Now(),
				})
			}
			currentBuffer = nil
		}
	}

	if len(currentBuffer) > 0 {
		chunkText := strings.TrimSpace(strings.Join(currentBuffer, "\n"))
		if len(chunkText) > 0 {
			chunkID := fmt.Sprintf("chk_rst_%s", uuid.New().String()[:8])
			chunks = append(chunks, DocumentChunk{
				ChunkID:    chunkID,
				TenantID:   tenantID,
				FileName:   fileName,
				PageNumber: 1,
				Content:    chunkText,
				Metadata: map[string]interface{}{
					"source":    fileName,
					"tenant_id": tenantID,
					"doc_type":  "restructuredtext",
				},
				CreatedAt: time.Now(),
			})
		}
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("empty reST content in document %s", fileName)
	}

	return chunks, nil
}

func isRSTUnderline(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	char := trimmed[0]
	if char != '=' && char != '-' && char != '~' && char != '`' {
		return false
	}
	for i := 1; i < len(trimmed); i++ {
		if trimmed[i] != char {
			return false
		}
	}
	return true
}
