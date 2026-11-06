package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/embed"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// fakeQdrant keeps points in memory and applies the tenant filter like Qdrant does.
type fakeQdrant struct {
	mu        sync.Mutex
	created   bool
	indexed   bool
	points    []map[string]any
	lastQuery map[string]any
}

func (f *fakeQdrant) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/collections/test":
		if !f.created {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	case r.Method == http.MethodPut && r.URL.Path == "/collections/test":
		f.created = true
		_, _ = w.Write([]byte(`{"result":true}`))
	case r.Method == http.MethodPut && r.URL.Path == "/collections/test/index":
		f.indexed = true
		_, _ = w.Write([]byte(`{"result":{}}`))
	case r.Method == http.MethodPut && r.URL.Path == "/collections/test/points":
		var body struct {
			Points []map[string]any `json:"points"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.points = append(f.points, body.Points...)
		_, _ = w.Write([]byte(`{"result":{}}`))
	case r.Method == http.MethodPost && r.URL.Path == "/collections/test/points/search":
		var q map[string]any
		_ = json.NewDecoder(r.Body).Decode(&q)
		f.lastQuery = q
		allowed := map[string]bool{}
		must := q["filter"].(map[string]any)["must"].([]any)[0].(map[string]any)
		for _, t := range must["match"].(map[string]any)["any"].([]any) {
			allowed[t.(string)] = true
		}
		var res []map[string]any
		for _, p := range f.points {
			payload := p["payload"].(map[string]any)
			if allowed[payload["tenant_id"].(string)] {
				res = append(res, map[string]any{"score": 0.9, "payload": payload})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func TestQdrantStoreTenantIsolation(t *testing.T) {
	fake := &fakeQdrant{}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	store := NewQdrantStore(srv.URL, "", "test", embed.NewHashEmbedder(32))
	ctx := context.Background()
	err := store.StoreChunks(ctx, []DocumentChunk{
		{ChunkID: "1", TenantID: "tenant_acme", FileName: "acme.pdf", PageNumber: 1, Content: "clave fiscal acme"},
		{ChunkID: "2", TenantID: "tenant_otro", FileName: "otro.pdf", PageNumber: 1, Content: "clave fiscal otro"},
		{ChunkID: "3", TenantID: "GLOBAL", FileName: "normativa.pdf", PageNumber: 2, Content: "normativa general"},
	})
	if err != nil {
		t.Fatalf("StoreChunks: %v", err)
	}
	if !fake.created || !fake.indexed {
		t.Fatal("collection and tenant_id index should be created")
	}

	res, err := store.SimilaritySearch(tenant.WithTenantID(ctx, "tenant_acme"), "clave fiscal", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected acme + GLOBAL chunks, got %d", len(res))
	}
	for _, r := range res {
		if r.Chunk.TenantID == "tenant_otro" {
			t.Fatal("SECURITY VIOLATION: got another tenant's chunk")
		}
	}
	if !strings.Contains(res[0].CitationID, "(Pagina") {
		t.Errorf("unexpected citation %q", res[0].CitationID)
	}

	if _, err := store.SimilaritySearch(ctx, "clave", 5); err != ErrNoTenant {
		t.Errorf("search without tenant should fail with ErrNoTenant, got %v", err)
	}
}
