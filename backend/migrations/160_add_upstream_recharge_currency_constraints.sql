-- Align databases that applied the early 159 migration before currency checks were added.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'upstream_recharge_records'::regclass
          AND conname = 'upstream_recharge_records_paid_currency_check'
    ) THEN
        ALTER TABLE upstream_recharge_records
        ADD CONSTRAINT upstream_recharge_records_paid_currency_check
        CHECK (paid_currency = 'CNY');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'upstream_recharge_records'::regclass
          AND conname = 'upstream_recharge_records_received_credit_currency_check'
    ) THEN
        ALTER TABLE upstream_recharge_records
        ADD CONSTRAINT upstream_recharge_records_received_credit_currency_check
        CHECK (received_credit_currency = 'USD');
    END IF;
END $$;
