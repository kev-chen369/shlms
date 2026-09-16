-- A preview is an expiring quote snapshot, never a settlement or tracking record.
ALTER TABLE promotion_positions ADD CONSTRAINT promotion_position_owner_key UNIQUE (id, owner_user_id);

CREATE TABLE promotion_previews (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) BETWEEN 1 AND 128),
    owner_user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    position_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_fingerprint TEXT NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    scene TEXT NOT NULL CHECK (length(btrim(scene)) BETWEEN 1 AND 80),
    channel TEXT NOT NULL CHECK (channel = 'JD'),
    external_product_id TEXT NOT NULL CHECK (length(btrim(external_product_id)) BETWEEN 1 AND 128),
    product_name TEXT NOT NULL CHECK (length(btrim(product_name)) BETWEEN 1 AND 256),
    currency TEXT NOT NULL CHECK (currency = 'CNY'),
    coupon_price_minor BIGINT NOT NULL CHECK (coupon_price_minor >= 0),
    promoter_estimate_minor BIGINT NOT NULL CHECK (promoter_estimate_minor >= 0),
    consumer_cashback_estimate_minor BIGINT NOT NULL CHECK (consumer_cashback_estimate_minor >= 0),
    rule_version TEXT NOT NULL CHECK (length(btrim(rule_version)) BETWEEN 1 AND 80),
    evidence_ref TEXT NOT NULL CHECK (length(btrim(evidence_ref)) BETWEEN 1 AND 256),
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (expires_at > updated_at),
    FOREIGN KEY (position_id, owner_user_id) REFERENCES promotion_positions(id, owner_user_id),
    UNIQUE (owner_user_id, idempotency_key)
);
CREATE INDEX promotion_previews_owner_created ON promotion_previews(owner_user_id, created_at DESC, id);
CREATE INDEX promotion_previews_expiry ON promotion_previews(expires_at);
