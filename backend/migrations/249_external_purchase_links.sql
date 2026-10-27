INSERT INTO settings (key, value)
VALUES
    ('card_shop_url', 'https://catfk.com/shop/DFMROLGS'),
    ('external_shop_url', 'https://wzyp.cn/shop/best-ai'),
    ('external_shop_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
