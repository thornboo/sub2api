-- Add optional model-level time pricing to channel model pricing.
-- Account stats pricing intentionally does not get this column: those rules
-- describe upstream/accounting cost snapshots, not customer sale schedules.

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS time_pricing JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN channel_model_pricing.time_pricing IS
    'Optional token model sale schedule: timezone and non-overlapping HH:MM multiplier rules. Empty object means disabled.';
