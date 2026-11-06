package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "bge-m3" {
			t.Errorf("unexpected model %s", req.Model)
		}
		vecs := make([][]float32, len(req.Input))
		for i := range vecs {
			vecs[i] = []float32{1, 0, 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vecs})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL+"/", "bge-m3", 3)
	vecs, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil || len(vecs) != 2 {
		t.Fatalf("Embed: %v, %d vectors", err, len(vecs))
	}

	wrongDim := NewOllamaEmbedder(srv.URL, "bge-m3", 1024)
	if _, err := wrongDim.Embed(context.Background(), []string{"a"}); err == nil {
		t.Error("expected dimension mismatch error")
	}
}

func TestHashEmbedderIsDeterministic(t *testing.T) {
	e := NewHashEmbedder(64)
	a, _ := e.Embed(context.Background(), []string{"clave fiscal ARCA"})
	b, _ := e.Embed(context.Background(), []string{"clave fiscal ARCA"})
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatal("hash embedder is not deterministic")
		}
	}
}
