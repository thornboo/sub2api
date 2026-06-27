-- Create pricing-driven model self-check tables.
-- User-facing model health reads these tables, not channel_monitor_*.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS model_self_check_config (
    id          BIGSERIAL   PRIMARY KEY,
    channel_id  BIGINT      NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    model       VARCHAR(255) NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_self_check_config_channel_model
    ON model_self_check_config (channel_id, model);

CREATE INDEX IF NOT EXISTS idx_model_self_check_config_enabled
    ON model_self_check_config (enabled)
    WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS model_self_check_histories (
    id          BIGSERIAL    PRIMARY KEY,
    model       VARCHAR(255) NOT NULL,
    account_id  BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    platform    VARCHAR(50)  NOT NULL,
    status      VARCHAR(20)  NOT NULL,
    latency_ms  INT,
    http_status INT,
    error_code  VARCHAR(100),
    checked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_self_check_histories_model_account_checked
    ON model_self_check_histories (model, account_id, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_self_check_histories_checked_at
    ON model_self_check_histories (checked_at DESC);

COMMENT ON TABLE model_self_check_config IS '模型自检开关：按渠道和公开模型名配置';
COMMENT ON TABLE model_self_check_histories IS '模型自检明细：按公开模型和上游账号记录探测结果';
COMMENT ON COLUMN model_self_check_config.model IS '公开模型名，用户侧可展示';
COMMENT ON COLUMN model_self_check_histories.account_id IS '被探测账号，仅内部排障使用，不进入用户 DTO';
COMMENT ON COLUMN model_self_check_histories.error_code IS '归一化错误码，仅内部排障使用，不进入用户 DTO';
