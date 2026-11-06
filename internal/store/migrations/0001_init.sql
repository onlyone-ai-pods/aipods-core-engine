-- SPEC-CORE-67: tenants, users, approvals, vault and audit.
CREATE TABLE tenants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    roles         TEXT[] NOT NULL,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE approvals (
    token            TEXT PRIMARY KEY,
    tenant_id        TEXT NOT NULL,
    pod_id           TEXT NOT NULL,
    action_name      TEXT NOT NULL,
    summary          TEXT NOT NULL,
    command          TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    execution_result TEXT NOT NULL DEFAULT '',
    decided_by       TEXT NOT NULL DEFAULT '',
    requested_at     TIMESTAMPTZ NOT NULL,
    decided_at       TIMESTAMPTZ
);
CREATE INDEX approvals_tenant_idx ON approvals (tenant_id, requested_at DESC);

CREATE TABLE vault_secrets (
    tenant_id    TEXT NOT NULL,
    key_name     TEXT NOT NULL,
    cipher_text  TEXT NOT NULL,
    masked_value TEXT NOT NULL,
    algorithm    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key_name)
);

CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  TEXT NOT NULL,
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT NOT NULL DEFAULT '',
    outcome    TEXT NOT NULL,
    client_ip  TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_tenant_idx ON audit_log (tenant_id, created_at DESC);

-- audit_log is append-only.
CREATE FUNCTION audit_log_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

INSERT INTO tenants (id, name) VALUES ('GLOBAL', 'Plataforma AI Pods');
