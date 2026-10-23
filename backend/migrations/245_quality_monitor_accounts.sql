-- Quality monitoring owns its inventory and queues; business accounts remain unchanged.
ALTER TABLE quality_monitor_settings ADD COLUMN IF NOT EXISTS inventory_initialized BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE IF NOT EXISTS quality_monitor_accounts (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Repeated manual clicks are independent jobs. Only automatic jobs are deduplicated.
DROP INDEX IF EXISTS quality_monitor_one_pending_idx;
CREATE UNIQUE INDEX IF NOT EXISTS quality_monitor_one_auto_pending_idx
    ON quality_monitor_runs(target_id) WHERE status IN ('queued','running') AND NOT manual;
CREATE UNIQUE INDEX IF NOT EXISTS quality_monitor_one_running_idx
    ON quality_monitor_runs(target_id) WHERE status='running';
ALTER TABLE quality_monitor_daily_budget ADD COLUMN IF NOT EXISTS manual_requests INTEGER NOT NULL DEFAULT 0;
ALTER TABLE quality_monitor_daily_budget ADD COLUMN IF NOT EXISTS manual_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE quality_monitor_daily_budget ADD COLUMN IF NOT EXISTS manual_unconfirmed INTEGER NOT NULL DEFAULT 0;
