CREATE TABLE promoter_admin_audits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('REVIEW','DISABLE')),
    idempotency_key TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES promoter_profiles(user_id),
    request_fingerprint TEXT NOT NULL,
    before_profile JSONB NOT NULL,
    after_profile JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (actor_id, action, idempotency_key)
);
CREATE INDEX promoter_admin_audits_user_time ON promoter_admin_audits(user_id, created_at DESC);
CREATE FUNCTION prevent_promoter_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'promoter audit records are append-only';
END;
$$;
CREATE TRIGGER promoter_audit_append_only BEFORE UPDATE OR DELETE ON promoter_admin_audits
    FOR EACH ROW EXECUTE FUNCTION prevent_promoter_audit_mutation();
