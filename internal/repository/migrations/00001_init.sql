-- +goose Up
CREATE TABLE IF NOT EXISTS gm_users (
    id BIGSERIAL PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    current_balance NUMERIC(20, 2) NOT NULL DEFAULT 0,
    withdrawn_balance NUMERIC(20, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gm_sessions (
    token TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES gm_users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gm_orders (
    number TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES gm_users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    accrual NUMERIC(20, 2),
    rewarded BOOLEAN NOT NULL DEFAULT FALSE,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gm_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES gm_users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL UNIQUE,
    sum NUMERIC(20, 2) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS gm_orders_user_uploaded_idx ON gm_orders(user_id, uploaded_at DESC);
CREATE INDEX IF NOT EXISTS gm_withdrawals_user_processed_idx ON gm_withdrawals(user_id, processed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS gm_withdrawals;
DROP TABLE IF EXISTS gm_orders;
DROP TABLE IF EXISTS gm_sessions;
DROP TABLE IF EXISTS gm_users;
