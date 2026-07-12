-- Durable, append-only audit evidence for enterprise member administration.
-- Triggers keep the audit write in the same database transaction as the
-- business mutation. Payloads are deliberately allow-listed: API key values,
-- import previews/results, and other credential material are never copied.

CREATE TABLE IF NOT EXISTS enterprise_member_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    enterprise_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    member_id BIGINT,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    entity_type VARCHAR(32) NOT NULL,
    entity_id BIGINT,
    before_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_audit_logs_action_check CHECK (BTRIM(action) <> ''),
    CONSTRAINT enterprise_member_audit_logs_entity_type_check CHECK (BTRIM(entity_type) <> ''),
    CONSTRAINT enterprise_member_audit_logs_payload_shape_check CHECK (
        jsonb_typeof(before_data) = 'object'
        AND jsonb_typeof(after_data) = 'object'
        AND jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_audit_owner_created
    ON enterprise_member_audit_logs(enterprise_user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_audit_member_created
    ON enterprise_member_audit_logs(enterprise_user_id, member_id, created_at DESC, id DESC)
    WHERE member_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enterprise_member_audit_reject_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'enterprise member audit logs are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_immutable ON enterprise_member_audit_logs;
CREATE TRIGGER enterprise_member_audit_immutable
    BEFORE UPDATE OR DELETE ON enterprise_member_audit_logs
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_audit_reject_mutation();

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
            'member_code', NEW.member_code,
            'name', NEW.name,
            'status', NEW.status,
            'monthly_limit_usd', NEW.monthly_limit_usd,
            'version', NEW.version,
            'deleted_at', NEW.deleted_at
        );
    ELSIF TG_OP = 'DELETE' THEN
        owner_id := OLD.enterprise_user_id;
        target_member_id := OLD.id;
        audit_action := 'member.deleted';
        before_payload := jsonb_build_object(
            'member_code', OLD.member_code,
            'name', OLD.name,
            'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'version', OLD.version,
            'deleted_at', OLD.deleted_at
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
            'member_code', OLD.member_code,
            'name', OLD.name,
            'status', OLD.status,
            'monthly_limit_usd', OLD.monthly_limit_usd,
            'version', OLD.version,
            'deleted_at', OLD.deleted_at
        );
        after_payload := jsonb_build_object(
            'member_code', NEW.member_code,
            'name', NEW.name,
            'status', NEW.status,
            'monthly_limit_usd', NEW.monthly_limit_usd,
            'version', NEW.version,
            'deleted_at', NEW.deleted_at
        );
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (owner_id, target_member_id, owner_id, audit_action, 'member', target_member_id, before_payload, after_payload);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_member ON enterprise_members;
CREATE TRIGGER enterprise_member_audit_member
    AFTER INSERT OR UPDATE OR DELETE ON enterprise_members
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_audit_member_change();

CREATE OR REPLACE FUNCTION enterprise_member_audit_group_binding_change()
RETURNS TRIGGER AS $$
DECLARE
    owner_id BIGINT;
    target_member_id BIGINT;
    target_group_id BIGINT;
    audit_action VARCHAR(64);
    before_payload JSONB := '{}'::jsonb;
    after_payload JSONB := '{}'::jsonb;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_member_id := OLD.member_id;
        target_group_id := OLD.group_id;
        audit_action := 'member_group.unbound';
        before_payload := jsonb_build_object('group_id', OLD.group_id, 'sort_order', OLD.sort_order);
    ELSIF TG_OP = 'INSERT' THEN
        target_member_id := NEW.member_id;
        target_group_id := NEW.group_id;
        audit_action := 'member_group.bound';
        after_payload := jsonb_build_object('group_id', NEW.group_id, 'sort_order', NEW.sort_order);
    ELSE
        target_member_id := NEW.member_id;
        target_group_id := NEW.group_id;
        audit_action := 'member_group.reordered';
        before_payload := jsonb_build_object('group_id', OLD.group_id, 'sort_order', OLD.sort_order);
        after_payload := jsonb_build_object('group_id', NEW.group_id, 'sort_order', NEW.sort_order);
    END IF;

    SELECT enterprise_user_id INTO STRICT owner_id
    FROM enterprise_members WHERE id = target_member_id;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (owner_id, target_member_id, owner_id, audit_action, 'group', target_group_id, before_payload, after_payload);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_group_binding ON enterprise_member_group_bindings;
CREATE TRIGGER enterprise_member_audit_group_binding
    AFTER INSERT OR UPDATE OR DELETE ON enterprise_member_group_bindings
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_audit_group_binding_change();

CREATE OR REPLACE FUNCTION enterprise_member_audit_key_change()
RETURNS TRIGGER AS $$
DECLARE
    row_data api_keys%ROWTYPE;
    audit_action VARCHAR(64);
    before_payload JSONB := '{}'::jsonb;
    after_payload JSONB := '{}'::jsonb;
BEGIN
    IF TG_OP = 'DELETE' THEN
        row_data := OLD;
        audit_action := 'member_key.deleted';
        before_payload := jsonb_build_object(
            'name', OLD.name, 'status', OLD.status, 'quota', OLD.quota,
            'expires_at', OLD.expires_at, 'rate_limit_5h', OLD.rate_limit_5h,
            'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'ip_whitelist', OLD.ip_whitelist, 'ip_blacklist', OLD.ip_blacklist,
            'tags', OLD.tags, 'disabled_reason', OLD.disabled_reason, 'deleted_at', OLD.deleted_at
        );
    ELSIF TG_OP = 'INSERT' THEN
        row_data := NEW;
        audit_action := 'member_key.created';
        after_payload := jsonb_build_object(
            'name', NEW.name, 'status', NEW.status, 'quota', NEW.quota,
            'expires_at', NEW.expires_at, 'rate_limit_5h', NEW.rate_limit_5h,
            'rate_limit_1d', NEW.rate_limit_1d, 'rate_limit_7d', NEW.rate_limit_7d,
            'ip_whitelist', NEW.ip_whitelist, 'ip_blacklist', NEW.ip_blacklist,
            'tags', NEW.tags, 'disabled_reason', NEW.disabled_reason, 'deleted_at', NEW.deleted_at
        );
    ELSE
        row_data := NEW;
        audit_action := CASE
            WHEN OLD.member_id IS NULL AND NEW.member_id IS NOT NULL THEN 'member_key.adopted'
            WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN 'member_key.deleted'
            WHEN OLD.status <> NEW.status AND NEW.status = 'active' THEN 'member_key.enabled'
            WHEN OLD.status <> NEW.status AND NEW.status <> 'active' THEN 'member_key.disabled'
            ELSE 'member_key.updated'
        END;
        before_payload := jsonb_build_object(
            'name', OLD.name, 'status', OLD.status, 'quota', OLD.quota,
            'group_id', OLD.group_id, 'member_id', OLD.member_id,
            'expires_at', OLD.expires_at, 'rate_limit_5h', OLD.rate_limit_5h,
            'rate_limit_1d', OLD.rate_limit_1d, 'rate_limit_7d', OLD.rate_limit_7d,
            'ip_whitelist', OLD.ip_whitelist, 'ip_blacklist', OLD.ip_blacklist,
            'tags', OLD.tags, 'disabled_reason', OLD.disabled_reason, 'deleted_at', OLD.deleted_at
        );
        after_payload := jsonb_build_object(
            'name', NEW.name, 'status', NEW.status, 'quota', NEW.quota,
            'group_id', NEW.group_id, 'member_id', NEW.member_id,
            'expires_at', NEW.expires_at, 'rate_limit_5h', NEW.rate_limit_5h,
            'rate_limit_1d', NEW.rate_limit_1d, 'rate_limit_7d', NEW.rate_limit_7d,
            'ip_whitelist', NEW.ip_whitelist, 'ip_blacklist', NEW.ip_blacklist,
            'tags', NEW.tags, 'disabled_reason', NEW.disabled_reason, 'deleted_at', NEW.deleted_at
        );
    END IF;

    IF row_data.member_id IS NULL THEN
        RETURN NULL;
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (row_data.user_id, row_data.member_id, row_data.user_id, audit_action, 'api_key', row_data.id, before_payload, after_payload);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_key ON api_keys;
CREATE TRIGGER enterprise_member_audit_key
    AFTER INSERT OR DELETE OR UPDATE OF name, status, quota, expires_at,
        rate_limit_5h, rate_limit_1d, rate_limit_7d, ip_whitelist,
        ip_blacklist, tags, disabled_reason, deleted_at, member_id, group_id
    ON api_keys
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_audit_key_change();

CREATE OR REPLACE FUNCTION enterprise_member_audit_budget_entry()
RETURNS TRIGGER AS $$
DECLARE
    owner_id BIGINT;
BEGIN
    SELECT enterprise_user_id INTO STRICT owner_id
    FROM enterprise_members WHERE id = NEW.member_id;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, after_data)
    VALUES (
        owner_id,
        NEW.member_id,
        COALESCE(NEW.actor_user_id, owner_id),
        'budget.' || NEW.kind,
        'budget_entry',
        NEW.id,
        jsonb_build_object(
            'period_start', NEW.period_start,
            'kind', NEW.kind,
            'request_id', NEW.request_id,
            'amount_usd', NEW.amount_usd,
            'usage_log_id', NEW.usage_log_id,
            'note', NEW.note
        )
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_budget ON enterprise_member_budget_entries;
CREATE TRIGGER enterprise_member_audit_budget
    AFTER INSERT ON enterprise_member_budget_entries
    FOR EACH ROW
    WHEN (NEW.kind <> 'usage')
    EXECUTE FUNCTION enterprise_member_audit_budget_entry();

CREATE OR REPLACE FUNCTION enterprise_member_audit_import_job_change()
RETURNS TRIGGER AS $$
DECLARE
    audit_action VARCHAR(64);
    before_payload JSONB := '{}'::jsonb;
    after_payload JSONB := '{}'::jsonb;
BEGIN
    IF TG_OP = 'INSERT' THEN
        audit_action := 'import.previewed';
        after_payload := jsonb_build_object(
            'file_hash', NEW.file_hash, 'format', NEW.format, 'status', NEW.status,
            'expires_at', NEW.expires_at, 'completed_at', NEW.completed_at
        );
    ELSE
        audit_action := 'import.' || NEW.status;
        before_payload := jsonb_build_object(
            'file_hash', OLD.file_hash, 'format', OLD.format, 'status', OLD.status,
            'expires_at', OLD.expires_at, 'completed_at', OLD.completed_at
        );
        after_payload := jsonb_build_object(
            'file_hash', NEW.file_hash, 'format', NEW.format, 'status', NEW.status,
            'expires_at', NEW.expires_at, 'completed_at', NEW.completed_at
        );
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, actor_user_id, action, entity_type, entity_id, before_data, after_data)
    VALUES
        (NEW.enterprise_user_id, NEW.enterprise_user_id, audit_action, 'import_job', NEW.id, before_payload, after_payload);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_import_job ON enterprise_member_import_jobs;
