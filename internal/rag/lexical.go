package rag

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

// orFillStopWords are left out of the OR fill (SPEC-CORE-72.R9), compared
// without accents. The Postgres spanish dictionary already drops "como" or
// "cuales", but not "cómo" or "cuáles", which open most questions and would
// fill the places with any page that has them.
var orFillStopWords = map[string]bool{
	"a": true, "al": true, "como": true, "con": true, "cual": true, "cuales": true, "cuando": true, "de": true,
	"del": true, "donde": true, "e": true, "el": true, "en": true, "es": true, "esta": true, "este": true,
	"hay": true, "la": true, "las": true, "le": true, "lo": true, "los": true, "mas": true, "me": true,
	"mi": true, "o": true, "para": true, "por": true, "que": true, "quien": true, "se": true, "si": true,
	"sin": true, "son": true, "su": true, "sus": true, "u": true, "un": true, "una": true, "y": true,
}

func isORFillStopWord(w string) bool { return orFillStopWords[Normalize(w)] }

// BuildTSQueryAny is the query of the OR fill (SPEC-CORE-72.R9): every word
// of every term, joined with OR (|) and without repeats or stop words. It
// follows the same rules as BuildTSQuery: only letter and digit tokens reach
// the query, so no tsquery syntax can be injected.
func BuildTSQueryAny(terms []string, fold bool) string {
	seen := make(map[string]bool)
	var parts []string
	for _, term := range terms {
		for _, w := range tokens(term, fold) {
			if !seen[w] && !isORFillStopWord(w) {
				seen[w] = true
				parts = append(parts, w)
			}
		}
	}
	return strings.Join(parts, " | ")
}

// orFallbackHook is called each time the OR fill adds at least one chunk to
// a search (SPEC-CORE-72.R9). It is atomic so it can be set while searching.
type orFallbackHook struct{ fn atomic.Pointer[func()] }

// SetORFallbackHook sets the function called when the OR fill contributes
// chunks (cmd/server wires the aipods_rag_lexical_or_fallback_total counter).
func (h *orFallbackHook) SetORFallbackHook(fn func()) {
	if fn == nil {
		h.fn.Store(nil)
		return
	}
	h.fn.Store(&fn)
}

func (h *orFallbackHook) fire() {
	if fn := h.fn.Load(); fn != nil {
		(*fn)()
	}
}

// PostgresLexical is the production lexical index: Spanish full-text search
// over the rag_chunks table (migration 0003).
type PostgresLexical struct {
	pool *pgxpool.Pool
	orFallbackHook
}

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
	if topK <= 0 {
		return nil, nil
	}
	accented, folded := BuildTSQuery(terms, false), BuildTSQuery(terms, true)
	if folded == "" {
		return nil, nil
	}
	out, err := p.query(ctx, tenantID, accented, folded, nil, topK)
	if err != nil {
		return nil, fmt.Errorf("lexical search: %w", err)
	}
	if len(out) >= topK {
		return out, nil
	}
	// OR fill (SPEC-CORE-72.R9): any word of any term completes the
	// missing places, after the first results and without repeating them.
	anyAccented, anyFolded := BuildTSQueryAny(terms, false), BuildTSQueryAny(terms, true)
	if anyFolded == "" {
		return out, nil
	}
	seen := make([]string, 0, len(out))
	for _, r := range out {
		seen = append(seen, r.Chunk.ChunkID)
	}
	fill, err := p.query(ctx, tenantID, anyAccented, anyFolded, seen, topK-len(out))
	if err != nil {
		return nil, fmt.Errorf("lexical OR fill: %w", err)
	}
	if len(fill) > 0 {
		p.fire()
	}
	return append(out, fill...), nil
}

// query runs one full-text search, best first, skipping the chunk IDs in exclude.
func (p *PostgresLexical) query(ctx context.Context, tenantID, accented, folded string, exclude []string, limit int) ([]SearchResult, error) {
	if exclude == nil {
		exclude = []string{}
	}
	// INVARIANT: tenant_id IN (current, GLOBAL), as in the vector stores.
	rows, err := p.pool.Query(ctx, `
		SELECT chunk_id, tenant_id, file_name, page_number, content, ts_rank_cd(tsv, q) AS score
		FROM rag_chunks, (SELECT to_tsquery('spanish', $1) || to_tsquery('spanish', $2) AS q) AS query
		WHERE tsv @@ q AND tenant_id IN ($3, 'GLOBAL') AND NOT (chunk_id = ANY($5::text[]))
		ORDER BY score DESC, chunk_id
		LIMIT $4`, accented, folded, tenantID, limit, exclude)
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
	orFallbackHook
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
	if topK <= 0 {
		return nil, nil
	}
	var queries [][]string
	anyWords := make(map[string]bool)
	for _, t := range terms {
		var ws []string
		for _, w := range words(t) {
			ws = append(ws, stem(w))
			if !isORFillStopWord(w) {
				anyWords[stem(w)] = true
			}
		}
		if len(ws) > 0 {
			queries = append(queries, ws)
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out, fill []SearchResult
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
			continue
		}
		// OR fill candidate (SPEC-CORE-72.R9): scored by how many of the
		// words it has, a rough stand-in for ts_rank_cd.
		anyScore := 0.0
		for w := range anyWords {
			if have[w] {
				anyScore++
			}
		}
		if anyScore > 0 {
			fill = append(fill, SearchResult{Chunk: c, Score: anyScore, CitationID: citationFor(c)})
		}
	}
	sortResults(out)
	if len(out) >= topK {
		return out[:topK], nil
	}
	sortResults(fill)
	if missing := topK - len(out); len(fill) > missing {
		fill = fill[:missing]
	}
	if len(fill) > 0 {
		m.fire()
	}
	return append(out, fill...), nil
}

// sortResults orders by score, best first, and by chunk ID on ties.
func sortResults(rs []SearchResult) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Score != rs[j].Score {
			return rs[i].Score > rs[j].Score
		}
		return rs[i].Chunk.ChunkID < rs[j].Chunk.ChunkID
	})
}
