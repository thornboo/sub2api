-- Separate external migration facts from sub2api group authorization.
-- Import jobs persist the owner-selected system policy, while immutable
-- baselines preserve aggregate external usage without fabricating usage_logs.

ALTER TABLE enterprise_member_import_jobs
    ADD COLUMN IF NOT EXISTS default_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS activate_members BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_default_group_ids_shape_check;

ALTER TABLE enterprise_member_import_jobs
    ADD CONSTRAINT enterprise_member_import_jobs_default_group_ids_shape_check
        CHECK (jsonb_typeof(default_group_ids) = 'array');

CREATE TABLE IF NOT EXISTS enterprise_member_import_usage_baselines (
    id BIGSERIAL PRIMARY KEY,
    enterprise_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    member_id BIGINT NOT NULL,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE RESTRICT,
    import_job_id BIGINT NOT NULL REFERENCES enterprise_member_import_jobs(id) ON DELETE RESTRICT,
    source_row_number INTEGER NOT NULL,
    period_start DATE NOT NULL,
    billed_usd DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_import_usage_baselines_owner_member_fk
        FOREIGN KEY (member_id, enterprise_user_id)
        REFERENCES enterprise_members(id, enterprise_user_id)
        ON DELETE RESTRICT,
    CONSTRAINT enterprise_member_import_usage_baselines_source_unique
        UNIQUE (import_job_id, source_row_number),
    CONSTRAINT enterprise_member_import_usage_baselines_values_check CHECK (
        source_row_number > 0
        AND billed_usd >= 0
        AND total_tokens >= 0
        AND input_tokens >= 0
        AND output_tokens >= 0
        AND cache_tokens >= 0
        AND cache_creation_tokens >= 0
        AND cache_read_tokens >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_usage_baselines_member_period
    ON enterprise_member_import_usage_baselines(member_id, period_start, id);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_usage_baselines_owner_period
    ON enterprise_member_import_usage_baselines(enterprise_user_id, period_start, member_id);

COMMENT ON TABLE enterprise_member_import_usage_baselines IS
    'Immutable aggregate usage evidence imported from an external system; never synthetic request logs.';

CREATE OR REPLACE FUNCTION enterprise_member_import_usage_baseline_reject_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'enterprise member import usage baselines are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_import_usage_baseline_immutable
    ON enterprise_member_import_usage_baselines;
CREATE TRIGGER enterprise_member_import_usage_baseline_immutable
    BEFORE UPDATE OR DELETE ON enterprise_member_import_usage_baselines
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_import_usage_baseline_reject_mutation();
