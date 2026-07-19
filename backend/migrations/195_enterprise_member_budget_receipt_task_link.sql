-- Make enterprise-member receipts self-describing and durably link asynchronous
-- image reservations to their Redis task. Existing rows remain `legacy` because
-- their original endpoint cannot be reconstructed safely.

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS receipt_kind VARCHAR(32) NOT NULL DEFAULT 'legacy';

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS async_task_id VARCHAR(64);

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS async_task_phase VARCHAR(16);

ALTER TABLE enterprise_member_budget_reservations
    DROP CONSTRAINT IF EXISTS enterprise_member_budget_reservations_receipt_kind_check;

ALTER TABLE enterprise_member_budget_reservations
    ADD CONSTRAINT enterprise_member_budget_reservations_receipt_kind_check
    CHECK (receipt_kind IN ('legacy', 'sync', 'async_image', 'async_video', 'batch_image'));

ALTER TABLE enterprise_member_budget_reservations
    DROP CONSTRAINT IF EXISTS enterprise_member_budget_reservations_async_task_phase_check;

ALTER TABLE enterprise_member_budget_reservations
    ADD CONSTRAINT enterprise_member_budget_reservations_async_task_phase_check
    CHECK (async_task_phase IS NULL OR async_task_phase IN ('queued', 'executing'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_enterprise_member_budget_reservations_async_task
    ON enterprise_member_budget_reservations (async_task_id)
    WHERE async_task_id IS NOT NULL;

COMMENT ON COLUMN enterprise_member_budget_reservations.receipt_kind IS
    'Explicit request lifecycle: legacy, sync, async_image, async_video, or batch_image.';

COMMENT ON COLUMN enterprise_member_budget_reservations.async_task_id IS
    'Redis image task identifier attached before asynchronous execution is allowed.';

COMMENT ON COLUMN enterprise_member_budget_reservations.async_task_phase IS
    'PostgreSQL durability fence proving whether an attached image task may have reached upstream.';
