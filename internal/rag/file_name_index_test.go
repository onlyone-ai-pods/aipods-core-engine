package rag

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

func TestSPEC_CORE_72_R7_FileNameWords(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"ARCA_Monotributo_Categorias_Feb-Jul2026.pdf", "arca monotributo categorias feb jul2026"},
		{"Guía Facturación.v2.md", "guia facturacion v2"},
		{"sin_extension", "sin extension"},
		{".pdf", ""},
		{"", ""},
	} {
		if got := strings.Join(fileNameWords(tc.in), " "); got != tc.want {
			t.Errorf("fileNameWords(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := searchText("ARCA_Monotributo.pdf", "Página Única"); got != "pagina unica arca monotributo" {
		t.Errorf("searchText = %q", got)
	}
}

// SPEC-CORE-72.R7: a page that does not repeat the title words is found by
// them, because search_text holds the file name words.
func TestSPEC_CORE_72_R7_PageMatchesFileNameWords(t *testing.T) {
	page := DocumentChunk{ChunkID: "cat_p1", TenantID: GlobalTenant, FileName: "ARCA_Monotributo_Categorias_Feb-Jul2026.pdf", PageNumber: 1,
		Content: "Ingresos brutos anuales hasta $ 10.277.988,13. Impuesto integrado y aportes."}
	for name, idx := range lexicalIndexes(t) {
		t.Run(name, func(t *testing.T) {
			ctx := tenant.WithTenantID(context.Background(), "tenant_acme")
			if err := idx.IndexChunks(ctx, []DocumentChunk{page}); err != nil {
				t.Fatal(err)
			}
			hits, err := idx.Search(ctx, []string{"categorías monotributo"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 1 || hits[0].Chunk.ChunkID != "cat_p1" {
				t.Errorf("page without the title words must match them, got %v", hits)
			}
		})
	}
}

// SPEC-CORE-72.R7: migration 0004 adds the file name words to rows indexed
// before, with the same rule as Go, and tsv follows.
func TestSPEC_CORE_72_R7_MigrationAddsFileNameToExistingRows(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping migration 0004 test")
	}
	ctx := context.Background()
	p, err := store.OpenPostgres(ctx, url)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	defer p.Close()
	migration, err := os.ReadFile("../store/migrations/0004_rag_search_text_file_name.sql")
	if err != nil {
		t.Fatal(err)
	}

	rows := []DocumentChunk{
		{ChunkID: "mig0004_a", FileName: "ARCA_Monotributo_Categorias_Feb-Jul2026.pdf", Content: "Ingresos brutos anuales. Impuesto integrado."},
		{ChunkID: "mig0004_b", FileName: "Guía Facturación.v2.md", Content: "Comprobantes electrónicos."},
		{ChunkID: "mig0004_c", FileName: ".pdf", Content: "Sin nombre."},
	}
	pool := p.Pool()
	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM rag_chunks WHERE chunk_id LIKE 'mig0004_%'`) }
	cleanup()
	defer cleanup()
	for _, c := range rows {
		// As indexed before SPEC-CORE-72: search_text = Normalize(content).
		if _, err := pool.Exec(ctx, `INSERT INTO rag_chunks (chunk_id, tenant_id, file_name, page_number, content, search_text)
			VALUES ($1, 'GLOBAL', $2, 1, $3, $4)`, c.ChunkID, c.FileName, c.Content, Normalize(c.Content)); err != nil {
			t.Fatal(err)
		}
	}
	// Re-run only on these rows' old state: the migration is already applied
	// by OpenPostgres, so execute its SQL directly.
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("migration 0004: %v", err)
	}
	for _, c := range rows {
		var got string
		var matches bool
		if err := pool.QueryRow(ctx, `SELECT search_text, tsv @@ to_tsquery('spanish', 'monotributo & categorias')
			FROM rag_chunks WHERE chunk_id = $1`, c.ChunkID).Scan(&got, &matches); err != nil {
			t.Fatal(err)
		}
		if want := searchText(c.FileName, c.Content); got != want {
			t.Errorf("%s: search_text %q, want %q (same rule as Go)", c.ChunkID, got, want)
		}
		if wantMatch := c.ChunkID == "mig0004_a"; matches != wantMatch {
			t.Errorf("%s: tsv matches title words = %t, want %t", c.ChunkID, matches, wantMatch)
		}
	}
}
