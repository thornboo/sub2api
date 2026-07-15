-- Repair usage facts written by provider-specific recorders that settled an
-- enterprise member budget but omitted the member attribution on usage_logs.
--
-- The immutable member budget ledger is the positive attribution evidence.
-- Current API-key membership and audit timestamps alone cannot prove ownership
-- at request authentication time (an ordinary key may be adopted mid-request).
-- Historical member snapshots intentionally remain NULL because the current
-- member row cannot prove the name/code that was visible when the request ran.

UPDATE usage_logs AS usage
SET member_id = entry.member_id
FROM enterprise_member_budget_entries AS entry,
     api_keys AS key
WHERE usage.api_key_id = key.id
  AND usage.user_id = key.user_id
  AND usage.member_id IS NULL
  AND entry.kind = 'usage'
  AND entry.usage_log_id IS NULL
  AND entry.member_id = key.member_id
  AND entry.request_id = usage.api_key_id::text || ':' || usage.request_id;

-- Usage ledger rows are immutable except for this one-time evidence link,
-- which migration 185 explicitly permits when member and request identity match.
UPDATE enterprise_member_budget_entries AS entry
SET usage_log_id = usage.id
FROM usage_logs AS usage
WHERE entry.kind = 'usage'
  AND entry.usage_log_id IS NULL
  AND usage.member_id = entry.member_id
  AND entry.request_id = usage.api_key_id::text || ':' || usage.request_id;

COMMENT ON COLUMN usage_logs.member_id IS
    'Enterprise member attribution captured from the API key at request time; historical provider-specific omissions are backfilled only from matching immutable member budget evidence.';
