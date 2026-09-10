ALTER TABLE upstream_suppliers
    ADD COLUMN IF NOT EXISTS balance_revision BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS balance_next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TABLE IF NOT EXISTS upstream_supplier_balance_samples (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES upstream_suppliers(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL,
    balance NUMERIC(24,8) NOT NULL,
    unit TEXT NOT NULL,
    sampled_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_upstream_supplier_balance_samples_supplier_revision_time
    ON upstream_supplier_balance_samples (supplier_id, revision, sampled_at DESC);

CREATE INDEX IF NOT EXISTS idx_upstream_supplier_balance_samples_sampled_at
    ON upstream_supplier_balance_samples (sampled_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_upstream_suppliers_balance_next_poll_at
    ON upstream_suppliers (balance_next_poll_at, id)
    WHERE status = 'active';
