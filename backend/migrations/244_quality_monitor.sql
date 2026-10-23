-- Independent, opt-in administrator monitoring. No gateway or billing rows are changed.
CREATE TABLE IF NOT EXISTS quality_monitor_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    payload JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS quality_monitor_targets (
    id UUID PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    overrides JSONB,
    version INTEGER NOT NULL DEFAULT 1,
    next_availability_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_quality_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_cache_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retry_attempt INTEGER NOT NULL DEFAULT 0,
    last_availability_at TIMESTAMPTZ,
    last_quality_at TIMESTAMPTZ,
    last_cache_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, model)
);

CREATE TABLE IF NOT EXISTS quality_monitor_runs (
    id UUID PRIMARY KEY,
    target_id UUID NOT NULL REFERENCES quality_monitor_targets(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('availability','quality','cache')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('queued','running','finished')),
    manual BOOLEAN NOT NULL DEFAULT FALSE,
    verdict VARCHAR(32) NOT NULL DEFAULT 'pending',
    profile_hash VARCHAR(64) NOT NULL DEFAULT '',
    config_snapshot JSONB,
    result JSONB,
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS quality_monitor_one_pending_idx
    ON quality_monitor_runs(target_id) WHERE status IN ('queued','running');
CREATE INDEX IF NOT EXISTS quality_monitor_history_idx
    ON quality_monitor_runs(target_id, kind, created_at DESC);
CREATE INDEX IF NOT EXISTS quality_monitor_queue_idx
    ON quality_monitor_runs(status, created_at) WHERE status IN ('queued','running');

CREATE TABLE IF NOT EXISTS quality_monitor_daily_budget (
    day DATE PRIMARY KEY,
    requests INTEGER NOT NULL DEFAULT 0 CHECK (requests >= 0),
    tokens BIGINT NOT NULL DEFAULT 0 CHECK (tokens >= 0),
    unconfirmed_requests INTEGER NOT NULL DEFAULT 0 CHECK (unconfirmed_requests >= 0)
);
