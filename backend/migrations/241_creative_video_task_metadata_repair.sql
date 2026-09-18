-- Repair migration for deployments where the metadata migration was recorded
-- before all columns were present in the database.
ALTER TABLE creative_video_tasks
    ADD COLUMN IF NOT EXISTS actual_cost NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS content_type VARCHAR(128),
    ADD COLUMN IF NOT EXISTS download_url TEXT,
    ADD COLUMN IF NOT EXISTS file_id VARCHAR(256);
