ALTER TABLE normalized_orders ADD COLUMN public_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE normalized_orders ADD CONSTRAINT normalized_orders_public_id_unique UNIQUE(public_id);
ALTER TABLE normalized_orders ADD COLUMN order_occurred_at TIMESTAMPTZ;
UPDATE normalized_orders o SET order_occurred_at=events.first_at FROM (
    SELECT channel,external_order_id,min(occurred_at) AS first_at
    FROM order_raw_events WHERE event_type='ORDER' GROUP BY channel,external_order_id
) events WHERE events.channel=o.channel AND events.external_order_id=o.external_order_id;
ALTER TABLE normalized_orders ALTER COLUMN order_occurred_at SET NOT NULL;
