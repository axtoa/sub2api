CREATE TABLE IF NOT EXISTS shop_products (
    id UUID PRIMARY KEY,
    payload JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shop_orders (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    product_id UUID NOT NULL REFERENCES shop_products(id),
    request_key UUID NOT NULL,
    request_hash TEXT NOT NULL,
    payload JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, request_key)
);
CREATE INDEX IF NOT EXISTS shop_orders_user_created_idx ON shop_orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS shop_orders_created_idx ON shop_orders(created_at DESC);

-- No prices or checkout links are assumed. Operators configure products before sale.
INSERT INTO shop_products (id, payload)
SELECT id::uuid, jsonb_build_object(
    'id', id, 'brand', brand, 'name', name, 'spec', '月度订阅',
    'description', '为自己的账号订阅官方产品，由工作人员人工核对与开通。',
    'requirements', '请先确认账号当前订阅状态与购买资格。',
    'purchase_notice', E'本站提供第三方人工代购服务，不代表品牌官方授权。\n付款在外部平台完成，经人工核对后处理。\n请勿在备注中提交密码、Session 或验证码。',
    'delivery_hours', 24, 'price_cents', NULL, 'payment_url', '',
    'reference_amount', NULL, 'reference_currency', 'USD',
    'reference_url', '', 'reference_date', '', 'active', true,
    'sort', sort, 'version', 1
)
FROM (VALUES
    ('88000000-0000-4000-8000-000000000001', 'ChatGPT', 'Go', 10),
    ('88000000-0000-4000-8000-000000000002', 'ChatGPT', 'Plus', 20),
    ('88000000-0000-4000-8000-000000000003', 'ChatGPT', 'Pro 5x', 30),
    ('88000000-0000-4000-8000-000000000004', 'ChatGPT', 'Pro 20x', 40),
    ('88000000-0000-4000-8000-000000000005', 'ChatGPT', 'Pro 50x', 50),
    ('88000000-0000-4000-8000-000000000006', 'Anthropic', 'Claude Pro', 60)
) AS defaults(id, brand, name, sort)
ON CONFLICT (id) DO NOTHING;
