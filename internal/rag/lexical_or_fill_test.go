package rag

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// orFillIndex is a lexical index that reports the OR fill (SPEC-CORE-72.R9).
type orFillIndex interface {
	LexicalIndex
	SetORFallbackHook(fn func())
}

// countedIndex is an index plus the number of OR fills it reported.
type countedIndex struct {
	orFillIndex
	fills *atomic.Int64
}

// orFillIndexes returns the memory index and, with TEST_DATABASE_URL, the
// Postgres one, loaded with chunks and counting the OR fills.
func orFillIndexes(t *testing.T, chunks []DocumentChunk) map[string]countedIndex {
	t.Helper()
	out := map[string]countedIndex{}
	for name, idx := range lexicalIndexes(t) {
		fi, ok := idx.(orFillIndex)
		if !ok {
			t.Fatalf("%s does not support the OR fill hook", name)
		}
		if err := fi.IndexChunks(context.Background(), chunks); err != nil {
			t.Fatal(err)
		}
		n := &atomic.Int64{}
		fi.SetORFallbackHook(func() { n.Add(1) })
		out[name] = countedIndex{fi, n}
	}
	return out
}

func ids(hits []SearchResult) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Chunk.ChunkID
	}
	return out
}

func assertNoDuplicates(t *testing.T, hits []SearchResult) {
	t.Helper()
	seen := map[string]bool{}
	for _, h := range hits {
		if seen[h.Chunk.ChunkID] {
			t.Errorf("chunk %s repeated in %v", h.Chunk.ChunkID, ids(hits))
		}
		seen[h.Chunk.ChunkID] = true
	}
}

// SPEC-CORE-72.R9: when no term matches a page entirely, the OR fill still
// brings page 1 of the categories PDF, after the first-pass results.
func TestSPEC_CORE_72_R9_ORFillRetrievesCategoriesTable(t *testing.T) {
	const topK = 10 // router.DefaultRAGTopK
	cases := []struct {
		name      string
		terms     []string
		wantFirst string // first-pass hit expected at position 0, "" if none
	}{
		{
			name:  "no term matches entirely",
			terms: []string{categoriasQuestion, "categorías vigentes tabla", "tabla de montos febrero julio"},
		},
		{
			name:      "fill goes after the first-pass hit",
			terms:     []string{categoriasQuestion, "categorías vigentes tabla", "recategorización semestral"},
			wantFirst: "a_mono_00",
		},
	}
	indexes := orFillIndexes(t, arcaCorpus(categoriasPDF))
	ctx := tenant.WithTenantID(context.Background(), "tenant_afip")
	for name, idx := range indexes {
		for _, tc := range cases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				before := idx.fills.Load()
				hits, err := idx.Search(ctx, tc.terms, topK)
				if err != nil {
					t.Fatal(err)
				}
				if len(hits) == 0 || len(hits) > topK {
					t.Fatalf("got %d hits, want 1..%d", len(hits), topK)
				}
				if !hasPage1(hits) {
					t.Errorf("page 1 (table) missing from %v", ids(hits))
				}
				if tc.wantFirst != "" && hits[0].Chunk.ChunkID != tc.wantFirst {
					t.Errorf("first hit %s, want first-pass hit %s (%v)", hits[0].Chunk.ChunkID, tc.wantFirst, ids(hits))
				}
				assertNoDuplicates(t, hits)
				if got := idx.fills.Load() - before; got != 1 {
					t.Errorf("OR fill counter +%d, want +1", got)
				}
			})
		}
	}
}

// SPEC-CORE-72.R9: when the first search already returns topK chunks, the
// OR fill does not run.
func TestSPEC_CORE_72_R9_NoFillWhenFirstPassIsFull(t *testing.T) {
	cases := []struct {
		name  string
		terms []string
		topK  int
	}{
		{"single word, many pages", []string{"Monotributo"}, 3},
		{"two terms", []string{"categorías", "ingresos brutos"}, 2},
	}
	indexes := orFillIndexes(t, arcaCorpus(categoriasPDF))
	ctx := tenant.WithTenantID(context.Background(), "tenant_afip")
	for name, idx := range indexes {
		for _, tc := range cases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				before := idx.fills.Load()
				hits, err := idx.Search(ctx, tc.terms, tc.topK)
				if err != nil {
					t.Fatal(err)
				}
				if len(hits) != tc.topK {
					t.Fatalf("got %d hits, want %d", len(hits), tc.topK)
				}
				if got := idx.fills.Load() - before; got != 0 {
					t.Errorf("OR fill ran (+%d) with a full first pass", got)
				}
			})
		}
	}
}

