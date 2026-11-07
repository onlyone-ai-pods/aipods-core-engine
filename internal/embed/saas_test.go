package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAPI answers /embeddings like Voyage/OpenAI, in reverse index order to
// check that the client reorders.
func fakeAPI(t *testing.T, dim int, seen *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad request", http.StatusUnauthorized)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		*seen = append(*seen, body)
		input := body["input"].([]any)
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		data := make([]item, 0, len(input))
		for i := len(input) - 1; i >= 0; i-- {
			v := make([]float32, dim)
			v[0] = float32(len(input[i].(string)))
			data = append(data, item{Index: i, Embedding: v})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

// SPEC-CORE-70.R2
func TestVoyageEmbedder(t *testing.T) {
	var seen []map[string]any
	srv := fakeAPI(t, 1024, &seen)
	defer srv.Close()

	e := NewVoyageEmbedder("k", "", 0, srv.URL)
	if e.Name() != "voyage:voyage-3.5-lite" || e.Dimension() != 1024 {
		t.Fatalf("defaults: %s %d", e.Name(), e.Dimension())
	}
	texts := make([]string, saasBatchSize+3)
	for i := range texts {
		texts[i] = strings.Repeat("a", i+1)
	}
	vecs, err := e.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != len(texts) || len(seen) != 2 {
		t.Fatalf("got %d vectors in %d calls", len(vecs), len(seen))
	}
	for i, v := range vecs {
		if int(v[0]) != i+1 {
			t.Fatalf("vector %d out of order", i)
		}
	}
	if seen[0]["model"] != "voyage-3.5-lite" || seen[0]["output_dimension"] != float64(1024) {
		t.Errorf("request = %v", seen[0])
	}
}

func TestVoyageFixedDimensionModel(t *testing.T) {
	var seen []map[string]any
	srv := fakeAPI(t, 512, &seen)
	defer srv.Close()
	e := NewVoyageEmbedder("k", "voyage-3-lite", 512, srv.URL)
	if _, err := e.Embed(context.Background(), []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen[0]["output_dimension"]; ok {
		t.Error("voyage-3-lite does not accept output_dimension")
	}
}

func TestOpenAIEmbedder(t *testing.T) {
	var seen []map[string]any
	srv := fakeAPI(t, 1024, &seen)
	defer srv.Close()
	e := NewOpenAIEmbedder("k", "", 1024, srv.URL)
	if e.Name() != "openai:text-embedding-3-small" {
		t.Fatalf("name %s", e.Name())
	}
	vecs, err := e.Embed(context.Background(), []string{"uno", "dos"})
	if err != nil || len(vecs) != 2 {
		t.Fatalf("got %d, %v", len(vecs), err)
	}
	if seen[0]["dimensions"] != float64(1024) {
		t.Errorf("request = %v", seen[0])
	}
}

func TestAPIEmbedderErrors(t *testing.T) {
	var seen []map[string]any
	srv := fakeAPI(t, 256, &seen)
	defer srv.Close()

	if _, err := NewVoyageEmbedder("k", "", 1024, srv.URL).Embed(context.Background(), []string{"x"}); err == nil || !strings.Contains(err.Error(), "dimension 256") {
		t.Errorf("dimension mismatch must fail, got %v", err)
	}
	if _, err := NewOpenAIEmbedder("wrong", "", 256, srv.URL).Embed(context.Background(), []string{"x"}); err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Errorf("HTTP error must fail, got %v", err)
	}
	if _, err := NewOpenAIEmbedder("", "", 256, srv.URL).Embed(context.Background(), []string{"x"}); err == nil {
		t.Error("missing key must fail")
	}
	srv.Close()
	if _, err := NewVoyageEmbedder("k", "", 256, srv.URL).Embed(context.Background(), []string{"x"}); err == nil {
		t.Error("network failure must fail")
	}
}
