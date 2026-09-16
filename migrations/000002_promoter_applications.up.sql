CREATE TABLE promoter_profiles (
    user_id TEXT PRIMARY KEY CHECK (length(btrim(user_id)) > 0),
    status TEXT NOT NULL CHECK (status IN ('NOT_APPLIED','PENDING','ENABLED','REJECTED','DISABLED')),
    reason TEXT NOT NULL DEFAULT '',
    application_id TEXT,
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    CHECK ((status = 'NOT_APPLIED' AND application_id IS NULL) OR
           (status <> 'NOT_APPLIED' AND application_id IS NOT NULL))
);

CREATE TABLE promoter_applications (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    display_name TEXT NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 80),
    scene TEXT NOT NULL CHECK (length(btrim(scene)) BETWEEN 1 AND 80),
    agreement_version TEXT NOT NULL CHECK (length(btrim(agreement_version)) BETWEEN 1 AND 80),
    consented_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING','ENABLED','REJECTED')),
    UNIQUE (user_id, idempotency_key),
    UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX promoter_one_pending_application ON promoter_applications(user_id) WHERE status = 'PENDING';
CREATE INDEX promoter_applications_user_time ON promoter_applications(user_id, consented_at DESC);
ALTER TABLE promoter_profiles ADD CONSTRAINT promoter_current_application_owner
    FOREIGN KEY (application_id, user_id) REFERENCES promoter_applications(id, user_id);
