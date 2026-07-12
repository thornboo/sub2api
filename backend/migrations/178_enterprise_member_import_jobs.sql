-- Turn enterprise member commit into a durable, restart-safe background job.
-- Generated/imported key plaintext is stored only as encrypted result material
-- and is atomically consumed once by the authenticated owner.

ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_status_check;

ALTER TABLE enterprise_member_import_jobs
    ADD CONSTRAINT enterprise_member_import_jobs_status_check
        CHECK (status IN ('previewed', 'queued', 'processing', 'completed', 'failed')),
    ADD COLUMN IF NOT EXISTS selected_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS queued_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS lock_owner VARCHAR(128),
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error_code VARCHAR(80),
    ADD COLUMN IF NOT EXISTS error_summary TEXT,
    ADD COLUMN IF NOT EXISTS result_secrets_ciphertext TEXT,
    ADD COLUMN IF NOT EXISTS result_secrets_consumed_at TIMESTAMPTZ;

ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_selected_rows_shape_check;

ALTER TABLE enterprise_member_import_jobs
    ADD CONSTRAINT enterprise_member_import_jobs_selected_rows_shape_check
        CHECK (jsonb_typeof(selected_rows) = 'array'),
    ADD CONSTRAINT enterprise_member_import_jobs_attempt_count_check
        CHECK (attempt_count >= 0);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_jobs_queue
    ON enterprise_member_import_jobs(status, queued_at, id)
    WHERE status IN ('queued', 'processing');

COMMENT ON COLUMN enterprise_member_import_jobs.result_secrets_ciphertext IS
    'Short-lived encrypted one-time result material; never plaintext API keys.';
