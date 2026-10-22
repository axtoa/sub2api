INSERT INTO settings (key, value)
VALUES ('official_shop_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
