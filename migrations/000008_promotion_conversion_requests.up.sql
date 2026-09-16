-- Legacy shopping tracking stays intact; promotion attribution is attached separately.
ALTER TABLE promotion_previews ADD CONSTRAINT promotion_preview_owner_position_key UNIQUE (id, owner_user_id, position_id);

CREATE TABLE promotion_conversion_requests (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) BETWEEN 1 AND 128),
    owner_user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    position_id TEXT NOT NULL,
    preview_id TEXT NOT NULL,
    tracking_id TEXT NOT NULL UNIQUE REFERENCES tracking_records(id),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_fingerprint TEXT NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    scene TEXT NOT NULL CHECK (length(btrim(scene)) BETWEEN 1 AND 80),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PROCESSING','SUCCEEDED','FAILED_RETRYABLE','FAILED_FINAL')),
    channel_request_id TEXT,
    link_url TEXT,
    scheme_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (position_id, owner_user_id) REFERENCES promotion_positions(id, owner_user_id),
    FOREIGN KEY (preview_id, owner_user_id, position_id) REFERENCES promotion_previews(id, owner_user_id, position_id),
    UNIQUE (owner_user_id, idempotency_key),
    CHECK ((status = 'SUCCEEDED' AND link_url IS NOT NULL) OR (status <> 'SUCCEEDED' AND link_url IS NULL AND scheme_url IS NULL))
);
CREATE INDEX promotion_conversion_owner_created ON promotion_conversion_requests(owner_user_id, created_at DESC, id);
CREATE INDEX promotion_conversion_recovery ON promotion_conversion_requests(status, updated_at) WHERE status IN ('PENDING','PROCESSING','FAILED_RETRYABLE');
