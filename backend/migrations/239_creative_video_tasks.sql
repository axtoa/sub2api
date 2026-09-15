CREATE TABLE IF NOT EXISTS creative_video_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    provider_request_id VARCHAR(128),
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT,
    account_id BIGINT,
    provider VARCHAR(32) NOT NULL DEFAULT 'grok',
    model VARCHAR(128) NOT NULL,
    prompt_preview TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    resolution VARCHAR(16),
    duration_seconds INTEGER,
    output_expires_at TIMESTAMPTZ,
    downloaded_at TIMESTAMPTZ,
    output_deleted_at TIMESTAMPTZ,
    user_deleted_at TIMESTAMPTZ,
    last_error_code VARCHAR(128),
    last_error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS creative_video_tasks_provider_request_uq
    ON creative_video_tasks (provider_request_id)
    WHERE provider_request_id IS NOT NULL AND provider_request_id <> '';

CREATE INDEX IF NOT EXISTS creative_video_tasks_owner_created_idx
    ON creative_video_tasks (user_id, api_key_id, created_at DESC, id DESC)
    WHERE user_deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS creative_video_tasks_active_user_idx
    ON creative_video_tasks (user_id, status)
    WHERE user_deleted_at IS NULL AND status IN ('queued', 'submitted', 'running');

CREATE INDEX IF NOT EXISTS creative_video_tasks_cleanup_idx
    ON creative_video_tasks (created_at, id)
    WHERE user_deleted_at IS NULL;
