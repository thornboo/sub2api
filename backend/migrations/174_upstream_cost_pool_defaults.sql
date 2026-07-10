-- Keep low-frequency supplier defaults separate from the latest recorded cost.
-- Recharge records continue to snapshot their actual values, while the UI can
-- reuse these defaults for the common add-record flow.

ALTER TABLE upstream_cost_pools
    ADD COLUMN IF NOT EXISTS default_effective_cny_per_usd NUMERIC(18,6),
    ADD COLUMN IF NOT EXISTS default_reference_fx_rate NUMERIC(18,6),
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

-- Give the operational default pool a stable identity. Historical phase-1
-- account pools belong to a system supplier and keep is_default = FALSE.
-- Prefer the canonical name, but recover a manually renamed legacy default
-- only when it is the real supplier's sole active pool. Multiple non-canonical
-- pools stay unmarked because choosing one would be ambiguous.
WITH eligible_default_pools AS (
    SELECT pool.id,
           pool.supplier_id,
           pool.name,
           COUNT(*) OVER (PARTITION BY pool.supplier_id) AS active_pool_count,
           COUNT(*) FILTER (WHERE pool.name = '主余额池')
               OVER (PARTITION BY pool.supplier_id) AS named_default_count,
           ROW_NUMBER() OVER (
               PARTITION BY pool.supplier_id
               ORDER BY CASE WHEN pool.name = '主余额池' THEN 0 ELSE 1 END,
                        pool.id ASC
           ) AS position
    FROM upstream_cost_pools pool
    JOIN upstream_suppliers supplier
      ON supplier.id = pool.supplier_id
     AND supplier.is_system = FALSE
    WHERE pool.status = 'active'
      AND pool.archived_at IS NULL
),
ranked_default_pools AS (
    SELECT id
    FROM eligible_default_pools
    WHERE position = 1
      AND (
          name = '主余额池'
          OR (named_default_count = 0 AND active_pool_count = 1)
      )
)
UPDATE upstream_cost_pools pool
SET is_default = TRUE
FROM ranked_default_pools ranked
WHERE pool.id = ranked.id;

CREATE UNIQUE INDEX IF NOT EXISTS upstream_cost_pools_one_default_per_supplier
    ON upstream_cost_pools(supplier_id)
    WHERE is_default = TRUE AND archived_at IS NULL;

UPDATE upstream_cost_pools
SET default_effective_cny_per_usd = COALESCE(
        default_effective_cny_per_usd,
        NULLIF(current_effective_cny_per_usd, 0),
        NULLIF(reference_fx_rate, 0),
        7
    ),
    default_reference_fx_rate = COALESCE(
        default_reference_fx_rate,
        NULLIF(reference_fx_rate, 0),
        7
    )
WHERE default_effective_cny_per_usd IS NULL
   OR default_reference_fx_rate IS NULL;

-- An earlier phase-1 implementation created a manual snapshot immediately when
-- a supplier default pool was created. Those rows represent configuration, not
-- a real recharge fact. Remove only the exact synthetic rows from pools that
-- still have no recharge history so clean suppliers remain deletable.
WITH synthetic_initial_snapshots AS (
    SELECT snapshot.id, snapshot.cost_pool_id
    FROM upstream_cost_snapshots snapshot
    WHERE snapshot.source_record_id IS NULL
      AND snapshot.calculation_method = 'manual'
      AND snapshot.note = '供应商默认资金池创建时生成的初始成本快照。'
      AND NOT EXISTS (
          SELECT 1
          FROM upstream_recharge_records record
          WHERE record.cost_pool_id = snapshot.cost_pool_id
      )
)
UPDATE upstream_cost_pools pool
SET current_snapshot_id = NULL,
    current_effective_cny_per_usd = NULL,
    updated_at = NOW()
FROM synthetic_initial_snapshots synthetic
WHERE pool.id = synthetic.cost_pool_id
  AND pool.current_snapshot_id = synthetic.id;

DELETE FROM upstream_cost_snapshots snapshot
WHERE snapshot.source_record_id IS NULL
  AND snapshot.calculation_method = 'manual'
  AND snapshot.note = '供应商默认资金池创建时生成的初始成本快照。'
  AND NOT EXISTS (
      SELECT 1
      FROM upstream_recharge_records record
      WHERE record.cost_pool_id = snapshot.cost_pool_id
  );

-- Repair default pools created by the intermediate implementation after the
-- synthetic-snapshot experiment was removed but before defaults stopped
-- writing current_effective_cny_per_usd. With no snapshot and no recharge fact,
-- the value is configuration, not a real current cost.
UPDATE upstream_cost_pools pool
SET current_effective_cny_per_usd = NULL,
    updated_at = NOW()
WHERE pool.is_default = TRUE
  AND pool.current_snapshot_id IS NULL
  AND pool.current_effective_cny_per_usd IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM upstream_recharge_records record
      WHERE record.cost_pool_id = pool.id
  );

ALTER TABLE upstream_cost_pools
    ALTER COLUMN default_effective_cny_per_usd SET DEFAULT 7,
    ALTER COLUMN default_effective_cny_per_usd SET NOT NULL,
    ALTER COLUMN default_reference_fx_rate SET DEFAULT 7,
    ALTER COLUMN default_reference_fx_rate SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'upstream_cost_pools'::regclass
          AND conname = 'upstream_cost_pools_default_effective_cost_check'
    ) THEN
        ALTER TABLE upstream_cost_pools
            ADD CONSTRAINT upstream_cost_pools_default_effective_cost_check
            CHECK (default_effective_cny_per_usd > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'upstream_cost_pools'::regclass
          AND conname = 'upstream_cost_pools_default_reference_fx_rate_check'
    ) THEN
        ALTER TABLE upstream_cost_pools
            ADD CONSTRAINT upstream_cost_pools_default_reference_fx_rate_check
            CHECK (default_reference_fx_rate > 0);
    END IF;
END $$;
