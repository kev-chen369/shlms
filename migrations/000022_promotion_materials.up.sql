-- Trusted metadata only. Availability is not permission to generate a link.
CREATE FUNCTION promotion_material_array_valid(items TEXT[], min_count INTEGER, max_count INTEGER, value_limit INTEGER, allowed_values TEXT[])
RETURNS BOOLEAN LANGUAGE SQL IMMUTABLE AS $$
    SELECT items IS NOT NULL
        AND (cardinality(items) = 0 OR array_ndims(items) = 1)
        AND cardinality(items) BETWEEN min_count AND max_count
        AND NOT EXISTS (
            SELECT 1 FROM unnest(items) AS value
            WHERE value IS NULL OR octet_length(value) NOT BETWEEN 1 AND value_limit
                OR value <> btrim(value) OR value ~ '[[:cntrl:]]'
                OR (allowed_values IS NOT NULL AND NOT (value = ANY(allowed_values)))
        )
        AND cardinality(items) = (SELECT count(DISTINCT value) FROM unnest(items) AS value);
$$;

CREATE TABLE promotion_materials (
    id UUID PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid),
    platform TEXT NOT NULL CHECK (platform IN ('JD','TB','MT')),
    material_type TEXT NOT NULL CHECK (material_type IN ('PRODUCT','ACTIVITY')),
    external_material_id TEXT NOT NULL CHECK (octet_length(external_material_id) BETWEEN 1 AND 128 AND external_material_id = btrim(external_material_id)),
    canonical_url TEXT NOT NULL CHECK (octet_length(canonical_url) BETWEEN 1 AND 2048 AND canonical_url ~ '^https://'),
    title TEXT NOT NULL CHECK (octet_length(title) BETWEEN 1 AND 256 AND title = btrim(title)),
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','ACTIVE','SUSPENDED','REMOVED')),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ NOT NULL CHECK (isfinite(ends_at) AND ends_at >= '0001-01-01T00:00:00Z' AND ends_at < '10000-01-01T00:00:00Z'),
    source_updated_at TIMESTAMPTZ NOT NULL CHECK (isfinite(source_updated_at) AND source_updated_at >= '0001-01-01T00:00:00Z' AND source_updated_at < '10000-01-01T00:00:00Z'),
    rule_version TEXT NOT NULL CHECK (octet_length(rule_version) BETWEEN 1 AND 80 AND rule_version = btrim(rule_version)),
    evidence_ref TEXT NOT NULL CHECK (octet_length(evidence_ref) BETWEEN 1 AND 128 AND evidence_ref = btrim(evidence_ref)),
    region_mode TEXT NOT NULL DEFAULT 'NATIONWIDE' CHECK (region_mode IN ('NATIONWIDE','CITIES')),
    city_codes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    business TEXT NOT NULL DEFAULT '' CHECK (octet_length(business) <= 40 AND business = btrim(business)),
    terminals TEXT[] NOT NULL DEFAULT ARRAY['H5']::TEXT[],
    UNIQUE (platform, material_type, external_material_id),
    CHECK (platform <> 'MT' OR material_type = 'ACTIVITY'),
    CHECK (material_type <> 'ACTIVITY' OR starts_at IS NOT NULL),
    CHECK (starts_at IS NULL OR (isfinite(starts_at) AND starts_at >= '0001-01-01T00:00:00Z' AND starts_at < ends_at)),
    CHECK (promotion_material_array_valid(city_codes, CASE WHEN region_mode = 'CITIES' THEN 1 ELSE 0 END, CASE WHEN region_mode = 'CITIES' THEN 64 ELSE 0 END, 32, NULL)),
    CHECK (promotion_material_array_valid(terminals, 1, 2, 16, ARRAY['H5','WX_MINI']))
);
CREATE INDEX promotion_materials_active_scope ON promotion_materials(platform, material_type, id) WHERE status = 'ACTIVE';
