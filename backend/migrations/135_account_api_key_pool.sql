CREATE TABLE IF NOT EXISTS account_api_keys (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL DEFAULT '',
    api_key TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 50,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    model_restriction_mode VARCHAR(20) NOT NULL DEFAULT '',
    model_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    global_cooldown_until TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    recent_request_count BIGINT NOT NULL DEFAULT 0,
    recent_error_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_api_keys_account_priority
    ON account_api_keys(account_id, priority, id);

CREATE INDEX IF NOT EXISTS idx_account_api_keys_status
    ON account_api_keys(status);

ALTER TABLE account_api_keys
    ADD COLUMN IF NOT EXISTS model_restriction_mode VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE account_api_keys
    ADD COLUMN IF NOT EXISTS model_mapping JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS account_api_key_model_cooldowns (
    id BIGSERIAL PRIMARY KEY,
    account_api_key_id BIGINT NOT NULL REFERENCES account_api_keys(id) ON DELETE CASCADE,
    upstream_model VARCHAR(200) NOT NULL,
    reason VARCHAR(100) NOT NULL DEFAULT '',
    status_code INTEGER,
    cooldown_until TIMESTAMPTZ NOT NULL,
    last_error_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error_message_sanitized TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_api_key_model_cooldowns_key_model_unique UNIQUE (account_api_key_id, upstream_model)
);

CREATE INDEX IF NOT EXISTS idx_account_api_key_model_cooldowns_active
    ON account_api_key_model_cooldowns(account_api_key_id, upstream_model, cooldown_until);
