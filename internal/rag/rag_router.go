package rag

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type RAGIngestionService struct {
	pdfIngestor   *PDFIngestor
	mdIngestor    *MarkdownIngestor
	rstIngestor   *RSTIngestor
	txtIngestor   *PlainTextIngestor
	vectorStore   *VectorStore
	semanticCache *SemanticCacheManager
}

func NewRAGIngestionService(vectorStore *VectorStore, semanticCache *SemanticCacheManager) *RAGIngestionService {
	return &RAGIngestionService{
		pdfIngestor:   NewPDFIngestor(500, 50),
		mdIngestor:    NewMarkdownIngestor(),
		rstIngestor:   NewRSTIngestor(),
		txtIngestor:   NewPlainTextIngestor(500, 50),
		vectorStore:   vectorStore,
		semanticCache: semanticCache,
	}
}

// IngestDocument automatically detects file extension, sanitizes, chunks, and indexes into VectorStore
func (s *RAGIngestionService) IngestDocument(ctx context.Context, tenantID, fileName string, rawBytes []byte) ([]DocumentChunk, error) {
	ext := strings.ToLower(filepath.Ext(fileName))

	var chunks []DocumentChunk
	var err error

	switch ext {
	case ".pdf":
		chunks, err = s.pdfIngestor.IngestPDFText(ctx, tenantID, fileName, rawBytes)
	case ".md", ".markdown":
		chunks, err = s.mdIngestor.IngestMarkdownText(ctx, tenantID, fileName, string(rawBytes))
	case ".rst":
		chunks, err = s.rstIngestor.IngestRSTText(ctx, tenantID, fileName, string(rawBytes))
	case ".txt", ".log", "":
		chunks, err = s.txtIngestor.IngestPlainText(ctx, tenantID, fileName, string(rawBytes))
	default:
		// Fallback to Plain Text Ingestor for unknown text files
		chunks, err = s.txtIngestor.IngestPlainText(ctx, tenantID, fileName, string(rawBytes))
	}

	if err != nil {
		return nil, fmt.Errorf("ingestion failed for document %s: %w", fileName, err)
	}

	// Index chunks into VectorStore enforcing tenant isolation
	if err := s.vectorStore.StoreChunks(ctx, chunks); err != nil {
		return nil, fmt.Errorf("vector store indexing failed for document %s: %w", fileName, err)
	}

	return chunks, nil
}
