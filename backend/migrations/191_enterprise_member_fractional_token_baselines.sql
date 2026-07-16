-- External migration reports may contain aggregate token evidence with two
-- decimal places. Preserve those source facts without changing request-level
-- usage_logs, whose token counters remain integers.

ALTER TABLE enterprise_member_import_usage_baselines
    DROP CONSTRAINT IF EXISTS enterprise_member_import_usage_baselines_values_check;

ALTER TABLE enterprise_member_import_usage_baselines
    ALTER COLUMN total_tokens TYPE NUMERIC(21,2) USING total_tokens::NUMERIC(21,2),
    ALTER COLUMN input_tokens TYPE NUMERIC(21,2) USING input_tokens::NUMERIC(21,2),
    ALTER COLUMN output_tokens TYPE NUMERIC(21,2) USING output_tokens::NUMERIC(21,2),
    ALTER COLUMN cache_tokens TYPE NUMERIC(21,2) USING cache_tokens::NUMERIC(21,2),
    ALTER COLUMN cache_creation_tokens TYPE NUMERIC(21,2) USING cache_creation_tokens::NUMERIC(21,2),
    ALTER COLUMN cache_read_tokens TYPE NUMERIC(21,2) USING cache_read_tokens::NUMERIC(21,2);

ALTER TABLE enterprise_member_import_usage_baselines
    ADD CONSTRAINT enterprise_member_import_usage_baselines_values_check CHECK (
        source_row_number > 0
        AND billed_usd >= 0
        AND total_tokens >= 0
        AND total_tokens <= 9223372036854775807.99
        AND input_tokens >= 0
        AND input_tokens <= 9223372036854775807.99
        AND output_tokens >= 0
        AND output_tokens <= 9223372036854775807.99
        AND cache_tokens >= 0
        AND cache_tokens <= 9223372036854775807.99
        AND cache_creation_tokens >= 0
        AND cache_creation_tokens <= 9223372036854775807.99
        AND cache_read_tokens >= 0
        AND cache_read_tokens <= 9223372036854775807.99
    );

COMMENT ON COLUMN enterprise_member_import_usage_baselines.total_tokens IS
    'Immutable external aggregate token evidence with at most two decimal places.';
