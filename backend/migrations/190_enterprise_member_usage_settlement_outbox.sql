-- Preserve the exact unified-billing command before applying an enterprise
-- member usage settlement. A process crash or transient database failure after
-- upstream success can then be replayed idempotently instead of losing the
-- member request record and eventually releasing its budget receipt.

CREATE TABLE IF NOT EXISTS enterprise_member_usage_settlement_outbox (
    id BIGSERIAL PRIMARY KEY,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    member_id BIGINT NOT NULL,
    enterprise_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_id TEXT NOT NULL,
    member_budget_request_id TEXT NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    command_payload JSONB NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_usage_settlement_outbox_request_unique
        UNIQUE (api_key_id, request_id),
    CONSTRAINT enterprise_member_usage_settlement_outbox_key_member_owner_fk
        FOREIGN KEY (api_key_id, member_id, enterprise_user_id)
        REFERENCES api_keys(id, member_id, user_id)
        ON DELETE RESTRICT,
    CONSTRAINT enterprise_member_usage_settlement_outbox_member_owner_fk
        FOREIGN KEY (member_id, enterprise_user_id)
        REFERENCES enterprise_members(id, enterprise_user_id)
        ON DELETE RESTRICT,
    CONSTRAINT enterprise_member_usage_settlement_outbox_identity_check
        CHECK (api_key_id > 0 AND member_id > 0 AND enterprise_user_id > 0 AND BTRIM(request_id) <> ''),
    CONSTRAINT enterprise_member_usage_settlement_outbox_fingerprint_check
        CHECK (BTRIM(request_fingerprint) <> ''),
    CONSTRAINT enterprise_member_usage_settlement_outbox_budget_request_check
        CHECK (BTRIM(member_budget_request_id) <> '')
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_usage_settlement_outbox_due
    ON enterprise_member_usage_settlement_outbox(next_attempt_at, id);

COMMENT ON TABLE enterprise_member_usage_settlement_outbox IS
    'Pending idempotent unified-billing commands for successful enterprise-member upstream requests.';

COMMENT ON COLUMN enterprise_member_usage_settlement_outbox.command_payload IS
    'Versioned in-process UsageBillingCommand snapshot, including the usage log required for atomic replay.';
