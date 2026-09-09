ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_request_id VARCHAR(128);

COMMENT ON COLUMN usage_logs.upstream_request_id IS
    'Direct upstream response request identifier, when configured for the account';
