ALTER TABLE upstream_suppliers
  ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE upstream_suppliers
SET is_system = TRUE,
    updated_at = NOW()
WHERE name = '未归类供应商'
  AND archived_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_upstream_suppliers_system
ON upstream_suppliers(is_system)
WHERE is_system = TRUE;
