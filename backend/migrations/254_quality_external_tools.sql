CREATE TABLE IF NOT EXISTS quality_external_tools (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Empty built-in descriptions use the application's translated introduction.
INSERT INTO quality_external_tools(id,items) VALUES(TRUE, '[
 {"id":"cctest","name":"CCTest","url":"https://cctest.ai/zh","kind":"channel","families":["gpt","claude"],"description":""},
 {"id":"oktest","name":"okTest","url":"https://oktest.ai/zh","kind":"channel","families":["gpt","claude"],"description":""},
 {"id":"ztest","name":"真测 Ztest","url":"https://ztest.ai/","kind":"channel","families":["gpt","claude"],"description":""},
 {"id":"llmtest","name":"LLM Test","url":"https://llmtest.cn/","kind":"channel","families":["gpt","claude"],"description":""},
 {"id":"artificialanalysis","name":"Artificial Analysis","url":"https://artificialanalysis.ai/","kind":"benchmark","families":["gpt","claude"],"description":""}
]'::jsonb) ON CONFLICT DO NOTHING;
