-- Extend the existing durable budget reservation into the request receipt used
-- by every billable enterprise-member request, including unlimited members.

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES groups(id) ON DELETE RESTRICT;

ALTER TABLE enterprise_member_budget_reservations
    ADD COLUMN IF NOT EXISTS request_payload_hash CHAR(64) NOT NULL DEFAULT '';

ALTER TABLE enterprise_member_budget_reservations
    DROP CONSTRAINT IF EXISTS enterprise_member_budget_reservations_status_check;

ALTER TABLE enterprise_member_budget_reservations
    ADD CONSTRAINT enterprise_member_budget_reservations_status_check
    CHECK (status IN ('reserved', 'settled', 'released', 'expired', 'ambiguous'));

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_reservations_ambiguous
    ON enterprise_member_budget_reservations(expires_at, id)
    WHERE status = 'ambiguous';

COMMENT ON COLUMN enterprise_member_budget_reservations.request_payload_hash IS
    'Hash of the authorized request body; detects request-id reuse before upstream execution.';

COMMENT ON COLUMN enterprise_member_budget_reservations.group_id IS
    'Initially authorized member group; the final successful group remains authoritative in usage_logs.';

COMMENT ON INDEX idx_enterprise_member_budget_reservations_ambiguous IS
    'Supports reconciliation of requests whose outcome cannot be proven after process or database failure.';
