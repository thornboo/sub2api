ALTER TABLE upstream_account_cost_bindings
    ADD COLUMN IF NOT EXISTS price_reference_currency VARCHAR(3) NOT NULL DEFAULT 'USD';

ALTER TABLE upstream_account_cost_bindings
    ADD COLUMN IF NOT EXISTS price_reference_confirmed BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE upstream_account_cost_bindings
    DROP CONSTRAINT IF EXISTS upstream_account_cost_bindings_price_reference_currency_check;

ALTER TABLE upstream_account_cost_bindings
    ADD CONSTRAINT upstream_account_cost_bindings_price_reference_currency_check
        CHECK (price_reference_currency IN ('CNY', 'USD'));
