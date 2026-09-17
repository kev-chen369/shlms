-- Only opaque proof references are stored. Raw upstream payloads and secrets stay outside this table.
CREATE TABLE coupon_sync_events (
    coupon_id TEXT NOT NULL REFERENCES coupon_catalog(id),
    evidence_ref TEXT NOT NULL CHECK (length(btrim(evidence_ref)) BETWEEN 1 AND 256),
    verified_at TIMESTAMPTZ NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('PUBLISH', 'REVOKE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (coupon_id, evidence_ref)
);
CREATE INDEX coupon_sync_events_coupon_time ON coupon_sync_events(coupon_id, verified_at DESC);
