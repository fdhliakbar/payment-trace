-- payment-trace shared schema
-- Applied automatically on first Postgres boot via /docker-entrypoint-initdb.d

CREATE TABLE IF NOT EXISTS ledger_accounts (
    account_id      TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL UNIQUE,
    balance         NUMERIC(18, 2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    order_id        TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    item_id         TEXT NOT NULL,
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    total_amount    NUMERIC(18, 2) NOT NULL CHECK (total_amount >= 0),
    status          TEXT NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'AUTHORIZED', 'COMPLETED', 'FAILED')),
    payment_ref     TEXT,
    trace_id        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders (user_id);
CREATE INDEX IF NOT EXISTS idx_orders_trace_id ON orders (trace_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);

-- Seed a demo account for checkout flow testing
INSERT INTO ledger_accounts (account_id, user_id, balance, currency)
VALUES ('acct_demo_001', 'usr_9918', 1000.00, 'USD')
ON CONFLICT (user_id) DO NOTHING;
