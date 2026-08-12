-- Concurrent indexes for Ops routing evidence filters on hot logging tables.
-- This file must run outside a transaction (the migration runner recognizes _notx).

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_routing_plan_source_created
    ON ops_error_logs (routing_plan_source, created_at)
    WHERE routing_plan_source IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_route_plan_source_created
    ON usage_logs (route_plan_source, created_at)
    WHERE route_plan_source IS NOT NULL;
