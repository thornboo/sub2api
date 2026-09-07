CREATE TABLE IF NOT EXISTS key_announcement_reads (
    id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_key_announcement_reads_announcement_api_key
    ON key_announcement_reads(announcement_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_key_announcement_reads_api_key_id
    ON key_announcement_reads(api_key_id);

CREATE INDEX IF NOT EXISTS idx_key_announcement_reads_announcement_id
    ON key_announcement_reads(announcement_id);

CREATE INDEX IF NOT EXISTS idx_key_announcement_reads_read_at
    ON key_announcement_reads(read_at);

COMMENT ON TABLE key_announcement_reads IS 'API Key scoped announcement read records';
COMMENT ON COLUMN key_announcement_reads.read_at IS 'API Key first read time';
