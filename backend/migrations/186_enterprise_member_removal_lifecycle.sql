-- Separate reversible archive from irreversible owner-facing removal.
-- Members without historical facts can still be physically deleted. Members
-- with facts retain an internal tombstone so restrictive foreign keys and
-- append-only billing/audit evidence remain valid.

ALTER TABLE enterprise_members
    ADD COLUMN IF NOT EXISTS removed_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'enterprise_members'::regclass
          AND conname = 'enterprise_members_removed_requires_archive_check'
    ) THEN
        ALTER TABLE enterprise_members
            ADD CONSTRAINT enterprise_members_removed_requires_archive_check
            CHECK (removed_at IS NULL OR deleted_at IS NOT NULL)
            NOT VALID;
    END IF;
END $$;

ALTER TABLE enterprise_members
    VALIDATE CONSTRAINT enterprise_members_removed_requires_archive_check;

CREATE INDEX IF NOT EXISTS idx_enterprise_members_owner_visible
    ON enterprise_members(enterprise_user_id, id)
    WHERE removed_at IS NULL;

COMMENT ON COLUMN enterprise_members.removed_at IS
    'Irreversible owner-facing removal timestamp. Non-null rows are internal tombstones and are excluded from member management.';

-- Migration 179 last refreshed this function with rate-limit fields. Keep that
-- allow-list and add lifecycle-specific actions plus removed_at evidence.
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
            'version', NEW.version, 'deleted_at', NEW.deleted_at, 'removed_at', NEW.removed_at
        );
    ELSIF TG_OP = 'DELETE' THEN
        owner_id := OLD.enterprise_user_id;
        target_member_id := OLD.id;
        audit_action := 'member.deleted';
        before_payload := jsonb_build_object(
            'member_code', OLD.member_code, 'name', OLD.name, 'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'rate_limit_5h', OLD.rate_limit_5h, 'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'version', OLD.version, 'deleted_at', OLD.deleted_at, 'removed_at', OLD.removed_at
        );
    ELSE
        owner_id := NEW.enterprise_user_id;
        target_member_id := NEW.id;
        audit_action := CASE
            WHEN OLD.removed_at IS NULL AND NEW.removed_at IS NOT NULL THEN 'member.removed'
            WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN 'member.archived'
            WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN 'member.restored'
            WHEN OLD.status <> NEW.status AND NEW.status = 'active' THEN 'member.enabled'
            WHEN OLD.status <> NEW.status AND NEW.status = 'disabled' THEN 'member.disabled'
            ELSE 'member.updated'
        END;
        before_payload := jsonb_build_object(
            'member_code', OLD.member_code, 'name', OLD.name, 'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'rate_limit_5h', OLD.rate_limit_5h, 'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'version', OLD.version, 'deleted_at', OLD.deleted_at, 'removed_at', OLD.removed_at
        );
        after_payload := jsonb_build_object(
            'member_code', NEW.member_code, 'name', NEW.name, 'status', NEW.status,
            'monthly_limit_usd', NEW.monthly_limit_usd,
            'rate_limit_5h', NEW.rate_limit_5h, 'rate_limit_1d', NEW.rate_limit_1d, 'rate_limit_7d', NEW.rate_limit_7d,
            'version', NEW.version, 'deleted_at', NEW.deleted_at, 'removed_at', NEW.removed_at
        );
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (owner_id, target_member_id, owner_id, audit_action, 'member', target_member_id, before_payload, after_payload);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
