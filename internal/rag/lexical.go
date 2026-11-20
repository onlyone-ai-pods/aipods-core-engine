package rag

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// Lexical RAG search (SPEC-CORE-70.R1/R4): finds chunks by words instead of
// vectors, so it works without any embedding model.

// LexicalIndex stores chunks for full-text search. Search returns chunks that
// match any of the terms, best first, limited to the context's tenant and GLOBAL.
type LexicalIndex interface {
	IndexChunks(ctx context.Context, chunks []DocumentChunk) error
	Search(ctx context.Context, terms []string, topK int) ([]SearchResult, error)
	Name() string
}

// Normalize lowercases and removes accents, so "Facturación" and
// "facturacion" are the same word for the index.
func Normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}

// fileNameWords returns the words of a file name without its extension,
// split on "_", "-", "." and spaces and normalized (SPEC-CORE-72.R7):
// "ARCA_Monotributo_Categorias_Feb-Jul2026.pdf" -> arca monotributo
// categorias feb jul2026. Migration 0004 applies the same rule in SQL.
func fileNameWords(fileName string) []string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || unicode.IsSpace(r)
	})
	for i, p := range parts {
		parts[i] = Normalize(p)
	}
	return parts
}

// searchText is the unaccented text indexed for a chunk: its content plus
// the words of its file name, so every page of a document matches the words
// of its title even when the page does not repeat them (SPEC-CORE-72.R7).
func searchText(fileName, content string) string {
	text := Normalize(content)
	if w := fileNameWords(fileName); len(w) > 0 {
		text += " " + strings.Join(w, " ")
	}
	return text
}

// tokens splits lowercased text into letter/digit tokens; fold also removes accents.
func tokens(s string, fold bool) []string {
	if fold {
		s = Normalize(s)
	} else {
		s = strings.ToLower(s)
	}
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func words(s string) []string { return tokens(s, true) }

// BuildTSQuery turns search terms into a to_tsquery expression: the words of a
// term must all appear (&), and any term may match (|). Only letter and digit
// tokens reach the query, so user or model text cannot inject tsquery syntax.
// fold removes accents (for the unaccented half of the index).
func BuildTSQuery(terms []string, fold bool) string {
	seen := make(map[string]bool)
	var parts []string
	for _, term := range terms {
		ws := tokens(term, fold)
		if len(ws) == 0 {
			continue
		}
		p := strings.Join(ws, " & ")
		if len(ws) > 1 {
			p = "(" + p + ")"
		}
		if !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " | ")
}

// PostgresLexical is the production lexical index: Spanish full-text search
// over the rag_chunks table (migration 0003).
type PostgresLexical struct{ pool *pgxpool.Pool }

func NewPostgresLexical(pool *pgxpool.Pool) *PostgresLexical { return &PostgresLexical{pool: pool} }

func (p *PostgresLexical) Name() string { return "postgres-fts:spanish" }

func (p *PostgresLexical) IndexChunks(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range chunks {
		batch.Queue(`INSERT INTO rag_chunks (chunk_id, tenant_id, file_name, page_number, content, search_text)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (chunk_id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id, file_name = EXCLUDED.file_name,
				page_number = EXCLUDED.page_number, content = EXCLUDED.content, search_text = EXCLUDED.search_text`,
			c.ChunkID, c.TenantID, c.FileName, c.PageNumber, c.Content, searchText(c.FileName, c.Content))
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (p *PostgresLexical) Search(ctx context.Context, terms []string, topK int) ([]SearchResult, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, ErrNoTenant
	}
	accented, folded := BuildTSQuery(terms, false), BuildTSQuery(terms, true)
	if folded == "" {
		return nil, nil
	}
	// INVARIANT: tenant_id IN (current, GLOBAL), as in the vector stores.
	rows, err := p.pool.Query(ctx, `
		SELECT chunk_id, tenant_id, file_name, page_number, content, ts_rank_cd(tsv, q) AS score
		FROM rag_chunks, (SELECT to_tsquery('spanish', $1) || to_tsquery('spanish', $2) AS q) AS query
		WHERE tsv @@ q AND tenant_id IN ($3, 'GLOBAL')
		ORDER BY score DESC, chunk_id
		LIMIT $4`, accented, folded, tenantID, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var c DocumentChunk
		var score float64
		if err := rows.Scan(&c.ChunkID, &c.TenantID, &c.FileName, &c.PageNumber, &c.Content, &score); err != nil {
			return nil, err
		}
		if c.TenantID != tenantID && c.TenantID != "GLOBAL" {
			continue // defense in depth
		}
		out = append(out, SearchResult{Chunk: c, Score: score, CitationID: citationFor(c)})
	}
	return out, rows.Err()
}

// MemoryLexical is the lexical index for development and tests (no
// DATABASE_URL). It matches word prefixes, a rough stand-in for stemming.
type MemoryLexical struct {
	mu     sync.RWMutex
	chunks map[string]DocumentChunk
}

func NewMemoryLexical() *MemoryLexical { return &MemoryLexical{chunks: map[string]DocumentChunk{}} }

func (m *MemoryLexical) Name() string { return "memory-lexical" }

func (m *MemoryLexical) IndexChunks(_ context.Context, chunks []DocumentChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range chunks {
		c.Embedding = nil
		m.chunks[c.ChunkID] = c
	}
	return nil
}

// memoryStemLen is how many leading letters must match ("facturar" ~ "factura").
const memoryStemLen = 6

func stem(w string) string {
	r := []rune(w)
	if len(r) > memoryStemLen {
		return string(r[:memoryStemLen])
	}
	return w
}

func (m *MemoryLexical) Search(ctx context.Context, terms []string, topK int) ([]SearchResult, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, ErrNoTenant
	}
	var queries [][]string
	for _, t := range terms {
		var ws []string
		for _, w := range words(t) {
			ws = append(ws, stem(w))
		}
		if len(ws) > 0 {
			queries = append(queries, ws)
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []SearchResult
	for _, c := range m.chunks {
		if c.TenantID != tenantID && c.TenantID != "GLOBAL" {
			continue
		}
		have := make(map[string]bool)
		for _, w := range words(searchText(c.FileName, c.Content)) {
			have[stem(w)] = true
		}
		score := 0.0
		for _, q := range queries {
			all := true
			for _, w := range q {
				if !have[w] {
					all = false
					break
				}
			}
			if all {
				score++
			}
		}
		if score > 0 {
			out = append(out, SearchResult{Chunk: c, Score: score, CitationID: citationFor(c)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Chunk.ChunkID < out[j].Chunk.ChunkID
	})
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}
