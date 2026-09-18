-- Structural provenance only: real approval and current validity remain gates.
CREATE FUNCTION channel_capability_text_valid(value TEXT, value_limit INTEGER, optional BOOLEAN)
RETURNS BOOLEAN LANGUAGE SQL IMMUTABLE AS $$
    SELECT value IS NOT NULL AND ((optional AND value = '') OR
        (octet_length(value) BETWEEN 1 AND value_limit AND value = btrim(value) AND value !~ '[[:cntrl:]]'));
$$;

CREATE TABLE channel_capabilities (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
    platform TEXT NOT NULL CHECK (platform IN ('JD','TB','MT')),
    material_type TEXT NOT NULL CHECK (material_type IN ('PRODUCT','ACTIVITY')),
    kind TEXT NOT NULL CHECK (kind IN ('CATALOG','PRODUCT_PREVIEW','PRODUCT_LINK','ACTIVITY_LINK','RESULT_LOOKUP','ORDER_ATTRIBUTION')),
    media_id TEXT NOT NULL CHECK (channel_capability_text_valid(media_id,128,FALSE)),
    position_id TEXT NOT NULL REFERENCES promotion_positions(id) CHECK (channel_capability_text_valid(position_id,128,FALSE)),
    scene TEXT NOT NULL CHECK (channel_capability_text_valid(scene,80,FALSE)),
    terminal TEXT NOT NULL CHECK (terminal IN ('H5','WX_MINI')),
    city_code TEXT NOT NULL DEFAULT '' CHECK (channel_capability_text_valid(city_code,32,TRUE)),
    business TEXT NOT NULL DEFAULT '' CHECK (channel_capability_text_valid(business,40,TRUE)),
    status TEXT NOT NULL DEFAULT 'UNCONFIGURED' CHECK (status IN ('UNCONFIGURED','PENDING_VERIFICATION','READY','SUSPENDED')),
    evidence_id UUID,
    UNIQUE (platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business),
    CHECK (platform <> 'MT' OR material_type = 'ACTIVITY'),
    CHECK (kind IN ('CATALOG','RESULT_LOOKUP','ORDER_ATTRIBUTION') OR
        (material_type = 'PRODUCT' AND kind IN ('PRODUCT_PREVIEW','PRODUCT_LINK')) OR
        (material_type = 'ACTIVITY' AND kind = 'ACTIVITY_LINK')),
    CHECK (status <> 'READY' OR evidence_id IS NOT NULL)
);
CREATE TABLE channel_capability_evidence (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
    capability_id UUID NOT NULL REFERENCES channel_capabilities(id),
    owner_id TEXT NOT NULL CHECK (channel_capability_text_valid(owner_id,128,FALSE)),
    media_approval_ref TEXT NOT NULL CHECK (channel_capability_text_valid(media_approval_ref,128,FALSE)),
    source_approval_ref TEXT NOT NULL CHECK (channel_capability_text_valid(source_approval_ref,128,FALSE)),
    interface_version TEXT NOT NULL CHECK (channel_capability_text_valid(interface_version,128,FALSE)),
    real_call_evidence_ref TEXT NOT NULL CHECK (channel_capability_text_valid(real_call_evidence_ref,128,FALSE)),
    verified_at TIMESTAMPTZ NOT NULL CHECK (isfinite(verified_at) AND verified_at > '0001-01-01T00:00:00Z' AND verified_at < '10000-01-01T00:00:00Z'),
    expires_at TIMESTAMPTZ NOT NULL CHECK (isfinite(expires_at) AND expires_at < '10000-01-01T00:00:00Z'),
    recorded_by TEXT NOT NULL CHECK (channel_capability_text_valid(recorded_by,128,FALSE)),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (capability_id,id),
    CHECK (expires_at > verified_at)
);
ALTER TABLE channel_capabilities ADD CONSTRAINT channel_capabilities_evidence_scope_fk
    FOREIGN KEY (id,evidence_id) REFERENCES channel_capability_evidence(capability_id,id);

CREATE FUNCTION prevent_channel_capability_key_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.id,NEW.platform,NEW.material_type,NEW.kind,NEW.media_id,NEW.position_id,NEW.scene,NEW.terminal,NEW.city_code,NEW.business)
        IS DISTINCT FROM ROW(OLD.id,OLD.platform,OLD.material_type,OLD.kind,OLD.media_id,OLD.position_id,OLD.scene,OLD.terminal,OLD.city_code,OLD.business) THEN
        RAISE EXCEPTION 'capability identity and scope are immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER channel_capability_key_immutable BEFORE UPDATE ON channel_capabilities
    FOR EACH ROW EXECUTE FUNCTION prevent_channel_capability_key_mutation();

CREATE FUNCTION prevent_channel_capability_evidence_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'capability evidence is append-only' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER channel_capability_evidence_immutable BEFORE UPDATE OR DELETE ON channel_capability_evidence
    FOR EACH ROW EXECUTE FUNCTION prevent_channel_capability_evidence_mutation();
CREATE TRIGGER channel_capability_evidence_no_truncate BEFORE TRUNCATE ON channel_capability_evidence
    FOR EACH STATEMENT EXECUTE FUNCTION prevent_channel_capability_evidence_mutation();