CREATE TRIGGER enterprise_member_audit_import_job
    AFTER INSERT OR UPDATE OF status, expires_at, completed_at ON enterprise_member_import_jobs
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_audit_import_job_change();

CREATE OR REPLACE FUNCTION enterprise_member_audit_account_change()
RETURNS TRIGGER AS $$
DECLARE
    audit_action VARCHAR(64);
BEGIN
    IF OLD.account_type = NEW.account_type
       AND OLD.enterprise_disabled_at IS NOT DISTINCT FROM NEW.enterprise_disabled_at THEN
        RETURN NEW;
    END IF;
    IF OLD.enterprise_disabled_at IS NULL AND NEW.enterprise_disabled_at IS NOT NULL THEN
        audit_action := 'enterprise.capability_disabled';
    ELSIF OLD.enterprise_disabled_at IS NOT NULL AND NEW.enterprise_disabled_at IS NULL THEN
        audit_action := 'enterprise.capability_enabled';
    ELSE
        audit_action := 'enterprise.account_type_changed';
    END IF;

    INSERT INTO enterprise_member_audit_logs
        (enterprise_user_id, actor_user_id, action, entity_type, entity_id, before_data, after_data, metadata)
    VALUES (
        NEW.id,
        NULL,
        audit_action,
        'enterprise_account',
        NEW.id,
        jsonb_build_object('account_type', OLD.account_type, 'enterprise_disabled_at', OLD.enterprise_disabled_at),
        jsonb_build_object('account_type', NEW.account_type, 'enterprise_disabled_at', NEW.enterprise_disabled_at),
        jsonb_build_object('actor_source', 'administrative_change')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_audit_account ON users;
CREATE TRIGGER enterprise_member_audit_account
    AFTER UPDATE OF account_type, enterprise_disabled_at ON users
    FOR EACH ROW
    WHEN (OLD.account_type = 'enterprise' OR NEW.account_type = 'enterprise')
    EXECUTE FUNCTION enterprise_member_audit_account_change();

COMMENT ON TABLE enterprise_member_audit_logs IS
    'Append-only, credential-safe audit evidence for enterprise member administration.';
