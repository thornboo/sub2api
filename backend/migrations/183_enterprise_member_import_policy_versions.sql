-- Preserve the activation semantics of import jobs created before the
-- external-facts policy was introduced. New previews use explicit activation;
-- legacy previews keep the former "groups imply active" behavior.

ALTER TABLE enterprise_member_import_jobs
    ADD COLUMN IF NOT EXISTS import_policy_version SMALLINT,
    ADD COLUMN IF NOT EXISTS commit_protocol_version SMALLINT NOT NULL DEFAULT 1;

UPDATE enterprise_member_import_jobs
SET import_policy_version = 1
WHERE import_policy_version IS NULL;

ALTER TABLE enterprise_member_import_jobs
    -- Old application instances omit this column during a rolling deploy, so
    -- the database default must remain legacy. New code writes policy 2
    -- explicitly when it creates a preview.
    ALTER COLUMN import_policy_version SET DEFAULT 1,
    ALTER COLUMN import_policy_version SET NOT NULL;

ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_policy_version_check;

ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_commit_protocol_version_check;

ALTER TABLE enterprise_member_import_jobs
    ADD CONSTRAINT enterprise_member_import_jobs_policy_version_check
        CHECK (import_policy_version IN (1, 2)),
    ADD CONSTRAINT enterprise_member_import_jobs_commit_protocol_version_check
        CHECK (commit_protocol_version IN (1, 2));

-- Queue protocol v2 keeps policy-2 jobs invisible to old workers. The trigger
-- is the database safety boundary: an old API instance cannot queue a policy-2
-- preview after silently dropping its new commit fields, while new-code writes
-- are normalized to v2 states if they use a legacy status literal.
ALTER TABLE enterprise_member_import_jobs
    DROP CONSTRAINT IF EXISTS enterprise_member_import_jobs_status_check;

ALTER TABLE enterprise_member_import_jobs
    ADD CONSTRAINT enterprise_member_import_jobs_status_check
        CHECK (status IN ('previewed', 'queued', 'queued_v2', 'processing', 'processing_v2', 'completed', 'failed'));

CREATE OR REPLACE FUNCTION enterprise_member_import_enforce_queue_protocol()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.import_policy_version >= 2 THEN
        IF NEW.status IN ('queued', 'queued_v2', 'processing', 'processing_v2')
           AND NEW.commit_protocol_version < 2 THEN
            RAISE EXCEPTION 'policy-2 enterprise member import requires commit protocol 2'
                USING ERRCODE = '23514';
        END IF;
        IF NEW.status = 'queued' THEN
            NEW.status := 'queued_v2';
        ELSIF NEW.status = 'processing' THEN
            NEW.status := 'processing_v2';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_import_queue_protocol_guard
    ON enterprise_member_import_jobs;
CREATE TRIGGER enterprise_member_import_queue_protocol_guard
    BEFORE INSERT OR UPDATE OF status, import_policy_version, commit_protocol_version ON enterprise_member_import_jobs
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_import_enforce_queue_protocol();

CREATE INDEX IF NOT EXISTS idx_enterprise_member_import_jobs_queue_v2
    ON enterprise_member_import_jobs(status, queued_at, id)
    WHERE status IN ('queued', 'queued_v2', 'processing', 'processing_v2');

COMMENT ON COLUMN enterprise_member_import_jobs.import_policy_version IS
    '1=legacy or old-instance write where groups imply active; 2=new-code explicit activation policy';

COMMENT ON COLUMN enterprise_member_import_jobs.commit_protocol_version IS
    '1=legacy commit payload; 2=payload persisted default groups and explicit activation intent';
