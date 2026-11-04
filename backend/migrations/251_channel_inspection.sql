-- Manual channel inspections reuse the durable quality queue and account slots.
ALTER TABLE quality_monitor_runs ADD COLUMN IF NOT EXISTS inspection_options JSONB;
ALTER TABLE quality_monitor_runs DROP CONSTRAINT IF EXISTS quality_monitor_runs_kind_check;
ALTER TABLE quality_monitor_runs ADD CONSTRAINT quality_monitor_runs_kind_check
 CHECK (kind IN ('availability','quality','cache','inspection'));
