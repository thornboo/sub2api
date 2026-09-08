CREATE TABLE IF NOT EXISTS feedbacks (
    id BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL,
    source TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    member_id BIGINT NULL REFERENCES enterprise_members(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT feedbacks_content_not_empty CHECK (char_length(btrim(content)) BETWEEN 1 AND 2000),
    CONSTRAINT feedbacks_source_check CHECK (source IN ('user', 'key')),
    CONSTRAINT feedbacks_status_check CHECK (status IN ('pending', 'processed')),
    CONSTRAINT feedbacks_source_shape_check CHECK (
        source = 'key' OR
        (source = 'user' AND api_key_id IS NULL AND member_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_feedbacks_status_created
    ON feedbacks(status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_feedbacks_user_created
    ON feedbacks(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_feedbacks_api_key_created
    ON feedbacks(api_key_id, created_at DESC, id DESC)
    WHERE api_key_id IS NOT NULL;

COMMENT ON TABLE feedbacks IS 'Text-only onsite feedback submitted by signed-in users or public API Key usage sessions';
COMMENT ON COLUMN feedbacks.source IS 'Submission authority: user or key';
COMMENT ON COLUMN feedbacks.api_key_id IS 'Present only for key-sourced feedback';
