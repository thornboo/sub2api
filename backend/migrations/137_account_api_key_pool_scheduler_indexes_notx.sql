CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_api_keys_account_last_used
    ON account_api_keys(account_id, last_used_at ASC NULLS FIRST, id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_api_keys_account_priority_last_used
    ON account_api_keys(account_id, priority ASC, last_used_at ASC NULLS FIRST, id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_api_keys_active_scheduler
    ON account_api_keys(account_id, priority ASC, last_used_at ASC NULLS FIRST, id)
    WHERE status = 'active';
