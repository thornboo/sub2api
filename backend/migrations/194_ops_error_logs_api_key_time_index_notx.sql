-- Public Key usage records are always scoped by one API Key and a bounded
-- time window. Keep that read path indexable without blocking writes while
-- the index is built on existing production tables.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_api_key_created_at
    ON ops_error_logs (api_key_id, created_at DESC)
    WHERE api_key_id IS NOT NULL;
