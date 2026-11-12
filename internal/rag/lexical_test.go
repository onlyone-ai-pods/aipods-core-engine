package rag

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

func TestBuildTSQuery(t *testing.T) {
	terms := []string{"Factura E", "exportación de servicios", "RG 5616", "factura e", "x' | !y:*", "  ", "-Monotributo"}
	if got, want := BuildTSQuery(terms, true), "(factura & e) | (exportacion & de & servicios) | (rg & 5616) | (x & y) | monotributo"; got != want {
		t.Errorf("folded got  %q\nwant %q", got, want)
	}
	if got := BuildTSQuery(terms[1:2], false); got != "(exportación & de & servicios)" {
		t.Errorf("accented got %q", got)
	}
	if BuildTSQuery(nil, true) != "" {
		t.Error("no terms must give an empty query")
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("Facturación ELECTRÓNICA Año"); got != "facturacion electronica ano" {
		t.Errorf("got %q", got)
	}
}

func lexicalIndexes(t *testing.T) map[string]LexicalIndex {
	out := map[string]LexicalIndex{"memory": NewMemoryLexical()}
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		p, err := store.OpenPostgres(context.Background(), url)
		if err != nil {
			t.Fatalf("postgres: %v", err)
		}
		if _, err := p.Pool().Exec(context.Background(), "DELETE FROM rag_chunks"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		out["postgres"] = NewPostgresLexical(p.Pool())
	} else {
		t.Log("TEST_DATABASE_URL not set: skipping Postgres lexical tests")
	}
	return out
}

// SPEC-CORE-70.R1: the expanded formal terms find the ARCA chunk that the
// informal query alone would miss, and tenants stay isolated.
func TestLexicalSearch(t *testing.T) {
	chunks := []DocumentChunk{
		{ChunkID: "g1", TenantID: "GLOBAL", FileName: "ARCA_Factura_E.pdf", PageNumber: 3, Content: "Los comprobantes clase E se emiten para la exportación de servicios."},
		{ChunkID: "g2", TenantID: "GLOBAL", FileName: "ARCA_Monotributo.pdf", PageNumber: 1, Content: "Recategorización semestral del Monotributo."},
		{ChunkID: "a1", TenantID: "tenant_acme", FileName: "acme.pdf", PageNumber: 2, Content: "Acme factura servicios al exterior con comprobante E."},
		{ChunkID: "o1", TenantID: "tenant_otro", FileName: "otro.pdf", PageNumber: 1, Content: "Exportación de servicios del tenant otro."},
	}
	for name, idx := range lexicalIndexes(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if err := idx.IndexChunks(ctx, chunks); err != nil {
				t.Fatal(err)
			}
			if err := idx.IndexChunks(ctx, chunks[:1]); err != nil {
				t.Fatalf("re-indexing must be idempotent: %v", err)
			}
			if _, err := idx.Search(ctx, []string{"factura"}, 5); err != ErrNoTenant {
				t.Errorf("search without tenant: %v", err)
			}
			acme := tenant.WithTenantID(ctx, "tenant_acme")

			informal, err := idx.Search(acme, []string{"cómo cobro laburo afuera"}, 5)
			if err != nil {
				t.Fatal(err)
			}
			if len(informal) != 0 {
				t.Errorf("informal query alone should not match, got %d", len(informal))
			}

			hits, err := idx.Search(acme, []string{"cómo cobro laburo afuera", "Exportación de servicios", "Comprobantes clase E"}, 5)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) == 0 || hits[0].Chunk.ChunkID != "g1" || hits[0].CitationID != "ARCA_Factura_E.pdf (Pagina 3)" {
				t.Fatalf("hits = %+v", hits)
			}
			for _, h := range hits {
				if h.Chunk.TenantID == "tenant_otro" {
					t.Error("leaked another tenant's chunk")
				}
			}

			// Spanish stemming: "facturación" finds "factura" in a1's content
			// and, since SPEC-CORE-72.R7, in g1's file name (ARCA_Factura_E.pdf).
			stemmed, err := idx.Search(acme, []string{"facturación"}, 5)
			ids := map[string]bool{}
			for _, h := range stemmed {
				ids[h.Chunk.ChunkID] = true
			}
			if err != nil || len(stemmed) != 2 || !ids["a1"] || !ids["g1"] {
				t.Errorf("stemmed = %+v, %v", stemmed, err)
			}
			// Without accents: "recategorizacion" finds "Recategorización".
			folded, err := idx.Search(acme, []string{"recategorizacion"}, 5)
			if err != nil || len(folded) != 1 || folded[0].Chunk.ChunkID != "g2" {
				t.Errorf("folded = %+v, %v", folded, err)
			}
		})
	}
}

type failingStore struct{}

func (failingStore) StoreChunks(context.Context, []DocumentChunk) error { return errFailing }
func (failingStore) SimilaritySearch(context.Context, string, int) ([]SearchResult, error) {
	return nil, errFailing
}
func (failingStore) Name() string { return "failing" }

var errFailing = errors.New("embeddings API down")

// SPEC-CORE-70.R4/R5: documents land in the lexical index even with no
// vector store, or when the embeddings API fails during ingestion.
func TestIngestIntoLexicalIndex(t *testing.T) {
	ctx := context.Background()
	acme := tenant.WithTenantID(ctx, "tenant_acme")
	doc := []byte("El Monotributo se recategoriza cada seis meses.")

	noVector := NewRAGIngestionService(nil, nil)
	if _, err := noVector.IngestDocument(ctx, "tenant_acme", "a.txt", doc); err == nil {
		t.Error("ingesting with no index at all must fail")
	}
	noVector.SetLexicalIndex(NewMemoryLexical())
	if _, err := noVector.IngestDocument(ctx, "tenant_acme", "a.txt", doc); err != nil {
		t.Fatal(err)
	}
	if _, err := noVector.Search(acme, "monotributo", 5); err != ErrNoVectorStore {
		t.Errorf("vector search without store: %v", err)
	}
	if hits, err := noVector.SearchLexical(acme, []string{"monotributo"}, 5); err != nil || len(hits) != 1 {
		t.Errorf("lexical hits %d, %v", len(hits), err)
	}
	if noVector.StoreName() != "disabled" || noVector.LexicalName() != "memory-lexical" {
		t.Errorf("names %s %s", noVector.StoreName(), noVector.LexicalName())
	}

	broken := NewRAGIngestionService(failingStore{}, nil)
	if _, err := broken.IngestDocument(ctx, "tenant_acme", "a.txt", doc); err == nil {
		t.Error("vector-only ingestion must surface the API error")
	}
	broken.SetLexicalIndex(NewMemoryLexical())
	if _, err := broken.IngestDocument(ctx, "tenant_acme", "a.txt", doc); err != nil {
		t.Errorf("lexical index must keep the document when the vector store fails: %v", err)
	}
	if hits, _ := broken.SearchLexical(acme, []string{"recategoriza"}, 5); len(hits) != 1 {
		t.Errorf("hits %d", len(hits))
	}
}
