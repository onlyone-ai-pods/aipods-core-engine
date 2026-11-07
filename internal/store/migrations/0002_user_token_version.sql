-- SPEC-CORE-67.R15: changing or resetting a password bumps token_version,
-- which invalidates every token issued before.
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
