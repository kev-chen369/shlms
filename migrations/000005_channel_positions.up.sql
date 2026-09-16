CREATE TABLE channel_positions (
    position_id TEXT NOT NULL REFERENCES promotion_positions(id),
    channel TEXT NOT NULL CHECK (channel = 'JD'),
    account_id TEXT NOT NULL CHECK (length(btrim(account_id)) BETWEEN 1 AND 128),
    external_position_id TEXT NOT NULL CHECK (length(btrim(external_position_id)) BETWEEN 1 AND 128),
    status TEXT NOT NULL DEFAULT 'PENDING_VERIFICATION' CHECK (status = 'PENDING_VERIFICATION'),
    version BIGINT NOT NULL CHECK (version >= 1),
    configured_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (position_id, channel),
    UNIQUE (channel, account_id, external_position_id)
);
CREATE TABLE channel_position_config_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    position_id TEXT NOT NULL REFERENCES promotion_positions(id),
    request_fingerprint TEXT NOT NULL,
    before_config JSONB,
    after_config JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (actor_id, idempotency_key)
);
CREATE FUNCTION prevent_channel_position_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'channel position events are append-only';
END;
$$;
CREATE TRIGGER channel_position_event_append_only BEFORE UPDATE OR DELETE ON channel_position_config_events
    FOR EACH ROW EXECUTE FUNCTION prevent_channel_position_event_mutation();
