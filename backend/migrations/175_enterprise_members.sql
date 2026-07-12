-- Complete enterprise-member domain foundation.
-- Enterprise capability is orthogonal to role; members are non-login identities
-- that own keys, ordered group access, durable budgets, and usage evidence.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_type VARCHAR(20) NOT NULL DEFAULT 'individual',
    ADD COLUMN IF NOT EXISTS enterprise_disabled_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'users'::regclass
          AND conname = 'users_account_type_check'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_account_type_check
            CHECK (account_type IN ('individual', 'enterprise'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_account_type
    ON users(account_type)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS enterprise_members (
    id BIGSERIAL PRIMARY KEY,
    enterprise_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    member_code VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    monthly_limit_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT enterprise_members_status_check
        CHECK (status IN ('active', 'disabled')),
    CONSTRAINT enterprise_members_monthly_limit_check
        CHECK (monthly_limit_usd >= 0),
    CONSTRAINT enterprise_members_version_check
        CHECK (version > 0),
    CONSTRAINT enterprise_members_owner_code_unique
        UNIQUE (enterprise_user_id, member_code),
    CONSTRAINT enterprise_members_id_owner_unique
        UNIQUE (id, enterprise_user_id)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_members_owner_status
    ON enterprise_members(enterprise_user_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_enterprise_members_deleted_at
    ON enterprise_members(deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS enterprise_members_owner_code_ci_unique
    ON enterprise_members(enterprise_user_id, LOWER(member_code));

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS member_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'api_keys'::regclass
          AND conname = 'api_keys_member_owner_fk'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT api_keys_member_owner_fk
            FOREIGN KEY (member_id, user_id)
            REFERENCES enterprise_members(id, enterprise_user_id)
            ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'api_keys'::regclass
          AND conname = 'api_keys_member_group_exclusive_check'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT api_keys_member_group_exclusive_check
            CHECK (member_id IS NULL OR group_id IS NULL);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_api_keys_member_id
    ON api_keys(member_id)
    WHERE deleted_at IS NULL AND member_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS enterprise_member_group_bindings (
    member_id BIGINT NOT NULL REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (member_id, group_id),
    CONSTRAINT enterprise_member_group_bindings_sort_order_check
        CHECK (sort_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_group_bindings_group
    ON enterprise_member_group_bindings(group_id);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_group_bindings_order
    ON enterprise_member_group_bindings(member_id, sort_order, group_id);

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS member_id BIGINT,
    ADD COLUMN IF NOT EXISTS member_code_snapshot VARCHAR(100),
    ADD COLUMN IF NOT EXISTS member_name_snapshot VARCHAR(100);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'usage_logs'::regclass
          AND conname = 'usage_logs_member_fk'
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_member_fk
            FOREIGN KEY (member_id)
            REFERENCES enterprise_members(id)
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_usage_logs_member_created_at
    ON usage_logs(member_id, created_at)
    WHERE member_id IS NOT NULL;

ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES groups(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS member_id BIGINT REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS member_code_snapshot VARCHAR(128),
    ADD COLUMN IF NOT EXISTS member_name_snapshot VARCHAR(255),
    ADD COLUMN IF NOT EXISTS member_budget_request_id VARCHAR(128);

CREATE INDEX IF NOT EXISTS idx_batch_image_jobs_member_created_at
    ON batch_image_jobs(member_id, created_at)
    WHERE member_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_batch_image_jobs_member_budget_request
    ON batch_image_jobs(member_budget_request_id)
    WHERE member_budget_request_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS enterprise_member_budget_periods (
    id BIGSERIAL PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    used_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    reserved_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_budget_periods_member_period_unique
        UNIQUE (member_id, period_start),
    CONSTRAINT enterprise_member_budget_periods_amounts_check
        CHECK (used_usd >= 0 AND reserved_usd >= 0),
    CONSTRAINT enterprise_member_budget_periods_version_check
        CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_periods_period
    ON enterprise_member_budget_periods(period_start);

CREATE TABLE IF NOT EXISTS enterprise_member_budget_reservations (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL UNIQUE,
    member_id BIGINT NOT NULL REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    reserved_usd NUMERIC(20,8) NOT NULL,
    actual_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'reserved',
    usage_log_id BIGINT UNIQUE REFERENCES usage_logs(id) ON DELETE RESTRICT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_budget_reservations_amounts_check
        CHECK (reserved_usd >= 0 AND actual_usd >= 0),
    CONSTRAINT enterprise_member_budget_reservations_status_check
        CHECK (status IN ('reserved', 'settled', 'released', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_reservations_member_period_status
    ON enterprise_member_budget_reservations(member_id, period_start, status);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_reservations_expiry
    ON enterprise_member_budget_reservations(status, expires_at)
    WHERE status = 'reserved';

CREATE TABLE IF NOT EXISTS enterprise_member_budget_entries (
    id BIGSERIAL PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    period_start DATE NOT NULL,
    kind VARCHAR(32) NOT NULL,
    request_id VARCHAR(128) UNIQUE,
    amount_usd NUMERIC(20,8) NOT NULL,
    usage_log_id BIGINT UNIQUE REFERENCES usage_logs(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_budget_entries_kind_check
        CHECK (kind IN ('usage', 'migration_opening', 'manual_adjustment', 'reconciliation')),
    CONSTRAINT enterprise_member_budget_entries_usage_shape_check
        CHECK (
            (kind = 'usage' AND request_id IS NOT NULL AND amount_usd >= 0)
            OR (kind <> 'usage' AND usage_log_id IS NULL)
        )
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_entries_member_period_created
    ON enterprise_member_budget_entries(member_id, period_start, created_at);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_budget_entries_kind_created
    ON enterprise_member_budget_entries(kind, created_at);

CREATE TABLE IF NOT EXISTS enterprise_member_import_jobs (
    id BIGSERIAL PRIMARY KEY,
    enterprise_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash CHAR(64) NOT NULL UNIQUE,
    file_hash CHAR(64) NOT NULL,
    format VARCHAR(8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'previewed',
    preview JSONB NOT NULL,
    result JSONB,
    version_fingerprint JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key_hash CHAR(64),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT enterprise_member_import_jobs_format_check CHECK (format IN ('csv', 'xlsx')),
    CONSTRAINT enterprise_member_import_jobs_status_check CHECK (status IN ('previewed', 'processing', 'completed', 'failed')),
    CONSTRAINT enterprise_member_import_jobs_owner_idempotency_unique UNIQUE (enterprise_user_id, idempotency_key_hash)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_jobs_owner_created
    ON enterprise_member_import_jobs(enterprise_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_jobs_expiry
    ON enterprise_member_import_jobs(status, expires_at)
    WHERE status = 'previewed';
