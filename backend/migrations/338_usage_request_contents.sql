-- Client input/output is deliberately separate from permanent usage/billing
-- rows. Do not add bodies to usage log DTOs, list queries, exports or caches.
CREATE TABLE IF NOT EXISTS usage_request_contents (
    capture_id UUID NOT NULL,
    billing_request_id VARCHAR(128) NOT NULL DEFAULT '',
    api_key_id BIGINT NOT NULL,
    client_request_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL,
    request_body BYTEA NOT NULL,
    response_body BYTEA NOT NULL,
    format_version INTEGER NOT NULL DEFAULT 1 CHECK (format_version = 1),
    storage_bytes INTEGER NOT NULL CHECK (storage_bytes >= 0),
    PRIMARY KEY (capture_id, billing_request_id),
    CHECK (expires_at = created_at + INTERVAL '24 hours')
);

-- All instances serialize only their small background batches on this row;
-- regular gateway/billing requests never touch it. Avoid per-request SUM scans.
CREATE TABLE IF NOT EXISTS usage_content_budget (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (used_bytes >= 0)
);
INSERT INTO usage_content_budget(id,used_bytes) VALUES (1,0) ON CONFLICT (id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_usage_request_contents_billing
    ON usage_request_contents (api_key_id, billing_request_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_request_contents_client
    ON usage_request_contents (api_key_id, client_request_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_request_contents_expiry
    ON usage_request_contents (expires_at);

-- An append/delete TTL table needs vacuum earlier than permanent statistics.
ALTER TABLE usage_request_contents SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_vacuum_threshold = 1000,
    toast.autovacuum_vacuum_scale_factor = 0.05,
    toast.autovacuum_vacuum_threshold = 1000
);
