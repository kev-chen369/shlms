CREATE TABLE normalized_orders (
    channel TEXT NOT NULL CHECK (channel IN ('JD','TB','MT')),
    external_order_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('CREATED','PAID','CONFIRMED','COMMISSION_CONFIRMED','SETTLEMENT_PENDING','SETTLED','CANCELLED','INVALID','REFUNDED')),
    status_at TIMESTAMPTZ NOT NULL,
    latest_evidence_id TEXT NOT NULL REFERENCES order_raw_events(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(channel,external_order_id)
);
CREATE TABLE order_projection_events (
    evidence_id TEXT PRIMARY KEY REFERENCES order_raw_events(id),
    channel TEXT NOT NULL,
    external_order_id TEXT NOT NULL,
    mapped_status TEXT NOT NULL,
    input_fingerprint TEXT NOT NULL CHECK (input_fingerprint ~ '^[0-9a-f]{64}$'),
    previous_status TEXT NOT NULL DEFAULT '',
    resulting_status TEXT NOT NULL,
    disposition TEXT NOT NULL CHECK (disposition IN ('APPLIED','STALE','DUPLICATE','INVALID_TRANSITION')),
    projected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(channel,external_order_id) REFERENCES normalized_orders(channel,external_order_id)
);
CREATE TABLE order_refund_events (
    evidence_id TEXT PRIMARY KEY REFERENCES order_raw_events(id),
    channel TEXT NOT NULL,
    external_order_id TEXT NOT NULL,
    refund_id TEXT NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    kind TEXT NOT NULL CHECK (kind IN ('PARTIAL','FULL')),
    occurred_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel,refund_id),
    FOREIGN KEY(channel,external_order_id) REFERENCES normalized_orders(channel,external_order_id)
);
