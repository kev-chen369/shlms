-- Scope identity is maintained by the approved channel importer in a later task.
ALTER TABLE coupon_catalog
    ADD COLUMN scope_external_id TEXT NOT NULL DEFAULT '' CHECK (length(scope_external_id) <= 128),
    ADD COLUMN scope_name TEXT NOT NULL DEFAULT '' CHECK (length(scope_name) <= 256);

CREATE TABLE coupon_products (
    coupon_id TEXT NOT NULL REFERENCES coupon_catalog(id) ON DELETE CASCADE,
    external_product_id TEXT NOT NULL CHECK (length(btrim(external_product_id)) BETWEEN 1 AND 128),
    title TEXT NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 256),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (coupon_id, external_product_id),
    CHECK (verified_at >= updated_at AND expires_at > updated_at)
);
CREATE INDEX coupon_products_visible ON coupon_products(coupon_id, external_product_id) WHERE enabled;
