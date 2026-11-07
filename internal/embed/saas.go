package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Light embeddings through SaaS APIs (SPEC-CORE-70.R2). Both providers share
// the OpenAI-style /v1/embeddings contract: {model, input[]} in, data[] with
// {index, embedding} out.

const (
	VoyageBaseURL       = "https://api.voyageai.com/v1"
	VoyageDefaultModel  = "voyage-3.5-lite"
	VoyageDefaultDim    = 1024
	OpenAIBaseURL       = "https://api.openai.com/v1"
	OpenAIDefaultModel  = "text-embedding-3-small"
	OpenAIDefaultDim    = 1536
	saasBatchSize       = 64
	saasRequestTimeout  = 30 * time.Second
	maxErrorBodyPreview = 300
)

// voyageFixedDim lists Voyage models that do not accept output_dimension.
var voyageFixedDim = map[string]bool{
	"voyage-3-lite": true, "voyage-3": true, "voyage-large-2": true,
	"voyage-2": true, "voyage-code-2": true, "voyage-law-2": true, "voyage-finance-2": true,
}

// apiEmbedder is the shared client for Voyage AI and OpenAI.
type apiEmbedder struct {
	provider  string
	baseURL   string
	apiKey    string
	model     string
	dimension int
	// dimField is the request field that asks for a reduced dimension
	// ("output_dimension" for Voyage, "dimensions" for OpenAI); "" = not sent.
	dimField string
	client   *http.Client
}

// VoyageEmbedder calls Voyage AI. The dimension is sent as output_dimension
// unless the model has a fixed size (voyage-3-lite = 512).
type VoyageEmbedder struct{ apiEmbedder }

// NewVoyageEmbedder builds a Voyage client. Empty model/baseURL and
// dimension <= 0 use the defaults.
func NewVoyageEmbedder(apiKey, model string, dimension int, baseURL string) *VoyageEmbedder {
	if model == "" {
		model = VoyageDefaultModel
	}
	if dimension <= 0 {
		dimension = VoyageDefaultDim
	}
	dimField := "output_dimension"
	if voyageFixedDim[model] {
		dimField = ""
	}
	return &VoyageEmbedder{newAPIEmbedder("voyage", orDefault(baseURL, VoyageBaseURL), apiKey, model, dimension, dimField)}
}

// OpenAIEmbedder calls OpenAI. The dimension is sent as "dimensions", which
// text-embedding-3-* models accept.
type OpenAIEmbedder struct{ apiEmbedder }

func NewOpenAIEmbedder(apiKey, model string, dimension int, baseURL string) *OpenAIEmbedder {
	if model == "" {
		model = OpenAIDefaultModel
	}
	if dimension <= 0 {
		dimension = OpenAIDefaultDim
	}
	dimField := ""
	if strings.HasPrefix(model, "text-embedding-3") {
		dimField = "dimensions"
	}
	return &OpenAIEmbedder{newAPIEmbedder("openai", orDefault(baseURL, OpenAIBaseURL), apiKey, model, dimension, dimField)}
}

func newAPIEmbedder(provider, baseURL, apiKey, model string, dimension int, dimField string) apiEmbedder {
	return apiEmbedder{
		provider:  provider,
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		model:     model,
		dimension: dimension,
		dimField:  dimField,
		client:    &http.Client{Timeout: saasRequestTimeout},
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (a *apiEmbedder) Dimension() int { return a.dimension }
func (a *apiEmbedder) Name() string   { return a.provider + ":" + a.model }

func (a *apiEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += saasBatchSize {
		end := min(start+saasBatchSize, len(texts))
		vecs, err := a.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (a *apiEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("%s embed: missing API key", a.provider)
	}
	payload := map[string]any{"model": a.model, "input": texts}
	if a.dimField != "" {
		payload[a.dimField] = a.dimension
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s embed: %w", a.provider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyPreview))
		return nil, fmt.Errorf("%s embed: status %d: %s", a.provider, resp.StatusCode, strings.TrimSpace(string(preview)))
	}
	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%s embed: decode: %w", a.provider, err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("%s embed: got %d vectors for %d texts", a.provider, len(parsed.Data), len(texts))
	}
	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	vecs := make([][]float32, len(texts))
	for i, d := range parsed.Data {
		if d.Index != i {
			return nil, errors.New(a.provider + " embed: response indexes do not match the input")
		}
		if len(d.Embedding) != a.dimension {
			return nil, fmt.Errorf("%s embed: vector %d has dimension %d, expected %d (set EMBED_DIM)", a.provider, i, len(d.Embedding), a.dimension)
		}
		vecs[i] = d.Embedding
	}
	return vecs, nil
}
