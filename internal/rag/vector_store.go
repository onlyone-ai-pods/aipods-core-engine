package rag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/martinllanos/only-ai-pods/internal/embed"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

type SearchResult struct {
	Chunk      DocumentChunk `json:"chunk"`
	Score      float64       `json:"score"`
	CitationID string        `json:"citation_id"`
}

// Store indexes and searches document chunks with tenant isolation (SPEC-CORE-66.R5).
type Store interface {
	StoreChunks(ctx context.Context, chunks []DocumentChunk) error
	SimilaritySearch(ctx context.Context, query string, topK int) ([]SearchResult, error)
	Name() string
}

// ErrNoTenant is returned when a search has no tenant in its context (SPEC-CORE-66.R6).
var ErrNoTenant = errors.New("rag: search requires tenant_id in context")

// minScore drops weak matches so the LLM is not fed noise.
const minScore = 0.3

// VectorStore is the in-memory store, used when Qdrant is not configured.
type VectorStore struct {
	mu             sync.RWMutex
	collectionName string
	qdrantHost     string
	embedder       embed.Embedder
	chunks         []DocumentChunk
}

// NewVectorStore keeps the old signature and uses the hash embedder.
func NewVectorStore(qdrantHost, collectionName string) *VectorStore {
	return NewMemoryStore(embed.NewHashEmbedder(256), qdrantHost, collectionName)
}

// NewMemoryStore builds an in-memory store with the given embedder.
func NewMemoryStore(e embed.Embedder, qdrantHost, collectionName string) *VectorStore {
	return &VectorStore{
		collectionName: collectionName,
		qdrantHost:     qdrantHost,
		embedder:       e,
		chunks:         make([]DocumentChunk, 0),
	}
}

func (v *VectorStore) Name() string { return "memory+" + v.embedder.Name() }

// StoreChunks embeds (when needed) and indexes document chunks.
func (v *VectorStore) StoreChunks(ctx context.Context, chunks []DocumentChunk) error {
	if err := embedMissing(ctx, v.embedder, chunks); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.chunks = append(v.chunks, chunks...)
	return nil
}

// SimilaritySearch performs cosine search enforcing the tenant isolation invariant.
func (v *VectorStore) SimilaritySearch(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, ErrNoTenant
	}
	vecs, err := v.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	queryEmbedding := vecs[0]

	v.mu.RLock()
	defer v.mu.RUnlock()
	var results []SearchResult
	for _, chk := range v.chunks {
		// INVARIANT: tenant_id == CurrentTenantID OR tenant_id == 'GLOBAL'
		if chk.TenantID != tenantID && chk.TenantID != "GLOBAL" {
			continue
		}
		score := cosineSimilarity(queryEmbedding, chk.Embedding)
		if score > minScore {
			results = append(results, SearchResult{Chunk: chk, Score: score, CitationID: citationFor(chk)})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func embedMissing(ctx context.Context, e embed.Embedder, chunks []DocumentChunk) error {
	var idx []int
	var texts []string
	for i := range chunks {
		if len(chunks[i].Embedding) == 0 {
			idx = append(idx, i)
			texts = append(texts, chunks[i].Content)
		}
	}
	if len(texts) == 0 {
		return nil
	}
	vecs, err := e.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding chunks: %w", err)
	}
	for k, i := range idx {
		chunks[i].Embedding = vecs[k]
	}
	return nil
}

func citationFor(chk DocumentChunk) string {
	return fmt.Sprintf("%s (Pagina %d)", chk.FileName, chk.PageNumber)
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
