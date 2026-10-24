-- Nullable observation only. No default/backfill/index or billing changes.
-- Older binaries ignore this column, so code rollback does not require DDL.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS account_selection_reason VARCHAR(16);
