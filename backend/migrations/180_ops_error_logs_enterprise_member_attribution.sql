-- Preserve request-time enterprise-member attribution on failed requests.
-- These columns deliberately remain nullable and do not carry a foreign key:
-- authentication can fail before a member is known, and telemetry persistence
-- must not depend on the current lifecycle state of the member entity.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS member_id BIGINT,
    ADD COLUMN IF NOT EXISTS member_code_snapshot VARCHAR(100),
    ADD COLUMN IF NOT EXISTS member_name_snapshot VARCHAR(100);

COMMENT ON COLUMN ops_error_logs.member_id IS
    'Enterprise member identity resolved from the authenticated API key at request time; NULL when attribution is unavailable.';
COMMENT ON COLUMN ops_error_logs.member_code_snapshot IS
    'Request-time enterprise member code snapshot retained independently from later member edits or archival.';
COMMENT ON COLUMN ops_error_logs.member_name_snapshot IS
    'Request-time enterprise member display-name snapshot retained independently from later member edits or archival.';
