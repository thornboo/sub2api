ALTER TABLE model_self_check_histories
    ADD COLUMN IF NOT EXISTS input_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_tokens INT NOT NULL DEFAULT 0;

COMMENT ON COLUMN model_self_check_histories.input_tokens IS '模型自检探针消耗的输入 token 数，仅管理员聚合展示，不进入用户账单';
COMMENT ON COLUMN model_self_check_histories.output_tokens IS '模型自检探针消耗的输出 token 数，仅管理员聚合展示，不进入用户账单';
