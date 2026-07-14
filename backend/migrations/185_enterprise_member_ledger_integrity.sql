-- Close the remaining enterprise-member accounting integrity boundaries without
-- rewriting applied migrations. Migration 184 created the referenced unique
-- index before this transactional migration runs.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'enterprise_member_import_usage_baselines'::regclass
          AND conname = 'enterprise_member_import_usage_baselines_key_member_owner_fk'
    ) THEN
        ALTER TABLE enterprise_member_import_usage_baselines
            ADD CONSTRAINT enterprise_member_import_usage_baselines_key_member_owner_fk
            FOREIGN KEY (api_key_id, member_id, enterprise_user_id)
            REFERENCES api_keys(id, member_id, user_id)
            ON DELETE RESTRICT
            NOT VALID;
    END IF;
END;
$$;

ALTER TABLE enterprise_member_import_usage_baselines
    VALIDATE CONSTRAINT enterprise_member_import_usage_baselines_key_member_owner_fk;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'enterprise_member_import_jobs'::regclass
          AND conname = 'enterprise_member_import_jobs_policy_v2_activation_groups_check'
    ) THEN
        ALTER TABLE enterprise_member_import_jobs
            ADD CONSTRAINT enterprise_member_import_jobs_policy_v2_activation_groups_check
            CHECK (
                import_policy_version < 2
                OR NOT activate_members
                OR jsonb_array_length(default_group_ids) > 0
            )
            NOT VALID;
    END IF;
END;
$$;

ALTER TABLE enterprise_member_import_jobs
    VALIDATE CONSTRAINT enterprise_member_import_jobs_policy_v2_activation_groups_check;

CREATE OR REPLACE FUNCTION enterprise_member_budget_entry_reject_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'enterprise member budget entries cannot be deleted'
            USING ERRCODE = '23000';
    END IF;

    -- Idempotent no-op updates remain legal so recovery statements using
    -- ON CONFLICT DO UPDATE do not fail when the same evidence is replayed.
    IF NEW IS NOT DISTINCT FROM OLD THEN
        RETURN NEW;
    END IF;

    -- Usage entries may acquire their immutable request evidence after the
    -- accounting row has been created. No other field may change, and the
    -- linked usage log must describe the same member and budget request.
    IF OLD.kind = 'usage'
       AND NEW.kind = 'usage'
       AND OLD.usage_log_id IS NULL
       AND NEW.usage_log_id IS NOT NULL
       AND (to_jsonb(NEW) - 'usage_log_id') IS NOT DISTINCT FROM (to_jsonb(OLD) - 'usage_log_id')
       AND EXISTS (
            SELECT 1
            FROM usage_logs usage
            WHERE usage.id = NEW.usage_log_id
              AND usage.member_id = OLD.member_id
              AND OLD.request_id = usage.api_key_id::text || ':' || usage.request_id
       ) THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'enterprise member budget entries are immutable accounting facts'
        USING ERRCODE = '23000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS enterprise_member_budget_entry_immutable
    ON enterprise_member_budget_entries;
CREATE TRIGGER enterprise_member_budget_entry_immutable
    BEFORE UPDATE OR DELETE ON enterprise_member_budget_entries
    FOR EACH ROW EXECUTE FUNCTION enterprise_member_budget_entry_reject_mutation();

COMMENT ON FUNCTION enterprise_member_budget_entry_reject_mutation() IS
    'Rejects budget-ledger deletion and mutation while allowing a usage_log_id to be linked once to the matching usage fact.';
