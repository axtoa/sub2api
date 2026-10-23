-- Private operations ledger. Never changes billing, balances, accounts or orders.
CREATE TABLE ledger_resources (
    id UUID PRIMARY KEY,
    name VARCHAR(160) NOT NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('domain','server','other')),
    provider VARCHAR(160) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    renewal_amount NUMERIC(20,6),
    currency VARCHAR(8) NOT NULL,
    management_url VARCHAR(2000) NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (renewal_amount IS NULL OR renewal_amount > 0)
);
CREATE INDEX ledger_resources_due_idx ON ledger_resources(expires_at) WHERE active;

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('income','expense','gift')),
    amount NUMERIC(20,6) NOT NULL CHECK (amount > 0),
    currency VARCHAR(8) NOT NULL,
    category VARCHAR(80) NOT NULL,
    occurred_on DATE NOT NULL,
    counterparty VARCHAR(160) NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    resource_id UUID REFERENCES ledger_resources(id),
    voided BOOLEAN NOT NULL DEFAULT FALSE,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ledger_entries_date_idx ON ledger_entries(occurred_on DESC,created_at DESC);
CREATE INDEX ledger_entries_resource_idx ON ledger_entries(resource_id,occurred_on DESC);

CREATE TABLE ledger_events (
    id BIGSERIAL PRIMARY KEY,
    entity_id UUID NOT NULL,
    actor_id BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    before_value JSONB,
    after_value JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ledger_events_entity_idx ON ledger_events(entity_id,id DESC);

CREATE TABLE ledger_operations (
    id UUID PRIMARY KEY,
    request_hash VARCHAR(64) NOT NULL,
    response JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE ledger_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    email_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    recipients JSONB NOT NULL DEFAULT '[]',
    reminder_days JSONB NOT NULL DEFAULT '[30,7,1]',
    version INTEGER NOT NULL DEFAULT 1
);
INSERT INTO ledger_settings(id) VALUES(TRUE);

-- A durable claim before SMTP prevents repeated mail from multiple replicas.
CREATE TABLE ledger_email_deliveries (
    day DATE NOT NULL,
    recipient VARCHAR(254) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'sending' CHECK (status IN ('sending','sent','failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(day,recipient)
);
CREATE TABLE ledger_reminders_sent (
    resource_id UUID NOT NULL REFERENCES ledger_resources(id),
    expires_at TIMESTAMPTZ NOT NULL,
    threshold INTEGER NOT NULL,
    recipient VARCHAR(254) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resource_id,expires_at,threshold,recipient)
);
