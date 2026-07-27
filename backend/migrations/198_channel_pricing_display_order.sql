-- Persist administrator-defined display order for channel model mapping and pricing.
-- These fields are presentation-only and do not affect runtime mapping or pricing precedence.

ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS model_mapping_order JSONB NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'channel_model_pricing'
          AND column_name = 'sort_order'
    ) THEN
        ALTER TABLE channel_model_pricing
            ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

        WITH ranked AS (
            SELECT
                id,
                ROW_NUMBER() OVER (
                    PARTITION BY channel_id, platform
                    ORDER BY id
                ) - 1 AS sort_order
            FROM channel_model_pricing
        )
        UPDATE channel_model_pricing AS pricing
        SET sort_order = ranked.sort_order
        FROM ranked
        WHERE pricing.id = ranked.id;
    END IF;
END
$$;

COMMENT ON COLUMN channels.model_mapping_order IS
    'Admin display order for channel model mapping keys, grouped by platform; does not affect matching precedence';

COMMENT ON COLUMN channel_model_pricing.sort_order IS
    'Admin display order within a channel platform; does not affect pricing match precedence';
