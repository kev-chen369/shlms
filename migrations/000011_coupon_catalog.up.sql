-- Authorized coupon material only. No consumer claim or settlement state lives here.
CREATE TABLE coupon_catalog (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) BETWEEN 1 AND 128),
    platform TEXT NOT NULL CHECK (platform IN ('JD', 'TB', 'MT')),
    claim_mode TEXT NOT NULL CHECK (claim_mode IN ('IN_SITE_VERIFIED', 'PLATFORM_CLAIM', 'BUNDLED_OFFER', 'PLATFORM_ACTIVITY')),
    title TEXT NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 256),
    scope TEXT NOT NULL CHECK (scope IN ('PRODUCT', 'CATEGORY', 'SHOP', 'ACTIVITY')),
    currency TEXT NOT NULL DEFAULT 'CNY' CHECK (currency = 'CNY'),
    discount_minor BIGINT NOT NULL CHECK (discount_minor >= 0),
    threshold_minor BIGINT NOT NULL CHECK (threshold_minor >= 0),
    city_code TEXT NOT NULL DEFAULT '' CHECK (length(city_code) <= 32),
    business TEXT NOT NULL DEFAULT '' CHECK (length(business) <= 40),
    rule_version TEXT NOT NULL CHECK (length(btrim(rule_version)) BETWEEN 1 AND 80),
    evidence_ref TEXT NOT NULL CHECK (length(btrim(evidence_ref)) BETWEEN 1 AND 256),
    verified_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (expires_at > updated_at),
    CHECK (verified_at >= updated_at)
);
CREATE INDEX coupon_catalog_visible ON coupon_catalog(platform, id)
    WHERE enabled;
