-- Seed the approved ranking entries once without replacing administrator edits.
ALTER TABLE quality_external_tools ADD COLUMN IF NOT EXISTS ranking_entries_initialized BOOLEAN NOT NULL DEFAULT FALSE;

WITH defaults AS (
 SELECT value AS item, ordinal FROM jsonb_array_elements('[
  {"id":"folkbench","name":"Folkbench","url":"https://folkbench.com/rankings/board","kind":"ranking","families":["gpt","claude"],"description":""},
  {"id":"ztest-rank","name":"真测 Ztest · 中转站排名","url":"https://ztest.ai/","kind":"ranking","families":["gpt","claude"],"description":""},
  {"id":"zhongzhuancha","name":"中转查","url":"https://zhongzhuancha.com/","kind":"ranking","families":["gpt","claude"],"description":""}
 ]'::jsonb) WITH ORDINALITY AS d(value, ordinal) ORDER BY ordinal
), additions AS (
 SELECT COALESCE(jsonb_agg(item ORDER BY ordinal), '[]'::jsonb) AS items FROM defaults
 WHERE NOT EXISTS (SELECT 1 FROM quality_external_tools t, jsonb_array_elements(t.items) existing
   WHERE t.id=TRUE AND existing->>'id'=item->>'id')
)
UPDATE quality_external_tools t SET
 items=t.items || additions.items,
 version=t.version + CASE WHEN jsonb_array_length(additions.items)>0 THEN 1 ELSE 0 END,
 updated_at=CASE WHEN jsonb_array_length(additions.items)>0 THEN NOW() ELSE t.updated_at END,
 ranking_entries_initialized=TRUE
FROM additions WHERE t.id=TRUE AND NOT t.ranking_entries_initialized;
