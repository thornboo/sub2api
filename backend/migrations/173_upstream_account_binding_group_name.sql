-- Store the upstream supplier-side group name for an account/key binding.
-- The existing default_multiplier column remains the storage column for that
-- upstream group multiplier for compatibility with current sorting and reports.

ALTER TABLE upstream_account_cost_bindings
    ADD COLUMN IF NOT EXISTS upstream_group_name VARCHAR(120);

CREATE INDEX IF NOT EXISTS idx_upstream_account_cost_bindings_group
ON upstream_account_cost_bindings(cost_pool_id, upstream_group_name)
WHERE status = 'active';
