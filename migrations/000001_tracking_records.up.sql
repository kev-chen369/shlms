CREATE TABLE tracking_records (
    id TEXT PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    channel TEXT NOT NULL CHECK (channel IN ('JD')),
    external_product_id TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX tracking_records_user_created_idx ON tracking_records (user_id, created_at DESC);
