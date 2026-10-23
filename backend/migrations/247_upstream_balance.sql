-- Observation-only upstream wallets; never update gateway accounts or balances.
CREATE TABLE upstream_wallets (
    id UUID PRIMARY KEY,
    config JSONB NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    observation JSONB,
    source_identity VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'never',
    last_error VARCHAR(80) NOT NULL DEFAULT '',
    last_attempt_at TIMESTAMPTZ,
    next_query_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    failures INTEGER NOT NULL DEFAULT 0,
    lease_id UUID,
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE upstream_wallet_accounts (
    account_id BIGINT PRIMARY KEY,
    wallet_id UUID NOT NULL REFERENCES upstream_wallets(id)
);
CREATE INDEX upstream_wallet_accounts_wallet_idx ON upstream_wallet_accounts(wallet_id);
CREATE TABLE upstream_wallet_events (
    id BIGSERIAL PRIMARY KEY,
    wallet_id UUID NOT NULL REFERENCES upstream_wallets(id),
    actor_id BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    detail JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX upstream_wallet_events_wallet_idx ON upstream_wallet_events(wallet_id,id DESC);
CREATE TABLE upstream_wallet_operations (
    id UUID PRIMARY KEY,
    request_hash VARCHAR(64) NOT NULL,
    response JSONB NOT NULL DEFAULT '{}'
);
CREATE TABLE upstream_wallet_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
    email_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    recipients JSONB NOT NULL DEFAULT '[]',
    version INTEGER NOT NULL DEFAULT 1
);
INSERT INTO upstream_wallet_settings(id) VALUES(TRUE);
CREATE TABLE upstream_wallet_deliveries (
    day DATE NOT NULL,
    recipient VARCHAR(254) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'sending' CHECK(status IN ('sending','sent','failed')),
    PRIMARY KEY(day,recipient)
);
