CREATE TABLE IF NOT EXISTS openai_auto_warmup_attempts (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_type VARCHAR(32) NOT NULL,
    reset_at TIMESTAMPTZ NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    error_code VARCHAR(128),
    model VARCHAR(200),
    request_id VARCHAR(255),
    response_id VARCHAR(255),
    latency_ms INTEGER,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    source VARCHAR(32) NOT NULL DEFAULT 'auto_warmup',
    request_kind VARCHAR(32) NOT NULL DEFAULT 'warmup',
    CONSTRAINT openai_auto_warmup_attempts_status_check
        CHECK (status IN ('pending', 'succeeded', 'failed')),
    CONSTRAINT openai_auto_warmup_attempts_source_check
        CHECK (source = 'auto_warmup'),
    CONSTRAINT openai_auto_warmup_attempts_request_kind_check
        CHECK (request_kind = 'warmup'),
    CONSTRAINT openai_auto_warmup_attempts_exact_window_key
        UNIQUE (account_id, window_type, reset_at)
);

CREATE INDEX IF NOT EXISTS openai_auto_warmup_attempts_account_attempted_idx
    ON openai_auto_warmup_attempts (account_id, attempted_at DESC);
