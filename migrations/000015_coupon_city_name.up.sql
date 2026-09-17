ALTER TABLE coupon_catalog ADD COLUMN city_name TEXT NOT NULL DEFAULT '' CHECK (length(city_name) <= 80);
