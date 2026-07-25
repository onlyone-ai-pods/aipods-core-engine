package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/security"
)

type MarkdownIngestor struct {
	sanitizer *security.FileSanitizer
}

func NewMarkdownIngestor() *MarkdownIngestor {
	return &MarkdownIngestor{
		sanitizer: security.NewFileSanitizer(),
	}
}

// IngestMarkdownText parses Markdown documents by header sections (#, ##, ###)
func (m *MarkdownIngestor) IngestMarkdownText(ctx context.Context, tenantID, fileName, rawText string) ([]DocumentChunk, error) {
	// MANDATORY SECURITY GATE: Anti-Prompt Injection & Zero-Width Sanitization
	sanitizationRes := m.sanitizer.SanitizeTextContent(rawText)
	cleanText := sanitizationRes.SanitizedContent

	lines := strings.Split(cleanText, "\n")
	var chunks []DocumentChunk
	currentHeading := "General"
	var currentBuffer []string

	flushChunk := func() {
		if len(currentBuffer) == 0 {
			return
		}
		chunkText := strings.TrimSpace(strings.Join(currentBuffer, "\n"))
		if len(chunkText) > 0 {
			chunkID := fmt.Sprintf("chk_md_%s", uuid.New().String()[:8])
			chunks = append(chunks, DocumentChunk{
				ChunkID:    chunkID,
				TenantID:   tenantID,
				FileName:   fileName,
				PageNumber: 1,
				Content:    chunkText,
				Metadata: map[string]interface{}{
					"source":          fileName,
					"tenant_id":       tenantID,
					"heading_section": currentHeading,
					"doc_type":        "markdown",
				},
				CreatedAt: time.Now(),
			})
		}
		currentBuffer = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flushChunk()
			currentHeading = strings.TrimLeft(trimmed, "# ")
		} else {
			currentBuffer = append(currentBuffer, line)
		}
	}
	flushChunk()

	if len(chunks) == 0 {
		return nil, fmt.Errorf("empty markdown content in document %s", fileName)
	}

	return chunks, nil
}
