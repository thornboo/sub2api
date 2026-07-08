ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS disabled_reason VARCHAR(40) DEFAULT '';

UPDATE api_keys
SET disabled_reason = ''
WHERE disabled_reason IS NULL;

ALTER TABLE api_keys
  ALTER COLUMN disabled_reason SET DEFAULT '',
  ALTER COLUMN disabled_reason SET NOT NULL;
