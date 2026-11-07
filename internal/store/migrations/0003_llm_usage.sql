-- SPEC-CORE-69: tokens of every Claude call, attributed to a tenant.
CREATE TABLE llm_usage (
    id                 BIGSERIAL PRIMARY KEY,
    tenant_id          TEXT NOT NULL,
    model              TEXT NOT NULL,
    purpose            TEXT NOT NULL DEFAULT '',
    input_tokens       BIGINT NOT NULL DEFAULT 0,
    output_tokens      BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens  BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens BIGINT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX llm_usage_tenant_created ON llm_usage (tenant_id, created_at);
CREATE INDEX llm_usage_created ON llm_usage (created_at);
