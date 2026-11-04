-- Independent from scheduling: editing assessments never starts paid probes.
CREATE TABLE IF NOT EXISTS quality_assurance_settings (
 id SMALLINT PRIMARY KEY CHECK (id=1),
 config JSONB NOT NULL DEFAULT '{}'::jsonb,
 version INTEGER NOT NULL DEFAULT 1,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO quality_assurance_settings(id) VALUES(1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS quality_assurance_evidence (
 id UUID PRIMARY KEY,
 run_id UUID NOT NULL REFERENCES quality_monitor_runs(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('billing','identity','privacy')),
 status TEXT NOT NULL CHECK(status IN ('suspected','confirmed','cleared')),
 detail JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS quality_assurance_evidence_run ON quality_assurance_evidence(run_id,created_at DESC);
CREATE INDEX IF NOT EXISTS quality_runs_assurance_window ON quality_monitor_runs(target_id,finished_at DESC) WHERE status='finished';
