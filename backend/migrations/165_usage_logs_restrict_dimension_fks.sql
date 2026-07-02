-- Preserve usage_logs as an immutable billing/evidence ledger.
--
-- 001_init.sql created usage_logs.user_id/api_key_id/account_id with
-- ON DELETE CASCADE. That is unsafe for ledger integrity: any future hard
-- delete of a mutable dimension row would remove historical usage evidence.
--
-- Replace those three foreign keys with ON DELETE RESTRICT. Constraints are
-- added NOT VALID so large existing ledgers are not scanned during deploy; they
-- still protect new writes and future parent-row deletes.

DO $$
DECLARE
    spec record;
    fk record;
BEGIN
    FOR spec IN
        SELECT *
        FROM (VALUES
            ('user_id', 'users', 'fk_usage_logs_user_id_restrict'),
            ('api_key_id', 'api_keys', 'fk_usage_logs_api_key_id_restrict'),
            ('account_id', 'accounts', 'fk_usage_logs_account_id_restrict')
        ) AS v(column_name, ref_table_name, constraint_name)
    LOOP
        FOR fk IN
            SELECT c.conname
            FROM pg_constraint c
            JOIN pg_class tbl ON tbl.oid = c.conrelid
            JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
            JOIN pg_class ref_tbl ON ref_tbl.oid = c.confrelid
            JOIN pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attnum = ANY(c.conkey)
            WHERE ns.nspname = 'public'
              AND c.contype = 'f'
              AND tbl.relname = 'usage_logs'
              AND attr.attname = spec.column_name
              AND ref_tbl.relname = spec.ref_table_name
        LOOP
            EXECUTE format('ALTER TABLE public.usage_logs DROP CONSTRAINT IF EXISTS %I', fk.conname);
        END LOOP;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint c
            JOIN pg_class tbl ON tbl.oid = c.conrelid
            JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
            WHERE ns.nspname = 'public'
              AND tbl.relname = 'usage_logs'
              AND c.conname = spec.constraint_name
        ) THEN
            EXECUTE format(
                'ALTER TABLE public.usage_logs ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES public.%I(id) ON DELETE RESTRICT NOT VALID',
                spec.constraint_name,
                spec.column_name,
                spec.ref_table_name
            );
        END IF;
    END LOOP;
END $$;
