ALTER TABLE promotion_conversion_requests ADD CONSTRAINT promotion_conversion_owner_key UNIQUE (id, owner_user_id);

-- Client-reported operations only: not proof of delivery, clicks, or earnings.
CREATE TABLE promotion_share_events (
    owner_user_id TEXT NOT NULL,
    event_id TEXT NOT NULL CHECK (length(btrim(event_id)) BETWEEN 1 AND 128),
    conversion_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('copy_link','copy_text')),
    scene TEXT NOT NULL CHECK (length(btrim(scene)) BETWEEN 1 AND 80),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, event_id),
    FOREIGN KEY (conversion_id, owner_user_id) REFERENCES promotion_conversion_requests(id, owner_user_id)
);
CREATE INDEX promotion_share_conversion_owner ON promotion_share_events(conversion_id, owner_user_id);
