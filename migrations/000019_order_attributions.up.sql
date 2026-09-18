CREATE TABLE order_attributions (
    channel TEXT NOT NULL,
    external_order_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL REFERENCES order_raw_events(id),
    status TEXT NOT NULL CHECK (status IN ('ATTRIBUTED','PENDING_REVIEW')),
    method TEXT NOT NULL CHECK (method IN ('SUB_ID','LINK_REQUEST','CHANNEL_POSITION','NONE')),
    evidence_value_hash TEXT NOT NULL CHECK (evidence_value_hash ~ '^[0-9a-f]{64}$'),
    owner_user_id TEXT,
    position_id TEXT,
    tracking_id TEXT REFERENCES tracking_records(id),
    conversion_request_id TEXT REFERENCES promotion_conversion_requests(id),
    attributed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(channel,external_order_id),
    FOREIGN KEY(channel,external_order_id) REFERENCES normalized_orders(channel,external_order_id),
    FOREIGN KEY(position_id,owner_user_id) REFERENCES promotion_positions(id,owner_user_id),
    CHECK (
        (status = 'ATTRIBUTED' AND owner_user_id IS NOT NULL AND position_id IS NOT NULL AND method <> 'NONE')
        OR
        (status = 'PENDING_REVIEW' AND owner_user_id IS NULL AND position_id IS NULL AND tracking_id IS NULL AND conversion_request_id IS NULL)
    )
);

CREATE INDEX order_attributions_owner_created_idx
    ON order_attributions(owner_user_id, attributed_at DESC, external_order_id)
    WHERE status = 'ATTRIBUTED';
