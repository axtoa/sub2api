CREATE TABLE IF NOT EXISTS cdkshop_products (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, spec TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '',
  price_cents BIGINT NOT NULL CHECK (price_cents > 0), reward_cents BIGINT NOT NULL DEFAULT 0 CHECK (reward_cents >= 0),
  active BOOLEAN NOT NULL DEFAULT FALSE, sort_order INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE cdkshop_products ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS cdkshop_cdks (
  id BIGSERIAL PRIMARY KEY, product_id TEXT NOT NULL REFERENCES cdkshop_products(id), encrypted_code TEXT NOT NULL,
  fingerprint TEXT NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available','reserved','delivered')),
  order_id TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS cdkshop_cdks_available ON cdkshop_cdks(product_id, id) WHERE status='available';
CREATE TABLE IF NOT EXISTS cdkshop_orders (
  id TEXT PRIMARY KEY, request_id TEXT NOT NULL UNIQUE, user_id BIGINT, email TEXT NOT NULL, recovery_hash TEXT NOT NULL, recovery_secret TEXT NOT NULL,
  product_id TEXT NOT NULL, product_name TEXT NOT NULL, price_cents BIGINT NOT NULL, reward_cents BIGINT NOT NULL DEFAULT 0,
  payment_status TEXT NOT NULL DEFAULT 'pending' CHECK(payment_status IN ('pending','paid','failed')),
  delivery_status TEXT NOT NULL DEFAULT 'pending' CHECK(delivery_status IN ('pending','delivered','failed')),
  pay_url TEXT NOT NULL DEFAULT '', qr_code TEXT NOT NULL DEFAULT '', trade_no TEXT NOT NULL DEFAULT '', cdk_id BIGINT REFERENCES cdkshop_cdks(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), paid_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS cdkshop_orders_user ON cdkshop_orders(user_id, created_at DESC);
CREATE TABLE IF NOT EXISTS cdkshop_reward_tasks (
  order_id TEXT PRIMARY KEY REFERENCES cdkshop_orders(id), user_id BIGINT NOT NULL, email TEXT NOT NULL, amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','issued')), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  issued_at TIMESTAMPTZ, issued_by BIGINT, note TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS cdkshop_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
