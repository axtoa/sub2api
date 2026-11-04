-- Extend the resource library once; preserve administrator edits and deletions.
ALTER TABLE quality_external_tools ADD COLUMN IF NOT EXISTS business_entries_initialized BOOLEAN NOT NULL DEFAULT FALSE;

WITH defaults AS (
 SELECT value AS item, ordinal FROM jsonb_array_elements('[
  {"id":"yccc-ai","name":"AI YCCC","url":"https://ai.yccc.me/","kind":"creative","families":[],"description":"","tags":["图片","待核实"],"origin":"user","usage":"saved","review":"unconfirmed","reviewed_at":"2026-11-04","notes":"图片站"},
  {"id":"zpay","name":"ZPAY","url":"https://zpayz.cn/","kind":"payment","families":[],"description":"","tags":["国内收款","支付宝","微信","易支付"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"聚合支付"},
  {"id":"dulupay","name":"DuluPay","url":"https://dulupay.com","kind":"payment","families":[],"description":"","tags":["支付","待核实"],"origin":"user","usage":"saved","review":"unconfirmed","reviewed_at":"2026-11-04","notes":"聚合支付"},
  {"id":"zeptomail","name":"Zoho ZeptoMail","url":"https://zeptomail.zoho.com.cn","kind":"email","families":[],"description":"","tags":["验证码","事务邮件","SMTP"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"邮箱验证. AppId 三方登录","reference_url":"https://www.zoho.com.cn/zeptomail/"},
  {"id":"resend","name":"Resend","url":"https://resend.com/signup","kind":"email","families":[],"description":"","tags":["验证码","事务邮件","邮件 API"],"origin":"user","usage":"trial","review":"public","reviewed_at":"2026-11-04","notes":"邮箱验证，正在试用","reference_url":"https://resend.com/"},
  {"id":"cloudflare","name":"Cloudflare 控制台","url":"https://dash.cloudflare.com/","kind":"infrastructure","families":[],"description":"","tags":["DNS","CDN","防护","R2","备份"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"数据备份","reference_url":"https://www.cloudflare.com/products/r2/"},
  {"id":"ovhcloud","name":"OVHcloud US","url":"https://us.ovhcloud.com/","kind":"infrastructure","families":[],"description":"","tags":["服务器","VPS","独立服务器"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"服务器"},
  {"id":"aibijia","name":"AI 比价","url":"https://aibijia.org/","kind":"marketplace","families":["gpt","claude"],"description":"","tags":["订阅比价","数字商品","采购"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"AI 比价 选择免税州+菲律宾区"},
  {"id":"engy","name":"engy","url":"https://engy.ai/pricing","kind":"upstream","families":[],"description":"","tags":["开放模型","国模","推理 API","缓存"],"origin":"user","usage":"saved","review":"public","reviewed_at":"2026-11-04","notes":"国模出海"},
  {"id":"dmc-studio","name":"DMC Studio","url":"https://dmc.cc/studio","kind":"creative","families":[],"description":"","tags":["工作台","界面参考","待核实"],"origin":"user","usage":"saved","review":"unconfirmed","reviewed_at":"2026-11-04","notes":"AI工作台样式","reference_url":"https://dmc.cc/"},
  {"id":"zovocard","name":"ZovoCard 商城","url":"https://zovocard.com/mall","kind":"marketplace","families":[],"description":"","tags":["商城","采购","待核实"],"origin":"user","usage":"saved","review":"unconfirmed","reviewed_at":"2026-11-04"},
  {"id":"openrouter","name":"OpenRouter","url":"https://openrouter.ai/","kind":"upstream","families":["gpt","claude"],"description":"","tags":["多模型","路由","上游采购"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"siliconflow","name":"SiliconFlow","url":"https://www.siliconflow.com/","kind":"upstream","families":[],"description":"","tags":["国模","多模态","推理 API"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"modelstudio","name":"Alibaba Cloud Model Studio","url":"https://www.alibabacloud.com/en/product/modelstudio","kind":"upstream","families":[],"description":"","tags":["Qwen","厂商平台","国模"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"fal","name":"fal","url":"https://fal.ai/","kind":"upstream","families":[],"description":"","tags":["图片 API","视频 API","音频","多模态"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"replicate","name":"Replicate","url":"https://replicate.com/","kind":"upstream","families":[],"description":"","tags":["模型市场","图片 API","视频 API"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"wechat-pay","name":"微信支付商户平台","url":"https://pay.weixin.qq.com/","kind":"payment","families":[],"description":"","tags":["官方支付","微信","国内收款"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"alipay","name":"支付宝开放平台","url":"https://open.alipay.com/","kind":"payment","families":[],"description":"","tags":["官方支付","支付宝","国内收款"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"stripe","name":"Stripe Payments","url":"https://stripe.com/payments","kind":"payment","families":[],"description":"","tags":["国际收款","信用卡","Checkout"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"paddle","name":"Paddle","url":"https://www.paddle.com/","kind":"payment","families":[],"description":"","tags":["国际收款","订阅","MoR","销售税"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"auth0","name":"Auth0","url":"https://auth0.com/","kind":"email","families":[],"description":"","tags":["OAuth","第三方登录","身份认证","MFA"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"hetzner","name":"Hetzner Cloud","url":"https://www.hetzner.com/cloud/","kind":"infrastructure","families":[],"description":"","tags":["云主机","服务器","部署成本"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"uptime-kuma","name":"Uptime Kuma","url":"https://github.com/louislam/uptime-kuma","kind":"operations","families":[],"description":"","tags":["开源","可用性","告警","状态页"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"sentry","name":"Sentry","url":"https://sentry.io/welcome/","kind":"operations","families":[],"description":"","tags":["错误追踪","性能","调用链"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"restic","name":"restic","url":"https://restic.net/","kind":"operations","families":[],"description":"","tags":["开源","加密备份","恢复","容灾"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"langfuse","name":"Langfuse","url":"https://langfuse.com/","kind":"operations","families":[],"description":"","tags":["LLM 评估","调用跟踪","成本分析","开源"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"huggingface-spaces","name":"Hugging Face Spaces","url":"https://huggingface.co/spaces","kind":"creative","families":[],"description":"","tags":["产品灵感","AI 演示","图片","语音"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04"},
  {"id":"lobehub","name":"LobeHub","url":"https://lobehub.com/","kind":"creative","families":["gpt","claude"],"description":"","tags":["工作台","Agent","界面参考","多模型"],"origin":"curated","usage":"saved","review":"public","reviewed_at":"2026-11-04","reference_url":"https://github.com/lobehub/lobehub"}
 ]'::jsonb) WITH ORDINALITY AS d(value, ordinal)
), base AS (
 SELECT COALESCE(jsonb_agg(CASE
   WHEN RTRIM(item->>'url','/')='https://status.openai.com' AND NOT item ? 'notes'
   THEN item || '{"notes":"各模型的运行状态"}'::jsonb
   ELSE item END ORDER BY ordinal),'[]'::jsonb) AS items
 FROM quality_external_tools t, jsonb_array_elements(t.items) WITH ORDINALITY AS d(item,ordinal)
 WHERE t.id=TRUE
), eligible AS (
 SELECT item, ordinal FROM defaults
 WHERE NOT EXISTS (SELECT 1 FROM quality_external_tools t, jsonb_array_elements(t.items) existing
   WHERE t.id=TRUE AND (existing->>'id'=item->>'id'
     OR RTRIM(existing->>'url','/')=RTRIM(item->>'url','/')))
 ORDER BY ordinal
 LIMIT (SELECT GREATEST(0,100-jsonb_array_length(items)) FROM quality_external_tools WHERE id=TRUE)
), sized AS (
 SELECT item, ordinal, SUM(octet_length(item::text)+2) OVER (ORDER BY ordinal) AS added_bytes
 FROM eligible
), additions AS (
 -- Match Go's HTML escaping conservatively; jsonb text also includes separator spaces.
 SELECT COALESCE(jsonb_agg(item ORDER BY ordinal), '[]'::jsonb) AS items FROM sized
 WHERE added_bytes + (SELECT octet_length(replace(replace(replace(replace(replace(items::text,
   '<','\u003c'),'>','\u003e'),'&','\u0026'),chr(8232),'\u2028'),chr(8233),'\u2029')) FROM base) <= 256*1024
)
UPDATE quality_external_tools t SET
 items=base.items || additions.items,
 version=t.version + CASE WHEN t.items IS DISTINCT FROM base.items || additions.items THEN 1 ELSE 0 END,
 updated_at=CASE WHEN t.items IS DISTINCT FROM base.items || additions.items THEN NOW() ELSE t.updated_at END,
 business_entries_initialized=TRUE
FROM additions, base WHERE t.id=TRUE AND NOT t.business_entries_initialized;
