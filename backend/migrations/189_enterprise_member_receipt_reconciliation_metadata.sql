-- Persist reconciliation diagnostics for enterprise-member request receipts.
-- Ambiguous outcomes remain fail-closed, but operators can now distinguish and
-- monitor the reason instead of relying on process-local counters alone.

ALTER TABLE enterprise_member_budget_reservations
    ALTER COLUMN request_payload_hash TYPE VARCHAR(64)
    USING BTRIM(request_payload_hash);

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS outcome_reason VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS reconcile_attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS last_reconcile_at TIMESTAMPTZ;

ALTER TABLE enterprise_member_budget_reservations
    DROP CONSTRAINT IF EXISTS enterprise_member_budget_reservations_reconcile_attempts_check;

ALTER TABLE enterprise_member_budget_reservations
    ADD CONSTRAINT enterprise_member_budget_reservations_reconcile_attempts_check
    CHECK (reconcile_attempts >= 0);

COMMENT ON COLUMN enterprise_member_budget_reservations.outcome_reason IS
    'Machine-readable terminal or ambiguous outcome used by reconciliation and operations.';

COMMENT ON COLUMN enterprise_member_budget_reservations.reconcile_attempts IS
    'Number of background attempts that inspected this receipt after expiry.';

COMMENT ON COLUMN enterprise_member_budget_reservations.last_reconcile_at IS
    'Most recent time the background reconciler inspected this receipt.';
