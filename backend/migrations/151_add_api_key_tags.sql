-- Add owner-managed API key tags before enabling tag-based batch maintenance.
ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE api_keys
SET tags = '[]'::jsonb
WHERE tags IS NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'api_keys_tags_json_array'
  ) THEN
    ALTER TABLE api_keys
      ADD CONSTRAINT api_keys_tags_json_array
      CHECK (jsonb_typeof(tags) = 'array') NOT VALID;
  END IF;
END $$;

ALTER TABLE api_keys
  VALIDATE CONSTRAINT api_keys_tags_json_array;
