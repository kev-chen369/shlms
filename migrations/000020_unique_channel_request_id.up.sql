CREATE UNIQUE INDEX promotion_conversion_channel_request_unique
    ON promotion_conversion_requests(channel_request_id)
    WHERE channel_request_id IS NOT NULL;
