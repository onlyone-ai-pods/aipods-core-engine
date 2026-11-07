package rag

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// ErrNoVectorStore is returned by Search when no embedding model is configured.
var ErrNoVectorStore = errors.New("rag: no vector store configured")

// ErrNoLexicalIndex is returned by SearchLexical when no lexical index is configured.
var ErrNoLexicalIndex = errors.New("rag: no lexical index configured")

type RAGIngestionService struct {
	pdfIngestor   *PDFIngestor
	mdIngestor    *MarkdownIngestor
	rstIngestor   *RSTIngestor
	txtIngestor   *PlainTextIngestor
	vectorStore   Store        // nil when no embedding model is configured (SPEC-CORE-70.R5)
	lexical       LexicalIndex // nil = vector search only
	semanticCache *SemanticCacheManager
}

func NewRAGIngestionService(vectorStore Store, semanticCache *SemanticCacheManager) *RAGIngestionService {
	return &RAGIngestionService{
		pdfIngestor:   NewPDFIngestor(500, 50),
		mdIngestor:    NewMarkdownIngestor(),
		rstIngestor:   NewRSTIngestor(),
		txtIngestor:   NewPlainTextIngestor(500, 50),
		vectorStore:   vectorStore,
		semanticCache: semanticCache,
	}
}

// SetLexicalIndex enables full-text search (SPEC-CORE-70). Every ingested
// chunk is then indexed there too.
func (s *RAGIngestionService) SetLexicalIndex(l LexicalIndex) { s.lexical = l }

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

	if s.lexical == nil && s.vectorStore == nil {
		return nil, fmt.Errorf("ingestion failed for document %s: no index configured", fileName)
	}
	if s.lexical != nil {
		if err := s.lexical.IndexChunks(ctx, chunks); err != nil {
			return nil, fmt.Errorf("lexical indexing failed for document %s: %w", fileName, err)
		}
	}
	if s.vectorStore != nil {
		// Index chunks into VectorStore enforcing tenant isolation
		if err := s.vectorStore.StoreChunks(ctx, chunks); err != nil {
			if s.lexical == nil {
				return nil, fmt.Errorf("vector store indexing failed for document %s: %w", fileName, err)
			}
			// The lexical index already has the document, so search keeps
			// working through the fallback (SPEC-CORE-70.R4).
			log.Printf("[rag] WARNING: vector indexing failed for %s (tenant %s), document is only in the lexical index: %v", fileName, tenantID, err)
		}
	}

	return chunks, nil
}

// Search returns the tenant's most similar chunks from the vector store.
func (s *RAGIngestionService) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if s.vectorStore == nil {
		return nil, ErrNoVectorStore
	}
	return s.vectorStore.SimilaritySearch(ctx, query, topK)
}

// SearchLexical returns the tenant's chunks matching any of the terms.
func (s *RAGIngestionService) SearchLexical(ctx context.Context, terms []string, topK int) ([]SearchResult, error) {
	if s.lexical == nil {
		return nil, ErrNoLexicalIndex
	}
	return s.lexical.Search(ctx, terms, topK)
}

// HasVectorStore reports whether vector search is available.
func (s *RAGIngestionService) HasVectorStore() bool { return s.vectorStore != nil }

// StoreName reports which vector store backend is active.
func (s *RAGIngestionService) StoreName() string {
	if s.vectorStore == nil {
		return "disabled"
	}
	return s.vectorStore.Name()
}

// LexicalName reports which lexical index is active.
func (s *RAGIngestionService) LexicalName() string {
	if s.lexical == nil {
		return "disabled"
	}
	return s.lexical.Name()
}
