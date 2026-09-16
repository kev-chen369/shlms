CREATE TABLE promotion_positions (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 80),
    scene TEXT NOT NULL CHECK (length(btrim(scene)) BETWEEN 1 AND 80),
    status TEXT NOT NULL CHECK (status IN ('ENABLED','DISABLED')),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (NOT is_default OR status = 'ENABLED')
);
CREATE UNIQUE INDEX promotion_one_default_position ON promotion_positions(owner_user_id) WHERE is_default;
CREATE INDEX promotion_positions_owner_id ON promotion_positions(owner_user_id, id);
CREATE TABLE promotion_position_requests (
    owner_user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    action TEXT NOT NULL CHECK (action IN ('CREATE','EDIT','DEFAULT','DISABLE')),
    idempotency_key TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, action, idempotency_key)
);
