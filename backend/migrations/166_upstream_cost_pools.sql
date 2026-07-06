-- Move upstream recharge ownership from account-only records to supplier cost pools.
-- Phase 1 keeps current behavior equivalent by giving each existing account an
-- default pool under a shared uncategorized supplier.

CREATE TABLE IF NOT EXISTS upstream_suppliers (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    note TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT upstream_suppliers_status_check
        CHECK (status IN ('active', 'archived'))
);

CREATE TABLE IF NOT EXISTS upstream_cost_pools (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES upstream_suppliers(id) ON DELETE RESTRICT,
    name VARCHAR(160) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    base_currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
    credit_currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    reference_fx_rate NUMERIC(18,6) NOT NULL DEFAULT 7,
    cost_method VARCHAR(20) NOT NULL DEFAULT 'latest',
    current_effective_cny_per_usd NUMERIC(18,6),
    current_snapshot_id BIGINT,
    balance_query_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    balance_provider VARCHAR(50),
    balance_endpoint TEXT,
    balance_auth_mode VARCHAR(30),
    balance_auth_header VARCHAR(100),
    balance_low_threshold NUMERIC(18,6),
    last_balance_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    note TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT upstream_cost_pools_status_check
        CHECK (status IN ('active', 'archived')),
    CONSTRAINT upstream_cost_pools_currency_check
        CHECK (base_currency = 'CNY' AND credit_currency = 'USD'),
    CONSTRAINT upstream_cost_pools_reference_fx_rate_check
        CHECK (reference_fx_rate > 0),
    CONSTRAINT upstream_cost_pools_cost_method_check
        CHECK (cost_method IN ('latest', 'weighted', 'manual'))
);

CREATE TABLE IF NOT EXISTS upstream_account_cost_bindings (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    cost_pool_id BIGINT NOT NULL REFERENCES upstream_cost_pools(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    default_multiplier NUMERIC(18,6) NOT NULL DEFAULT 1,
    model_family_multipliers JSONB NOT NULL DEFAULT '[]'::jsonb,
    note TEXT,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_account_cost_bindings_status_check
        CHECK (status IN ('active', 'archived')),
    CONSTRAINT upstream_account_cost_bindings_default_multiplier_check
        CHECK (default_multiplier > 0),
    CONSTRAINT upstream_account_cost_bindings_valid_range_check
        CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE TABLE IF NOT EXISTS upstream_cost_snapshots (
    id BIGSERIAL PRIMARY KEY,
    cost_pool_id BIGINT NOT NULL REFERENCES upstream_cost_pools(id) ON DELETE RESTRICT,
    effective_cny_per_usd NUMERIC(18,6) NOT NULL,
    reference_fx_rate NUMERIC(18,6) NOT NULL DEFAULT 7,
    calculation_method VARCHAR(20) NOT NULL DEFAULT 'latest',
    source_record_id BIGINT REFERENCES upstream_recharge_records(id) ON DELETE SET NULL,
    included_record_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note TEXT,
    CONSTRAINT upstream_cost_snapshots_effective_cost_check
        CHECK (effective_cny_per_usd > 0),
    CONSTRAINT upstream_cost_snapshots_reference_fx_rate_check
        CHECK (reference_fx_rate > 0),
    CONSTRAINT upstream_cost_snapshots_calculation_method_check
        CHECK (calculation_method IN ('latest', 'weighted', 'manual')),
    CONSTRAINT upstream_cost_snapshots_valid_range_check
        CHECK (valid_to IS NULL OR valid_to > valid_from)
);

ALTER TABLE upstream_recharge_records
    ADD COLUMN IF NOT EXISTS cost_pool_id BIGINT,
    ADD COLUMN IF NOT EXISTS source_account_id_snapshot BIGINT,
    ADD COLUMN IF NOT EXISTS merged_from_pool_id BIGINT,
    ADD COLUMN IF NOT EXISTS source VARCHAR(30) NOT NULL DEFAULT 'legacy_account',
    ADD COLUMN IF NOT EXISTS external_order_id VARCHAR(120),
    ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS voided_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS void_reason TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'upstream_recharge_records'::regclass
          AND conname = 'fk_upstream_recharge_records_cost_pool_id'
    ) THEN
        ALTER TABLE upstream_recharge_records
        ADD CONSTRAINT fk_upstream_recharge_records_cost_pool_id
        FOREIGN KEY (cost_pool_id) REFERENCES upstream_cost_pools(id) ON DELETE RESTRICT NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'upstream_recharge_records'::regclass
          AND conname = 'fk_upstream_recharge_records_merged_from_pool_id'
    ) THEN
        ALTER TABLE upstream_recharge_records
        ADD CONSTRAINT fk_upstream_recharge_records_merged_from_pool_id
        FOREIGN KEY (merged_from_pool_id) REFERENCES upstream_cost_pools(id) ON DELETE SET NULL NOT VALID;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_upstream_suppliers_status_name
