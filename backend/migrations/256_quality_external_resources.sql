-- Add complementary references once, preserving administrator content and deletions.
ALTER TABLE quality_external_tools ADD COLUMN IF NOT EXISTS reference_entries_initialized BOOLEAN NOT NULL DEFAULT FALSE;

WITH defaults AS (
 SELECT value AS item, ordinal FROM jsonb_array_elements('[
  {"id":"openai-status","name":"OpenAI · 官方服务状态","url":"https://status.openai.com/","kind":"status","families":["gpt"],"description":""},
  {"id":"claude-status","name":"Claude · 官方服务状态","url":"https://status.claude.com/","kind":"status","families":["claude"],"description":""},
  {"id":"openai-pricing","name":"OpenAI · 官方 API 价格","url":"https://developers.openai.com/api/docs/pricing","kind":"pricing","families":["gpt"],"description":""},
  {"id":"claude-pricing","name":"Claude · 官方 API 价格","url":"https://platform.claude.com/docs/en/about-claude/pricing","kind":"pricing","families":["claude"],"description":""},
  {"id":"models-dev","name":"Models.dev · 模型规格库","url":"https://models.dev/","kind":"pricing","families":["gpt","claude"],"description":""},
  {"id":"livebench","name":"LiveBench","url":"https://livebench.ai/","kind":"benchmark","families":["gpt","claude"],"description":""}
 ]'::jsonb) WITH ORDINALITY AS d(value, ordinal)
), eligible AS (
 SELECT item, ordinal FROM defaults
 WHERE NOT EXISTS (SELECT 1 FROM quality_external_tools t, jsonb_array_elements(t.items) existing
   WHERE t.id=TRUE AND (existing->>'id'=item->>'id'
     OR (existing->>'kind'=item->>'kind' AND RTRIM(existing->>'url','/')=RTRIM(item->>'url','/'))))
 ORDER BY ordinal
 LIMIT (SELECT GREATEST(0,100-jsonb_array_length(items)) FROM quality_external_tools WHERE id=TRUE)
), additions AS (
 SELECT COALESCE(jsonb_agg(item ORDER BY ordinal), '[]'::jsonb) AS items FROM eligible
)
UPDATE quality_external_tools t SET
 items=t.items || additions.items,
 version=t.version + CASE WHEN jsonb_array_length(additions.items)>0 THEN 1 ELSE 0 END,
 updated_at=CASE WHEN jsonb_array_length(additions.items)>0 THEN NOW() ELSE t.updated_at END,
 reference_entries_initialized=TRUE
FROM additions WHERE t.id=TRUE AND NOT t.reference_entries_initialized;
