-- Persist the request-time evidence used to calculate what the selected
-- upstream account should have billed. This is intentionally separate from
-- account_rate_multiplier, which has an older quota/statistics meaning.
DO $$
DECLARE
    expected_cost_was_missing BOOLEAN;
BEGIN
    SELECT NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'usage_logs'
          AND column_name = 'upstream_expected_cost'
    ) INTO expected_cost_was_missing;

    ALTER TABLE usage_logs
        ADD COLUMN IF NOT EXISTS upstream_cost_binding_id BIGINT,
        ADD COLUMN IF NOT EXISTS upstream_group_multiplier NUMERIC(18,6),
        ADD COLUMN IF NOT EXISTS upstream_price_reference_currency VARCHAR(8),
        ADD COLUMN IF NOT EXISTS upstream_reference_fx_rate NUMERIC(18,6),
        ADD COLUMN IF NOT EXISTS upstream_expected_cost NUMERIC(20,10);

    -- Existing aggregate rows contain the legacy account-cost formula. Clear
    -- that derived value exactly once so it cannot be presented as audited
    -- upstream expected cost after this migration.
    IF expected_cost_was_missing THEN
        UPDATE usage_dashboard_hourly SET account_cost = 0 WHERE account_cost <> 0;
        UPDATE usage_dashboard_daily SET account_cost = 0 WHERE account_cost <> 0;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_upstream_price_reference_currency_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_upstream_price_reference_currency_check
            CHECK (
                upstream_price_reference_currency IS NULL
                OR upstream_price_reference_currency IN ('CNY', 'USD')
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_upstream_expected_cost_evidence_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_upstream_expected_cost_evidence_check
            CHECK (
                upstream_expected_cost IS NULL
                OR (
                    upstream_expected_cost >= 0
                    AND upstream_cost_binding_id IS NOT NULL
                    AND upstream_group_multiplier > 0
                    AND upstream_price_reference_currency IS NOT NULL
                )
            );
    END IF;
END $$;
