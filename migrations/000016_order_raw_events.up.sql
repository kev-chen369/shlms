CREATE TABLE order_raw_events (
    id TEXT PRIMARY KEY,
    channel TEXT NOT NULL CHECK (channel IN ('JD','TB','MT')),
    event_id TEXT NOT NULL CHECK (length(btrim(event_id)) BETWEEN 1 AND 128),
    external_order_id TEXT NOT NULL CHECK (length(btrim(external_order_id)) BETWEEN 1 AND 128),
    event_type TEXT NOT NULL CHECK (event_type IN ('ORDER','REFUND')),
    occurred_at TIMESTAMPTZ NOT NULL,
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256)=32),
    payload_nonce BYTEA NOT NULL CHECK (octet_length(payload_nonce)=12),
    payload_ciphertext BYTEA NOT NULL CHECK (octet_length(payload_ciphertext)>16),
    key_version TEXT NOT NULL CHECK (length(btrim(key_version)) BETWEEN 1 AND 64),
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (channel,event_id)
);
CREATE INDEX order_raw_events_order_time ON order_raw_events(channel,external_order_id,occurred_at);
