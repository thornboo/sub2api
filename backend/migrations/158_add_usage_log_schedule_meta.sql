-- Store non-sensitive scheduler diagnostics for usage-log troubleshooting.

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS schedule_meta JSONB;