ON upstream_suppliers(status, name);

WITH active_supplier_duplicates AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY name ORDER BY id) AS row_number
    FROM upstream_suppliers
    WHERE archived_at IS NULL
)
UPDATE upstream_suppliers s
SET status = 'archived',
    archived_at = NOW(),
    updated_at = NOW(),
    note = COALESCE(s.note || E'\n', '') || '自动归档：创建 active 供应商名称唯一索引前发现重复名称。'
FROM active_supplier_duplicates duplicate
WHERE s.id = duplicate.id
  AND duplicate.row_number > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_suppliers_active_name
ON upstream_suppliers(name)
WHERE archived_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_cost_pools_supplier_status
ON upstream_cost_pools(supplier_id, status, name);

CREATE INDEX IF NOT EXISTS idx_upstream_recharge_records_cost_pool_recorded
ON upstream_recharge_records(cost_pool_id, recorded_at DESC, id DESC)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_account_cost_bindings_pool_status
ON upstream_account_cost_bindings(cost_pool_id, status);

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_account_cost_bindings_active
ON upstream_account_cost_bindings(account_id)
WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_upstream_cost_snapshots_pool_valid_from
ON upstream_cost_snapshots(cost_pool_id, valid_from DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_upstream_cost_snapshots_active
ON upstream_cost_snapshots(cost_pool_id)
WHERE valid_to IS NULL;

INSERT INTO upstream_suppliers (name, note)
VALUES ('未归类供应商', '阶段 1 等价迁移自动创建；每个账号默认资金池挂在该供应商下。')
ON CONFLICT (name) WHERE archived_at IS NULL DO NOTHING;

WITH default_supplier AS (
    SELECT id
    FROM upstream_suppliers
    WHERE name = '未归类供应商'
      AND archived_at IS NULL
    ORDER BY id
    LIMIT 1
),
account_cost AS (
    SELECT a.id AS account_id,
           a.name AS account_name,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_reference_fx_rate') = 'number'
                    AND (a.extra->>'upstream_reference_fx_rate')::numeric > 0
               THEN (a.extra->>'upstream_reference_fx_rate')::numeric
               ELSE 7
           END AS reference_fx_rate,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_recharge_cny_per_usd') = 'number'
                    AND (a.extra->>'upstream_recharge_cny_per_usd')::numeric > 0
               THEN (a.extra->>'upstream_recharge_cny_per_usd')::numeric
               ELSE NULL
           END AS configured_effective_cny_per_usd,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_group_multiplier') = 'number'
                    AND (a.extra->>'upstream_group_multiplier')::numeric > 0
               THEN (a.extra->>'upstream_group_multiplier')::numeric
               ELSE 1
           END AS default_multiplier,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_cost_model_families') = 'array'
               THEN a.extra->'upstream_cost_model_families'
               ELSE '[]'::jsonb
           END AS model_family_multipliers
    FROM accounts a
    WHERE a.deleted_at IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM upstream_account_cost_bindings b
          WHERE b.account_id = a.id
            AND b.status = 'active'
      )
)
INSERT INTO upstream_cost_pools (
    supplier_id,
    name,
    reference_fx_rate,
    current_effective_cny_per_usd,
    cost_method,
    note
)
SELECT ds.id,
       '账号默认资金池 #' || ac.account_id || ': ' || COALESCE(NULLIF(ac.account_name, ''), '未命名账号'),
       ac.reference_fx_rate,
       COALESCE(ac.configured_effective_cny_per_usd, ac.reference_fx_rate),
       'latest',
       '阶段 1 等价迁移自动创建，来源账号 ID: ' || ac.account_id
FROM account_cost ac
CROSS JOIN default_supplier ds
WHERE NOT EXISTS (
    SELECT 1
    FROM upstream_cost_pools p
    WHERE p.name = '账号默认资金池 #' || ac.account_id || ': ' || COALESCE(NULLIF(ac.account_name, ''), '未命名账号')
);

WITH account_cost AS (
    SELECT a.id AS account_id,
           a.name AS account_name,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_group_multiplier') = 'number'
                    AND (a.extra->>'upstream_group_multiplier')::numeric > 0
               THEN (a.extra->>'upstream_group_multiplier')::numeric
               ELSE 1
           END AS default_multiplier,
           CASE
               WHEN jsonb_typeof(a.extra->'upstream_cost_model_families') = 'array'
               THEN a.extra->'upstream_cost_model_families'
               ELSE '[]'::jsonb
           END AS model_family_multipliers
    FROM accounts a
    WHERE a.deleted_at IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM upstream_account_cost_bindings b
          WHERE b.account_id = a.id
            AND b.status = 'active'
      )
)
INSERT INTO upstream_account_cost_bindings (
    account_id,
    cost_pool_id,
    default_multiplier,
    model_family_multipliers,
    note
)
SELECT ac.account_id,
       p.id,
       ac.default_multiplier,
       ac.model_family_multipliers,
       '阶段 1 等价迁移自动创建。'
