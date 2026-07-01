-- Persist user-safe (group, model) model self-check status snapshots.
-- These rows retain no account/channel/provider/upstream/error/cost detail.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS model_self_check_status_snapshots (
    id                         BIGSERIAL    PRIMARY KEY,
    group_id                   BIGINT       NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model                      VARCHAR(255) NOT NULL,
    status                     VARCHAR(20)  NOT NULL,
    reason_code                VARCHAR(80)  NOT NULL,
    eligible_account_count     INT          NOT NULL DEFAULT 0,
    checked_account_count      INT          NOT NULL DEFAULT 0,
    operational_account_count  INT          NOT NULL DEFAULT 0,
    degraded_account_count     INT          NOT NULL DEFAULT 0,
    failed_account_count       INT          NOT NULL DEFAULT 0,
    latency_ms                 INT,
    checked_at                 TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_self_check_status_snapshots_group_model_checked
    ON model_self_check_status_snapshots (group_id, model, checked_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_model_self_check_status_snapshots_checked
    ON model_self_check_status_snapshots (checked_at DESC);

COMMENT ON TABLE model_self_check_status_snapshots IS '模型自检状态快照：按分组和公开模型保存用户侧可展示的脱敏聚合结果';
COMMENT ON COLUMN model_self_check_status_snapshots.reason_code IS '脱敏原因码，例如 ok/no_available_account/no_fresh_probe/partial_degraded/all_degraded/all_probe_failed';
COMMENT ON COLUMN model_self_check_status_snapshots.eligible_account_count IS '本次快照时可参与自检的账号数量，仅保存聚合计数';
COMMENT ON COLUMN model_self_check_status_snapshots.checked_account_count IS '本次快照时有新鲜自检结果的账号数量，仅保存聚合计数';
