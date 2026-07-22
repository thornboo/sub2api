CREATE TABLE IF NOT EXISTS account_model_protocol_capabilities (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    upstream_model VARCHAR(255) NOT NULL,
    protocol VARCHAR(64) NOT NULL,
    override_state VARCHAR(16) NOT NULL DEFAULT 'auto',
    observed_state VARCHAR(16) NOT NULL DEFAULT 'unknown',
    observed_source VARCHAR(32),
    observed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT account_model_protocol_capability_unique
        UNIQUE (account_id, upstream_model, protocol),
    CONSTRAINT account_model_protocol_capability_model_check
        CHECK (BTRIM(upstream_model) <> ''),
    CONSTRAINT account_model_protocol_capability_protocol_check
        CHECK (protocol IN (
            'anthropic_messages',
            'openai_chat_completions',
            'openai_responses'
        )),
    CONSTRAINT account_model_protocol_capability_override_check
        CHECK (override_state IN ('auto', 'supported', 'unsupported')),
    CONSTRAINT account_model_protocol_capability_observed_check
        CHECK (observed_state IN ('unknown', 'supported', 'unsupported'))
);

CREATE INDEX IF NOT EXISTS idx_account_model_protocol_capabilities_lookup
    ON account_model_protocol_capabilities (account_id, upstream_model, protocol);

-- Preserve explicit Responses probe results without treating route policy as capability.
INSERT INTO account_model_protocol_capabilities (
    account_id,
    upstream_model,
    protocol,
    observed_state,
    observed_source,
    observed_at
)
SELECT
    id,
    '*',
    'openai_responses',
    CASE
        WHEN LOWER(BTRIM(COALESCE(extra ->> 'openai_responses_supported', ''))) IN ('true', '1', 'yes')
            THEN 'supported'
        ELSE 'unsupported'
    END,
    'legacy_migration',
    NOW()
FROM accounts
WHERE extra ? 'openai_responses_supported'
  AND LOWER(BTRIM(COALESCE(extra ->> 'openai_responses_supported', ''))) IN
      ('true', '1', 'yes', 'false', '0', 'no')
ON CONFLICT (account_id, upstream_model, protocol) DO NOTHING;
