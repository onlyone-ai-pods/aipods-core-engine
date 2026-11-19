-- SPEC-CORE-73.R9, R10: sagas and dynamic pods shared by both colors.
-- Expand only (R14): new tables, nothing removed or changed.
-- 0004 is reserved by SPEC-CORE-72.

CREATE TABLE IF NOT EXISTS sagas (
    saga_id        TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL,
    action_name    TEXT NOT NULL,
    is_critical    BOOLEAN NOT NULL DEFAULT FALSE,
    status         TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    executed_at    TIMESTAMPTZ,
    compensated_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS sagas_tenant_idx ON sagas (tenant_id, created_at);

CREATE TABLE IF NOT EXISTS dynamic_pods (
    pod_id       TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    endpoint_url TEXT NOT NULL,
    keywords     TEXT[] NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
