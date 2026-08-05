-- Persist bounded, non-sensitive enterprise-member routing evidence for Ops
-- drill-downs. Raw/preagg/export/alert queries continue to consume the
-- existing terminal request fields.

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS routing_plan_source TEXT,
    ADD COLUMN IF NOT EXISTS routing_snapshot_age_ms BIGINT,
    ADD COLUMN IF NOT EXISTS routing_attempts JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS route_plan_source TEXT,
    ADD COLUMN IF NOT EXISTS route_plan_snapshot_age_ms BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        WHERE c.conname = 'ops_error_logs_routing_attempts_array_check'
            AND c.conrelid = 'public.ops_error_logs'::regclass
            AND c.connamespace = 'public'::regnamespace
    ) THEN
        ALTER TABLE ops_error_logs
            ADD CONSTRAINT ops_error_logs_routing_attempts_array_check
            CHECK (jsonb_typeof(routing_attempts) = 'array') NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        WHERE c.conname = 'ops_error_logs_routing_snapshot_age_check'
            AND c.conrelid = 'public.ops_error_logs'::regclass
            AND c.connamespace = 'public'::regnamespace
    ) THEN
        ALTER TABLE ops_error_logs
            ADD CONSTRAINT ops_error_logs_routing_snapshot_age_check
            CHECK (routing_snapshot_age_ms IS NULL OR routing_snapshot_age_ms >= 0) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        WHERE c.conname = 'usage_logs_route_plan_snapshot_age_check'
            AND c.conrelid = 'public.usage_logs'::regclass
            AND c.connamespace = 'public'::regnamespace
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_route_plan_snapshot_age_check
            CHECK (route_plan_snapshot_age_ms IS NULL OR route_plan_snapshot_age_ms >= 0) NOT VALID;
    END IF;
END $$;

COMMENT ON COLUMN ops_error_logs.routing_attempts IS
    'Bounded non-sensitive routing evidence: planned/pruned/actual group ids and closed reasons only; no keys, body, credentials, member identity, or upstream secrets.';
COMMENT ON COLUMN ops_error_logs.routing_plan_source IS
    'Execution/diagnostic route plan source such as live or last_known_good.';
COMMENT ON COLUMN usage_logs.route_plan_source IS
    'Admin-only successful usage route plan source for enterprise-member model-aware routing.';

-- Production validation is intentionally staged outside startup migrations:
-- validate these constraints during a maintenance window after checking table
-- cardinality and replica/write pressure.
