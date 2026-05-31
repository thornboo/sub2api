ALTER TABLE account_api_keys
    ALTER COLUMN priority SET DEFAULT 1;

ALTER TABLE account_api_keys
    ALTER COLUMN model_restriction_mode SET DEFAULT 'whitelist';

UPDATE account_api_keys
SET model_restriction_mode = 'whitelist'
WHERE model_restriction_mode = '';
