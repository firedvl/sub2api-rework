CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_upstream_request_id
    ON usage_logs (upstream_request_id, created_at DESC)
    WHERE upstream_request_id IS NOT NULL;
