ALTER TABLE normalized_orders DROP COLUMN order_occurred_at;
ALTER TABLE normalized_orders DROP CONSTRAINT normalized_orders_public_id_unique;
ALTER TABLE normalized_orders DROP COLUMN public_id;
