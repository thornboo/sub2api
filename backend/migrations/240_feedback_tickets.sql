ALTER TABLE feedbacks
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS closed_by TEXT NULL,
    ADD COLUMN IF NOT EXISTS origin_member_id BIGINT NULL;

UPDATE feedbacks
SET origin_member_id = member_id
WHERE source = 'key'
  AND origin_member_id IS NULL
  AND member_id IS NOT NULL;

ALTER TABLE feedbacks
    DROP CONSTRAINT IF EXISTS feedbacks_status_check;

UPDATE feedbacks
SET status = CASE status
    WHEN 'processed' THEN 'closed'
    ELSE 'open'
END
WHERE status IN ('pending', 'processed');

UPDATE feedbacks
SET closed_at = updated_at,
    closed_by = 'admin'
WHERE status = 'closed'
  AND closed_at IS NULL;

ALTER TABLE feedbacks
    ALTER COLUMN status SET DEFAULT 'open';

ALTER TABLE feedbacks
    ADD CONSTRAINT feedbacks_status_check CHECK (status IN ('open', 'closed'));

ALTER TABLE feedbacks
    DROP CONSTRAINT IF EXISTS feedbacks_closed_by_check;

ALTER TABLE feedbacks
    ADD CONSTRAINT feedbacks_closed_by_check CHECK (closed_by IS NULL OR closed_by IN ('user', 'admin'));

CREATE TABLE IF NOT EXISTS feedback_replies (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE CASCADE,
    author_role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT feedback_replies_author_role_check CHECK (author_role IN ('user', 'admin')),
    CONSTRAINT feedback_replies_content_not_empty CHECK (char_length(btrim(content)) BETWEEN 1 AND 2000)
);

CREATE INDEX IF NOT EXISTS idx_feedbacks_status_updated
    ON feedbacks(status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_feedback_replies_feedback_created
    ON feedback_replies(feedback_id, created_at DESC, id DESC);

COMMENT ON COLUMN feedbacks.closed_at IS 'Set when a feedback ticket is closed. Legacy processed feedback maps to updated_at during migration.';
COMMENT ON COLUMN feedbacks.closed_by IS 'First closer role: user or admin. Legacy processed feedback maps to admin during migration.';
COMMENT ON COLUMN feedbacks.origin_member_id IS 'Immutable original member id snapshot for key-scoped ticket ownership; unlike member_id it is not cleared by member deletion';
COMMENT ON TABLE feedback_replies IS 'Subsequent text-only messages in a feedback ticket thread; feedbacks.content is the opening message';
