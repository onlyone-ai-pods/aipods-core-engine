package rag

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// recordingLexical keeps what IndexChunks receives.
type recordingLexical struct {
	*MemoryLexical
	got []DocumentChunk
}

func (r *recordingLexical) IndexChunks(ctx context.Context, chunks []DocumentChunk) error {
	r.got = append(r.got, chunks...)
	return r.MemoryLexical.IndexChunks(ctx, chunks)
}

func assertCleanChunks(t *testing.T, chunks []DocumentChunk) {
	t.Helper()
	for _, c := range chunks {
		if strings.Contains(c.Content, "\x00") || !utf8.ValidString(c.Content) || strings.TrimSpace(c.Content) == "" {
			t.Errorf("chunk %s is not clean: %q", c.ChunkID, c.Content)
		}
	}
}

// SPEC-CORE-72.R5: testdata/nul_bytes.pdf is a one-page PDF (Helvetica with
// WinAnsiEncoding) whose text literal holds \000 escapes, so the extractor
// returns real 0x00 bytes, as with the ARCA ClaveFiscal manual.
func TestSPEC_CORE_72_R5_PDFWithNulBytesIsIngested(t *testing.T) {
	raw, err := os.ReadFile("testdata/nul_bytes.pdf")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := extractPDFPages(raw)
	if err != nil || len(pages) != 1 || !strings.Contains(pages[0], "\x00") {
		t.Fatalf("fixture must extract text with 0x00 (pages %q, err %v)", pages, err)
	}

	lex := &recordingLexical{MemoryLexical: NewMemoryLexical()}
	svc := NewRAGIngestionService(nil, nil)
	svc.SetLexicalIndex(lex)
	chunks, err := svc.IngestDocument(context.Background(), GlobalTenant, "ARCA_Manual_ServiciosClaveFiscal_Generalidades.pdf", raw)
	if err != nil {
		t.Fatalf("ingestion must succeed: %v", err)
	}
	if len(chunks) == 0 || len(lex.got) != len(chunks) {
		t.Fatalf("chunks %d, indexed %d", len(chunks), len(lex.got))
	}
	assertCleanChunks(t, chunks)
	assertCleanChunks(t, lex.got)
	if !strings.Contains(chunks[0].Content, "ClaveFiscal manual de ARCA") {
		t.Errorf("text must survive sanitization: %q", chunks[0].Content)
	}
}

// SPEC-CORE-72.R5: every format goes through the same cleaning; a document
// with nothing left is a clear error, not a Postgres one.
func TestSPEC_CORE_72_R5_SanitizeChunks(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"nul bytes removed", []string{"Clave\x00Fiscal\x00"}, []string{"ClaveFiscal"}},
		{"invalid utf-8 replaced", []string{"Categor\xeda A"}, []string{"Categor�a A"}},
		{"empty chunks dropped", []string{"\x00\x00", "  \x00 ", "texto"}, []string{"texto"}},
		{"valid text untouched", []string{"Facturación electrónica"}, []string{"Facturación electrónica"}},
		{"nothing left", []string{"\x00"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in []DocumentChunk
			for _, c := range tc.in {
				in = append(in, DocumentChunk{Content: c, FileName: "doc\x00.pdf"})
			}
			out := sanitizeChunks(in)
			if len(out) != len(tc.want) {
				t.Fatalf("got %d chunks, want %d", len(out), len(tc.want))
			}
			for i, c := range out {
				if c.Content != tc.want[i] || c.FileName != "doc.pdf" {
					t.Errorf("chunk %d = %q (%q), want %q", i, c.Content, c.FileName, tc.want[i])
				}
			}
		})
	}

	svc := NewRAGIngestionService(nil, nil)
	svc.SetLexicalIndex(NewMemoryLexical())
	ctx := context.Background()
	chunks, err := svc.IngestDocument(ctx, "tenant_acme", "notas.txt", []byte("Clave\x00fiscal nivel 3 \xff habilitada"))
	if err != nil || len(chunks) == 0 {
		t.Fatalf("txt with NUL must ingest: %v", err)
	}
	assertCleanChunks(t, chunks)
	if _, err := svc.IngestDocument(ctx, "tenant_acme", "vacio.txt", []byte("\x00\x00\x00")); !errors.Is(err, ErrNoTextAfterSanitize) {
		t.Errorf("a document with only NUL bytes must fail clearly, got %v", err)
	}
}
