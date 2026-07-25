package rag

import (
	"context"
	"fmt"
	"math"

	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

type SearchResult struct {
	Chunk      DocumentChunk `json:"chunk"`
	Score      float64       `json:"score"`
	CitationID string        `json:"citation_id"`
}

type VectorStore struct {
	collectionName string
	qdrantHost     string
	chunks         []DocumentChunk // Memory vector index for development
}

func NewVectorStore(qdrantHost, collectionName string) *VectorStore {
	return &VectorStore{
		collectionName: collectionName,
		qdrantHost:     qdrantHost,
		chunks:         make([]DocumentChunk, 0),
	}
}

// StoreChunks indexes document chunks into the vector store
func (v *VectorStore) StoreChunks(ctx context.Context, chunks []DocumentChunk) error {
	for i := range chunks {
		// Mock 1536-dim embedding vector for testing
		if len(chunks[i].Embedding) == 0 {
			chunks[i].Embedding = generateMockEmbedding(chunks[i].Content)
		}
		v.chunks = append(v.chunks, chunks[i])
	}
	return nil
}

// SimilaritySearch performs vector cosine search enforcing tenant_id isolation invariant
func (v *VectorStore) SimilaritySearch(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		tenantID = "GLOBAL"
	}

	queryEmbedding := generateMockEmbedding(query)
	var results []SearchResult

	for _, chk := range v.chunks {
		// INVARIANT: WHERE (tenant_id == CurrentTenantID OR tenant_id == 'GLOBAL')
		if chk.TenantID != tenantID && chk.TenantID != "GLOBAL" {
			continue
		}

		score := cosineSimilarity(queryEmbedding, chk.Embedding)
		if score > 0.3 {
			citation := fmt.Sprintf("%s (Pagina %d)", chk.FileName, chk.PageNumber)
			results = append(results, SearchResult{
				Chunk:      chk,
				Score:      score,
				CitationID: citation,
			})
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func generateMockEmbedding(text string) []float32 {
	vec := make([]float32, 128)
	for i, char := range text {
		idx := i % 128
		vec[idx] += float32(char) / 255.0
	}
	return vec
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
