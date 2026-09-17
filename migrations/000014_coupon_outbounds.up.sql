CREATE TABLE coupon_outbounds (
    id TEXT PRIMARY KEY,
    owner_key TEXT NOT NULL,
    coupon_id TEXT NOT NULL REFERENCES coupon_catalog(id),
    idempotency_key TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    platform TEXT NOT NULL,
    target_host TEXT NOT NULL,
    evidence_ref TEXT NOT NULL,
    terminal TEXT NOT NULL,
    entry_point TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_key, idempotency_key)
);
CREATE INDEX coupon_outbounds_coupon_created ON coupon_outbounds(coupon_id, created_at DESC);
