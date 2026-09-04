-- Preserve the distinction between a proven zero upstream cost and missing
-- upstream-cost evidence in the dashboard rollups. This is a follow-up
-- migration rather than a rewrite of migration 234 because local/dev
-- installations may already have applied the first version of 234.
DO $$
DECLARE
    hourly_coverage_was_missing BOOLEAN;
    daily_coverage_was_missing BOOLEAN;
BEGIN
    SELECT NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'usage_dashboard_hourly'
          AND column_name = 'upstream_expected_cost_count'
    ) INTO hourly_coverage_was_missing;

    SELECT NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'usage_dashboard_daily'
          AND column_name = 'upstream_expected_cost_count'
    ) INTO daily_coverage_was_missing;

    ALTER TABLE usage_dashboard_hourly
        ADD COLUMN IF NOT EXISTS upstream_expected_cost_count BIGINT NOT NULL DEFAULT 0,
        ADD COLUMN IF NOT EXISTS missing_upstream_cost_count BIGINT NOT NULL DEFAULT 0;

    ALTER TABLE usage_dashboard_daily
        ADD COLUMN IF NOT EXISTS upstream_expected_cost_count BIGINT NOT NULL DEFAULT 0,
        ADD COLUMN IF NOT EXISTS missing_upstream_cost_count BIGINT NOT NULL DEFAULT 0;

    -- Existing rollups predate immutable upstream evidence. Initialize them
    -- exactly once; rerunning this idempotent SQL must not erase newer evidence.
    IF hourly_coverage_was_missing THEN
        UPDATE usage_dashboard_hourly
        SET account_cost = 0,
            upstream_expected_cost_count = 0,
            missing_upstream_cost_count = total_requests;
    END IF;

    IF daily_coverage_was_missing THEN
        UPDATE usage_dashboard_daily
        SET account_cost = 0,
            upstream_expected_cost_count = 0,
            missing_upstream_cost_count = total_requests;
    END IF;
END $$;

COMMENT ON COLUMN usage_dashboard_hourly.upstream_expected_cost_count IS
    'Requests in this bucket with immutable upstream_expected_cost evidence.';
COMMENT ON COLUMN usage_dashboard_hourly.missing_upstream_cost_count IS
    'Requests in this bucket without immutable upstream_expected_cost evidence.';
COMMENT ON COLUMN usage_dashboard_daily.upstream_expected_cost_count IS
    'Requests in this bucket with immutable upstream_expected_cost evidence.';
COMMENT ON COLUMN usage_dashboard_daily.missing_upstream_cost_count IS
    'Requests in this bucket without immutable upstream_expected_cost evidence.';
