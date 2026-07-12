-- Persist the routing identity of asynchronous Grok video tasks. Status
-- queries must return to the group/account that created the upstream task;
-- selecting a fresh member candidate can query the wrong upstream tenant.

CREATE TABLE IF NOT EXISTS grok_media_tasks (
    id BIGSERIAL PRIMARY KEY,
    upstream_request_id VARCHAR(255) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    member_id BIGINT REFERENCES enterprise_members(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT grok_media_tasks_request_unique UNIQUE (upstream_request_id),
    CONSTRAINT grok_media_tasks_member_owner_fk
        FOREIGN KEY (member_id, user_id)
        REFERENCES enterprise_members(id, enterprise_user_id)
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_grok_media_tasks_owner_created
    ON grok_media_tasks(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_grok_media_tasks_member_created
    ON grok_media_tasks(member_id, created_at DESC)
    WHERE member_id IS NOT NULL;
