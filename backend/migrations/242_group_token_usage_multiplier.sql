ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS token_usage_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0;

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_token_usage_multiplier_range;
ALTER TABLE groups
    ADD CONSTRAINT groups_token_usage_multiplier_range
    CHECK (token_usage_multiplier >= 1.0 AND token_usage_multiplier <= 100.0);

-- Freeze the multiplier with the batch pricing snapshot: later group edits must
-- not change a pending job's hold, settlement or refund. Existing jobs stay at 1.
ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS token_usage_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0
    CHECK (token_usage_multiplier >= 1.0 AND token_usage_multiplier <= 100.0);

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS token_usage_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS actual_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_output_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_cache_creation_5m_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_cache_creation_1h_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_image_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_image_output_tokens INTEGER NOT NULL DEFAULT 0;

ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_token_usage_multiplier_range;
ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_token_usage_multiplier_range
    CHECK (token_usage_multiplier >= 1.0 AND token_usage_multiplier <= 100.0) NOT VALID;
-- NOT VALID avoids scanning the historical usage table during deployment.
-- Existing rows receive the valid constant default; new writes are constrained.
