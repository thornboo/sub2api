-- Store supplier recharge ledger records for account-level cost analysis.
CREATE TABLE IF NOT EXISTS upstream_recharge_records (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    account_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    account_platform_snapshot VARCHAR(50) NOT NULL DEFAULT '',
    account_type_snapshot VARCHAR(20) NOT NULL DEFAULT '',
    type VARCHAR(20) NOT NULL DEFAULT 'recharge',
    paid_amount NUMERIC(18,6) NOT NULL DEFAULT 0,
    paid_currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
    received_credit_amount NUMERIC(18,6) NOT NULL DEFAULT 0,
    received_credit_currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    reference_fx_rate NUMERIC(18,6) NOT NULL DEFAULT 7,
    effective_cny_per_usd NUMERIC(18,6),
    recharge_discount NUMERIC(18,6),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT upstream_recharge_records_type_check
        CHECK (type IN ('recharge', 'bonus', 'adjustment')),
    CONSTRAINT upstream_recharge_records_paid_amount_check
        CHECK (paid_amount >= 0),
    CONSTRAINT upstream_recharge_records_received_credit_amount_check
        CHECK (received_credit_amount >= 0),
    CONSTRAINT upstream_recharge_records_reference_fx_rate_check
        CHECK (reference_fx_rate > 0),
    CONSTRAINT upstream_recharge_records_paid_currency_check
        CHECK (paid_currency = 'CNY'),
    CONSTRAINT upstream_recharge_records_received_credit_currency_check
        CHECK (received_credit_currency = 'USD')
);

CREATE INDEX IF NOT EXISTS idx_upstream_recharge_records_account_recorded
ON upstream_recharge_records(account_id, recorded_at DESC, id DESC)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_recharge_records_recorded
ON upstream_recharge_records(recorded_at DESC, id DESC)
WHERE deleted_at IS NULL;
