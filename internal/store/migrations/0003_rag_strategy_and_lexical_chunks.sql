-- SPEC-CORE-70.R3: RAG strategy per tenant.
ALTER TABLE tenants ADD COLUMN rag_strategy TEXT NOT NULL DEFAULT 'query_expansion'
    CHECK (rag_strategy IN ('query_expansion', 'external_embeddings'));

-- SPEC-CORE-70.R1/R4: lexical index for RAG with Spanish full-text search.
-- tsv indexes the content twice: as written (the Spanish stemmer needs the
-- accents: "facturación" -> "factur") and without accents (search_text, done
-- in Go), so a query typed without accents still matches.
-- No FK to tenants: with AUTH_MODE=off documents can belong to demo tenants
-- that have no row.
CREATE TABLE rag_chunks (
    chunk_id    TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    file_name   TEXT NOT NULL,
    page_number INTEGER NOT NULL DEFAULT 0,
    content     TEXT NOT NULL,
    search_text TEXT NOT NULL,
    tsv         TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('spanish', content) || to_tsvector('spanish', search_text)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rag_chunks_tsv_idx ON rag_chunks USING GIN (tsv);
CREATE INDEX rag_chunks_tenant_idx ON rag_chunks (tenant_id);
