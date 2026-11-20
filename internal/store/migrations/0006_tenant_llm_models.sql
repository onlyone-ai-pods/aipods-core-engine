-- SPEC-CORE-74.R5: LLM model per task and fallback chain per tenant.
-- Expand only (SPEC-CORE-73.R14): nullable columns, NULL = inherit the
-- platform configuration. No CHECK on the model names: the allowed list
-- changes over time and is validated in Go.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS llm_model_classify TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS llm_model_expand TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS llm_model_answer TEXT;
-- Comma-separated, in order. NULL = inherit; '' = no fallback chain.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS llm_fallback_models TEXT;
