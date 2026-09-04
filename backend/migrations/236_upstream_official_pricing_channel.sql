-- Bind each upstream account to an explicit administrator-maintained price
-- catalog. The selected channel is used only as a catalog container; request
-- pricing must never infer it from the downstream user's group.
ALTER TABLE upstream_account_cost_bindings
    ADD COLUMN IF NOT EXISTS official_pricing_channel_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'upstream_account_cost_bindings_official_pricing_channel_fkey'
          AND conrelid = 'upstream_account_cost_bindings'::regclass
    ) THEN
        ALTER TABLE upstream_account_cost_bindings
            ADD CONSTRAINT upstream_account_cost_bindings_official_pricing_channel_fkey
            FOREIGN KEY (official_pricing_channel_id)
            REFERENCES channels(id)
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_upstream_account_cost_bindings_official_pricing_channel
    ON upstream_account_cost_bindings(official_pricing_channel_id)
    WHERE official_pricing_channel_id IS NOT NULL;

-- Aggregates must have one unit. Preserve the original upstream debit in
-- upstream_expected_cost and derive a comparable USD reference amount here.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_expected_cost_usd NUMERIC(20,10)
    GENERATED ALWAYS AS (
        CASE
            WHEN upstream_expected_cost IS NULL THEN NULL
            WHEN upstream_price_reference_currency = 'USD' THEN upstream_expected_cost
            WHEN upstream_price_reference_currency = 'CNY'
                 AND upstream_reference_fx_rate > 0
                THEN upstream_expected_cost / upstream_reference_fx_rate
            ELSE NULL
        END
    ) STORED;

COMMENT ON COLUMN upstream_account_cost_bindings.official_pricing_channel_id IS
    'Explicit channel used as the upstream official-price catalog; never inferred from a downstream group.';
COMMENT ON COLUMN usage_logs.upstream_expected_cost_usd IS
    'Generated comparable USD reference amount; original upstream debit and currency remain in their snapshot columns.';