FROM account_cost ac
JOIN upstream_cost_pools p
  ON p.name = '账号默认资金池 #' || ac.account_id || ': ' || COALESCE(NULLIF(ac.account_name, ''), '未命名账号')
WHERE NOT EXISTS (
    SELECT 1
    FROM upstream_account_cost_bindings b
    WHERE b.account_id = ac.account_id
      AND b.status = 'active'
);

UPDATE upstream_recharge_records r
SET cost_pool_id = b.cost_pool_id,
    source_account_id_snapshot = COALESCE(r.source_account_id_snapshot, r.account_id),
    source = COALESCE(NULLIF(r.source, ''), 'legacy_account'),
    updated_at = NOW()
FROM upstream_account_cost_bindings b
WHERE r.account_id = b.account_id
  AND b.status = 'active'
  AND r.cost_pool_id IS NULL;

WITH pool_cost AS (
    SELECT p.id AS cost_pool_id,
           COALESCE(
               latest.effective_cny_per_usd,
               weighted.weighted_effective_cny_per_usd,
               p.current_effective_cny_per_usd,
               p.reference_fx_rate,
               7
           ) AS effective_cny_per_usd,
           p.reference_fx_rate,
           latest.record_id AS source_record_id,
           CASE
               WHEN latest.effective_cny_per_usd IS NOT NULL THEN 'latest'
               WHEN weighted.weighted_effective_cny_per_usd IS NOT NULL THEN 'weighted'
               ELSE 'manual'
           END AS calculation_method
    FROM upstream_cost_pools p
    LEFT JOIN LATERAL (
        SELECT r.id AS record_id,
               r.effective_cny_per_usd
        FROM upstream_recharge_records r
        WHERE r.cost_pool_id = p.id
          AND r.deleted_at IS NULL
          AND r.voided_at IS NULL
          AND r.type IN ('recharge', 'bonus')
          AND r.effective_cny_per_usd IS NOT NULL
        ORDER BY r.recorded_at DESC, r.id DESC
        LIMIT 1
    ) latest ON true
    LEFT JOIN LATERAL (
        SELECT (SUM(r.paid_amount) / NULLIF(SUM(r.received_credit_amount), 0)) AS weighted_effective_cny_per_usd
        FROM upstream_recharge_records r
        WHERE r.cost_pool_id = p.id
          AND r.deleted_at IS NULL
          AND r.voided_at IS NULL
          AND r.type IN ('recharge', 'bonus')
          AND r.paid_amount > 0
          AND r.received_credit_amount > 0
    ) weighted ON true
    WHERE NOT EXISTS (
        SELECT 1
        FROM upstream_cost_snapshots s
        WHERE s.cost_pool_id = p.id
          AND s.valid_to IS NULL
    )
),
inserted AS (
    INSERT INTO upstream_cost_snapshots (
        cost_pool_id,
        effective_cny_per_usd,
        reference_fx_rate,
        calculation_method,
        source_record_id,
        note
    )
    SELECT cost_pool_id,
           effective_cny_per_usd,
           reference_fx_rate,
           calculation_method,
           source_record_id,
           '阶段 1 等价迁移生成的初始成本快照。'
    FROM pool_cost
    WHERE effective_cny_per_usd > 0
    RETURNING id, cost_pool_id, effective_cny_per_usd
)
UPDATE upstream_cost_pools p
SET current_snapshot_id = inserted.id,
    current_effective_cny_per_usd = inserted.effective_cny_per_usd,
    updated_at = NOW()
FROM inserted
WHERE p.id = inserted.cost_pool_id;

UPDATE upstream_cost_pools p
SET current_snapshot_id = s.id,
    current_effective_cny_per_usd = s.effective_cny_per_usd,
    updated_at = NOW()
FROM upstream_cost_snapshots s
WHERE s.cost_pool_id = p.id
  AND s.valid_to IS NULL
  AND (p.current_snapshot_id IS NULL OR p.current_effective_cny_per_usd IS NULL);

ALTER TABLE upstream_recharge_records
VALIDATE CONSTRAINT fk_upstream_recharge_records_cost_pool_id;

ALTER TABLE upstream_recharge_records
VALIDATE CONSTRAINT fk_upstream_recharge_records_merged_from_pool_id;
