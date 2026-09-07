-- Admin-only execution evidence. Never store credentials, URLs or raw upstream
-- responses here. Account names and priorities are snapshots, not live joins.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS model_self_check_probe_rounds (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    reason_code VARCHAR(80) NOT NULL,
    winner_account_id BIGINT,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    duration_ms INT,
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    CONSTRAINT model_self_check_probe_rounds_steps_array CHECK (jsonb_typeof(steps) = 'array'),
    CONSTRAINT model_self_check_probe_rounds_duration CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_model_self_check_probe_rounds_target_started
    ON model_self_check_probe_rounds (group_id, model, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_model_self_check_probe_rounds_started
    ON model_self_check_probe_rounds (started_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_self_check_probe_rounds_active_target
    ON model_self_check_probe_rounds (group_id, model) WHERE finished_at IS NULL;

COMMENT ON TABLE model_self_check_probe_rounds IS '管理员探测链路：同一分组模型的一整轮实际执行记录，包含账号名称和优先级快照';
COMMENT ON COLUMN model_self_check_probe_rounds.winner_account_id IS '历史赢家标识，账号删除后仍保留证据，故不关联账号外键';
