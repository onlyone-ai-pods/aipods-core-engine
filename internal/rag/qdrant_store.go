package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/embed"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// pointNamespace makes point IDs stable per chunk, so re-ingesting a document
// overwrites its points instead of duplicating them.
var pointNamespace = uuid.MustParse("6f1c8a52-3c1e-4d55-9a0e-2b7f4e8d9c10")

// QdrantStore talks to Qdrant over its REST API (SPEC-CORE-66.R5).
type QdrantStore struct {
	baseURL    string
	apiKey     string
	collection string
	embedder   embed.Embedder
	client     *http.Client

	mu    sync.Mutex
	ready bool
}

func NewQdrantStore(baseURL, apiKey, collection string, e embed.Embedder) *QdrantStore {
	return &QdrantStore{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		collection: collection,
		embedder:   e,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (q *QdrantStore) Name() string { return "qdrant+" + q.embedder.Name() }

func (q *QdrantStore) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, rdr)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("qdrant %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp.StatusCode, fmt.Errorf("qdrant %s %s: status %d: %s", method, path, resp.StatusCode, msg)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("qdrant %s %s: decode: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

// ensureCollection creates the collection and the tenant_id index if missing.
// On failure it retries on the next call.
func (q *QdrantStore) ensureCollection(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ready {
		return nil
	}
	path := "/collections/" + q.collection
	status, err := q.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		if status != http.StatusNotFound {
			return err
		}
		if _, err := q.do(ctx, http.MethodPut, path, map[string]any{
			"vectors": map[string]any{"size": q.embedder.Dimension(), "distance": "Cosine"},
		}, nil); err != nil {
			return err
		}
		if _, err := q.do(ctx, http.MethodPut, path+"/index", map[string]any{
			"field_name": "tenant_id", "field_schema": "keyword",
		}, nil); err != nil {
			return err
		}
	}
	q.ready = true
	return nil
}

func (q *QdrantStore) StoreChunks(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	if err := q.ensureCollection(ctx); err != nil {
		return err
	}
	if err := embedMissing(ctx, q.embedder, chunks); err != nil {
		return err
	}
	points := make([]map[string]any, 0, len(chunks))
	for _, c := range chunks {
		if c.TenantID == "" {
			return fmt.Errorf("rag: chunk %s has no tenant_id", c.ChunkID)
		}
		points = append(points, map[string]any{
			"id":     uuid.NewSHA1(pointNamespace, []byte(c.TenantID+"/"+c.FileName+"/"+c.ChunkID)).String(),
			"vector": c.Embedding,
			"payload": map[string]any{
				"chunk_id":    c.ChunkID,
				"tenant_id":   c.TenantID,
				"file_name":   c.FileName,
				"page_number": c.PageNumber,
				"content":     c.Content,
				"created_at":  c.CreatedAt,
			},
		})
	}
	_, err := q.do(ctx, http.MethodPut, "/collections/"+q.collection+"/points?wait=true", map[string]any{"points": points}, nil)
	return err
}

func (q *QdrantStore) SimilaritySearch(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, ErrNoTenant
	}
	if err := q.ensureCollection(ctx); err != nil {
		return nil, err
	}
	vecs, err := q.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	var out struct {
		Result []struct {
			Score   float64 `json:"score"`
			Payload struct {
				ChunkID    string `json:"chunk_id"`
				TenantID   string `json:"tenant_id"`
				FileName   string `json:"file_name"`
				PageNumber int    `json:"page_number"`
				Content    string `json:"content"`
			} `json:"payload"`
		} `json:"result"`
	}
	_, err = q.do(ctx, http.MethodPost, "/collections/"+q.collection+"/points/search", map[string]any{
		"vector":          vecs[0],
		"limit":           topK,
		"with_payload":    true,
		"score_threshold": minScore,
		// INVARIANT enforced server-side: tenant_id IN (current, GLOBAL).
		"filter": map[string]any{
			"must": []any{map[string]any{
				"key":   "tenant_id",
				"match": map[string]any{"any": []string{tenantID, "GLOBAL"}},
			}},
		},
	}, &out)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(out.Result))
	for _, r := range out.Result {
		// Defense in depth: never return another tenant's chunk.
		if r.Payload.TenantID != tenantID && r.Payload.TenantID != "GLOBAL" {
			continue
		}
		chk := DocumentChunk{
			ChunkID:    r.Payload.ChunkID,
			TenantID:   r.Payload.TenantID,
			FileName:   r.Payload.FileName,
			PageNumber: r.Payload.PageNumber,
			Content:    r.Payload.Content,
		}
		results = append(results, SearchResult{Chunk: chk, Score: r.Score, CitationID: citationFor(chk)})
	}
	return results, nil
}
