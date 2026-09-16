ALTER TABLE promotion_conversion_requests
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    ADD COLUMN lease_expires_at TIMESTAMPTZ,
    ADD COLUMN failure_code TEXT,
    ADD CONSTRAINT conversion_processing_lease CHECK ((status = 'PROCESSING') = (lease_expires_at IS NOT NULL)),
    ADD CONSTRAINT conversion_failure_code CHECK (failure_code IS NULL OR failure_code IN ('QUERY_REQUIRED','CHANNEL_REJECTED','INVALID_RESULT')),
    ADD CONSTRAINT conversion_request_identity CHECK (status = 'PENDING' OR channel_request_id IS NOT NULL),
    ADD CONSTRAINT conversion_success_link CHECK (status <> 'SUCCEEDED' OR length(btrim(link_url)) > 0);

CREATE INDEX promotion_conversion_lease_recovery ON promotion_conversion_requests(lease_expires_at, id) WHERE status = 'PROCESSING';
