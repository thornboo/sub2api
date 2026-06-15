-- Normalize legacy API key disabled status spelling.
UPDATE api_keys
SET status = 'disabled'
WHERE status = 'inactive';
