-- Supplier account wallets are independent of per-API-key quotas and cost records.
ALTER TABLE upstream_suppliers
    ADD COLUMN balance_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN balance_access_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN balance_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN upstream_suppliers.balance_access_token IS 'Encrypted account access token; never returned by the API';
