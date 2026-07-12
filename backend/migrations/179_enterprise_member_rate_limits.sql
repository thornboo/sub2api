ALTER TABLE enterprise_members
    ADD COLUMN IF NOT EXISTS rate_limit_5h NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rate_limit_1d NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rate_limit_7d NUMERIC(20,8) NOT NULL DEFAULT 0;

ALTER TABLE enterprise_members
    DROP CONSTRAINT IF EXISTS enterprise_members_rate_limits_check;

ALTER TABLE enterprise_members
    ADD CONSTRAINT enterprise_members_rate_limits_check
    CHECK (rate_limit_5h >= 0 AND rate_limit_1d >= 0 AND rate_limit_7d >= 0);

CREATE TABLE IF NOT EXISTS enterprise_member_rate_limit_periods (
    member_id BIGINT PRIMARY KEY REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    usage_5h NUMERIC(20,8) NOT NULL DEFAULT 0,
    usage_1d NUMERIC(20,8) NOT NULL DEFAULT 0,
    usage_7d NUMERIC(20,8) NOT NULL DEFAULT 0,
    window_5h_start TIMESTAMPTZ,
    window_1d_start TIMESTAMPTZ,
    window_7d_start TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_rate_limit_usage_check
        CHECK (usage_5h >= 0 AND usage_1d >= 0 AND usage_7d >= 0)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_rate_limit_periods_updated
    ON enterprise_member_rate_limit_periods(updated_at);

COMMENT ON TABLE enterprise_member_rate_limit_periods IS
    'Aggregate fixed-window spending projections shared by every API key assigned to one enterprise member.';

CREATE UNIQUE INDEX IF NOT EXISTS enterprise_member_usage_adjustment_idempotency_unique
    ON enterprise_member_audit_logs ((metadata->>'idempotency_key'))
    WHERE action = 'member.usage_adjusted';

-- Refresh the member audit allow-list for installations where migration 177
-- has already been applied.
CREATE OR REPLACE FUNCTION enterprise_member_audit_member_change()
RETURNS TRIGGER AS $$
DECLARE
    owner_id BIGINT;
    target_member_id BIGINT;
    audit_action VARCHAR(64);
    before_payload JSONB := '{}'::jsonb;
    after_payload JSONB := '{}'::jsonb;
BEGIN
    IF TG_OP = 'INSERT' THEN
        owner_id := NEW.enterprise_user_id;
        target_member_id := NEW.id;
        audit_action := 'member.created';
        after_payload := jsonb_build_object(
            'member_code', NEW.member_code, 'name', NEW.name, 'status', NEW.status,
            'monthly_limit_usd', NEW.monthly_limit_usd,
            'rate_limit_5h', NEW.rate_limit_5h, 'rate_limit_1d', NEW.rate_limit_1d, 'rate_limit_7d', NEW.rate_limit_7d,
            'version', NEW.version, 'deleted_at', NEW.deleted_at
        );
    ELSIF TG_OP = 'DELETE' THEN
        owner_id := OLD.enterprise_user_id;
        target_member_id := OLD.id;
        audit_action := 'member.deleted';
        before_payload := jsonb_build_object(
            'member_code', OLD.member_code, 'name', OLD.name, 'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'rate_limit_5h', OLD.rate_limit_5h, 'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'version', OLD.version, 'deleted_at', OLD.deleted_at
        );
    ELSE
        owner_id := NEW.enterprise_user_id;
        target_member_id := NEW.id;
        audit_action := CASE
            WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN 'member.archived'
            WHEN OLD.status <> NEW.status AND NEW.status = 'active' THEN 'member.enabled'
            WHEN OLD.status <> NEW.status AND NEW.status = 'disabled' THEN 'member.disabled'
            ELSE 'member.updated'
        END;
        before_payload := jsonb_build_object(
            'member_code', OLD.member_code, 'name', OLD.name, 'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'rate_limit_5h', OLD.rate_limit_5h, 'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'version', OLD.version, 'deleted_at', OLD.deleted_at
        );
        after_payload := jsonb_build_object(
            'member_code', NEW.member_code, 'name', NEW.name, 'status', NEW.status,
            'monthly_limit_usd', NEW.monthly_limit_usd,
            'rate_limit_5h', NEW.rate_limit_5h, 'rate_limit_1d', NEW.rate_limit_1d, 'rate_limit_7d', NEW.rate_limit_7d,
            'version', NEW.version, 'deleted_at', NEW.deleted_at
        );
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (owner_id, target_member_id, owner_id, audit_action, 'member', target_member_id, before_payload, after_payload);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
