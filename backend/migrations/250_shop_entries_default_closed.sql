-- Reset previously seeded shop availability on upgrade; administrators can reopen each entry independently.
INSERT INTO settings (key, value)
VALUES
    ('official_shop_enabled', 'false'),
    ('external_shop_enabled', 'false'),
    ('shop_homepage_enabled', 'false')
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = NOW()
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
