package rag

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
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

// IngestPDFText validates the PDF, extracts its text page by page, sanitizes it
// and splits each page into chunks that keep the real page number.
func (p *PDFIngestor) IngestPDFText(ctx context.Context, tenantID, fileName string, rawBytes []byte) ([]DocumentChunk, error) {
	// Security gate: PDF magic bytes and no executable objects (/JavaScript, /Launch).
	if err := p.sanitizer.ValidatePDFMagicBytes(rawBytes); err != nil {
		return nil, fmt.Errorf("security gate rejected file %s: %w", fileName, err)
	}

	pages, err := extractPDFPages(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot read PDF %s: %w", fileName, err)
	}

	var chunks []DocumentChunk
	step := p.chunkSize - p.chunkOverlap
	if step <= 0 {
		step = p.chunkSize
	}
	for i, pageText := range pages {
		// Security gate: anti prompt injection and zero-width characters.
		res := p.sanitizer.SanitizeTextContent(pageText)
		words := strings.Fields(res.SanitizedContent)
		for start := 0; start < len(words); start += step {
			end := start + p.chunkSize
			if end > len(words) {
				end = len(words)
			}
			chunks = append(chunks, DocumentChunk{
				ChunkID:    fmt.Sprintf("chk_%s", uuid.NewString()),
				TenantID:   tenantID,
				FileName:   fileName,
				PageNumber: i + 1,
				Content:    strings.Join(words[start:end], " "),
				Metadata: map[string]interface{}{
					"source":            fileName,
					"tenant_id":         tenantID,
					"word_cnt":          end - start,
					"sanitized_threats": len(res.DetectedThreats),
				},
				CreatedAt: time.Now(),
			})
			if end == len(words) {
				break
			}
		}
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no extractable text in document %s (scanned images are not supported)", fileName)
	}
	return chunks, nil
}

// extractPDFPages returns the plain text of each page. A malformed PDF can make
// the parser panic, so that is turned into an error.
func extractPDFPages(raw []byte) (pages []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed PDF: %v", r)
		}
	}()
	rd, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	fonts := map[string]*pdf.Font{}
	for i := 1; i <= rd.NumPage(); i++ {
		page := rd.Page(i)
		if page.V.IsNull() {
			pages = append(pages, "")
			continue
		}
		for _, name := range page.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := page.Font(name)
				fonts[name] = &f
			}
		}
		text, err := page.GetPlainText(fonts)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i, err)
		}
		pages = append(pages, text)
	}
	return pages, nil
}
