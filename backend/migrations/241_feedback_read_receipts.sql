CREATE TABLE IF NOT EXISTS feedback_read_receipts (
    feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE CASCADE,
    reader_kind TEXT NOT NULL,
    reader_id BIGINT NOT NULL,
    last_read_reply_id BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (feedback_id, reader_kind, reader_id),
    CONSTRAINT feedback_read_receipts_reader_kind_check CHECK (reader_kind IN ('user', 'key', 'admin')),
    CONSTRAINT feedback_read_receipts_reader_id_positive CHECK (reader_id > 0),
    CONSTRAINT feedback_read_receipts_last_read_reply_id_nonnegative CHECK (last_read_reply_id >= 0)
);

CREATE INDEX IF NOT EXISTS idx_feedback_read_receipts_reader
    ON feedback_read_receipts(reader_kind, reader_id, feedback_id);

CREATE INDEX IF NOT EXISTS idx_feedback_replies_feedback_author_id
    ON feedback_replies(feedback_id, author_role, id);

COMMENT ON TABLE feedback_read_receipts IS 'Per-ticket read cursors for feedback viewers. A row also means the opening message has been acknowledged for that viewer.';
COMMENT ON COLUMN feedback_read_receipts.reader_kind IS 'Feedback viewer identity kind: logged-in user, public key session, or administrator account.';
COMMENT ON COLUMN feedback_read_receipts.reader_id IS 'Stable viewer id for the selected reader kind: users.id, api_keys.id, or admin users.id.';
COMMENT ON COLUMN feedback_read_receipts.last_read_reply_id IS 'Highest feedback_replies.id displayed to the viewer; only advances.';