// SPEC-CORE-72.R9: the OR fill keeps tenant_id IN (current, GLOBAL).
func TestSPEC_CORE_72_R9_ORFillKeepsTenantIsolation(t *testing.T) {
	chunks := append(arcaCorpus(categoriasPDF), DocumentChunk{
		// No term matches it entirely: only the OR fill can reach it.
		ChunkID: "o_other", TenantID: "tenant_otro", FileName: "otro.pdf", PageNumber: 1,
		Content: "Tabla de montos vigentes del otro tenant.",
	})
	terms := []string{"categorías vigentes tabla", "tabla de montos febrero julio"}
	indexes := orFillIndexes(t, chunks)
	for name, idx := range indexes {
		t.Run(name, func(t *testing.T) {
			cases := []struct {
				tenantID  string
				wantOther bool
			}{
				{"tenant_afip", false},
				{"tenant_otro", true}, // proves the fill would find it
			}
			for _, tc := range cases {
				before := idx.fills.Load()
				hits, err := idx.Search(tenant.WithTenantID(context.Background(), tc.tenantID), terms, 20)
				if err != nil {
					t.Fatal(err)
				}
				if idx.fills.Load() == before {
					t.Errorf("%s: the OR fill did not run, the test proves nothing", tc.tenantID)
				}
				found := false
				for _, h := range hits {
					found = found || h.Chunk.ChunkID == "o_other"
					if h.Chunk.TenantID != tc.tenantID && h.Chunk.TenantID != GlobalTenant {
						t.Errorf("%s got chunk %s of tenant %s", tc.tenantID, h.Chunk.ChunkID, h.Chunk.TenantID)
					}
				}
				if found != tc.wantOther {
					t.Errorf("%s: tenant_otro chunk found=%v, want %v (%v)", tc.tenantID, found, tc.wantOther, ids(hits))
				}
			}
		})
	}
}

// SPEC-CORE-72.R9: the counter goes up only when the fill adds chunks.
func TestSPEC_CORE_72_R9_CounterOnlyWhenFillContributes(t *testing.T) {
	cases := []struct {
		name  string
		terms []string
		topK  int
		want  int64
	}{
		{"fill contributes", []string{"categorías vigentes tabla"}, 5, 1},
		{"first pass full", []string{"Monotributo"}, 3, 0},
		{"first pass short, fill finds nothing", []string{"recategorizacion"}, 5, 0},
		{"no word matches", []string{"cómo cobro laburo afuera"}, 5, 0},
		{"no searchable words", []string{"  ", "!:*"}, 5, 0},
	}
	indexes := orFillIndexes(t, arcaCorpus(categoriasPDF))
	ctx := tenant.WithTenantID(context.Background(), "tenant_afip")
	for name, idx := range indexes {
		for _, tc := range cases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				before := idx.fills.Load()
				if _, err := idx.Search(ctx, tc.terms, tc.topK); err != nil {
					t.Fatal(err)
				}
				if got := idx.fills.Load() - before; got != tc.want {
					t.Errorf("counter +%d, want +%d", got, tc.want)
				}
			})
		}
	}
}

// SPEC-CORE-72.R9: the OR query has every word once, joined with |, and the
// same anti-injection rules as BuildTSQuery.
func TestSPEC_CORE_72_R9_BuildTSQueryAny(t *testing.T) {
	cases := []struct {
		name  string
		terms []string
		fold  bool
		want  string
	}{
		{"folded, repeats removed", []string{"Categorías vigentes", "tabla de categorías", "RG 5616"}, true, "categorias | vigentes | tabla | rg | 5616"},
		{"accented stop words dropped", []string{"¿Cuáles son las categorías?", "Cómo cobro"}, false, "categorías | cobro"},
		{"accented", []string{"Categorías vigentes"}, false, "categorías | vigentes"},
		{"tsquery syntax stripped", []string{"x' | !w:*", "b&c", "(d)"}, true, "x | w | b | c | d"},
		{"no words", []string{"  ", "!:*"}, true, ""},
		{"only stop words", []string{"de la", "y o"}, true, ""},
		{"nil", nil, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildTSQueryAny(tc.terms, tc.fold); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
