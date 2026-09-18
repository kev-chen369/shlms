-- Consumer identity is provided by the verified API token; no promoter FK applies.
CREATE TABLE coupon_claims (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) BETWEEN 1 AND 128),
    owner_user_id TEXT NOT NULL CHECK (length(btrim(owner_user_id)) BETWEEN 1 AND 128),
    coupon_id TEXT NOT NULL REFERENCES coupon_catalog(id),
    idempotency_key TEXT NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_fingerprint TEXT NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL CHECK (status IN ('PENDING','QUERY_REQUIRED','CLAIMED','FAILED')),
    evidence_ref TEXT NOT NULL DEFAULT '' CHECK (length(evidence_ref) <= 256),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_user_id, idempotency_key),
    CHECK ((status = 'CLAIMED' AND length(btrim(evidence_ref)) > 0) OR (status <> 'CLAIMED' AND evidence_ref = ''))
);
CREATE INDEX coupon_claims_owner_created ON coupon_claims(owner_user_id, created_at DESC, id);
