CREATE TABLE promotion_share_events (
    owner_user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    event_id TEXT NOT NULL CHECK (event_id ~ '^[A-Za-z0-9_-]{1,128}$'),
    conversion_request_id TEXT NOT NULL REFERENCES promotion_conversion_requests(id),
    action TEXT NOT NULL CHECK (action = 'COPY_REPORTED'),
    artifact_type TEXT NOT NULL CHECK (artifact_type IN ('link','text')),
    scene TEXT NOT NULL CHECK (scene ~ '^[A-Za-z0-9_-]{1,80}$'),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, event_id)
);
CREATE INDEX promotion_share_events_request ON promotion_share_events(conversion_request_id, recorded_at DESC);
