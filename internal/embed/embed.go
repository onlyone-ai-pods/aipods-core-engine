// Package embed turns text into vectors for the RAG engine (SPEC-CORE-66).
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// Embedder converts texts into dense vectors of a fixed dimension.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	Name() string
}

// OllamaEmbedder calls a local Ollama server (POST /api/embed), so documents
// never leave the host to be vectorized.
type OllamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

// NewOllamaEmbedder builds an embedder. dimension must match the model
// (bge-m3 = 1024).
func NewOllamaEmbedder(baseURL, model string, dimension int) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL:   strings.TrimRight(baseURL, "/"),
		model:     model,
		dimension: dimension,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OllamaEmbedder) Dimension() int { return o.dimension }
func (o *OllamaEmbedder) Name() string   { return "ollama:" + o.model }

func (o *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{"model": o.model, "input": texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d", resp.StatusCode)
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama embed: decode: %w", err)
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed: got %d vectors for %d texts", len(out.Embeddings), len(texts))
	}
	for i, v := range out.Embeddings {
		if len(v) != o.dimension {
			return nil, fmt.Errorf("ollama embed: vector %d has dimension %d, expected %d", i, len(v), o.dimension)
		}
	}
	return out.Embeddings, nil
}

// HashEmbedder is a deterministic bag-of-words embedder for tests and for
// local development without Ollama. It is NOT semantic.
type HashEmbedder struct{ dimension int }

func NewHashEmbedder(dimension int) *HashEmbedder {
	if dimension <= 0 {
		dimension = 256
	}
	return &HashEmbedder{dimension: dimension}
}

func (h *HashEmbedder) Dimension() int { return h.dimension }
func (h *HashEmbedder) Name() string   { return "hash" }

func (h *HashEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, h.dimension)
		words := strings.FieldsFunc(strings.ToLower(t), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		for _, w := range words {
			f := fnv.New32a()
			_, _ = f.Write([]byte(w))
			vec[int(f.Sum32())%h.dimension]++
		}
		var norm float64
		for _, x := range vec {
			norm += float64(x * x)
		}
		if norm > 0 {
			n := float32(math.Sqrt(norm))
			for j := range vec {
				vec[j] /= n
			}
		}
		out[i] = vec
	}
	return out, nil
}
