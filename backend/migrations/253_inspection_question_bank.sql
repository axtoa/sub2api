-- This library has its own version so editing questions does not reschedule checks.
CREATE TABLE IF NOT EXISTS quality_inspection_question_bank (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Import the previous reusable questions once, preserving order and content.
INSERT INTO quality_inspection_question_bank(id, items)
SELECT TRUE, COALESCE((
    SELECT jsonb_agg(question || jsonb_build_object('id', gen_random_uuid()::text) ORDER BY ordinal)
    FROM quality_monitor_settings,
         jsonb_array_elements(CASE WHEN jsonb_typeof(payload->'inspection_questions') = 'array'
             THEN payload->'inspection_questions' ELSE '[]'::jsonb END) WITH ORDINALITY AS q(question, ordinal)
    WHERE quality_monitor_settings.id = TRUE
), '[]'::jsonb)
ON CONFLICT DO NOTHING;
