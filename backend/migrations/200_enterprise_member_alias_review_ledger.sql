-- Administrator review ledger for enterprise-member model alias migration.
--
-- This table records human disposition for aliases observed during
-- shadow_published routing. It is intentionally not consulted by the request
-- planner and must never grant model publication on its own.

CREATE TABLE IF NOT EXISTS enterprise_member_model_alias_reviews (
    id BIGSERIAL PRIMARY KEY,
    public_model VARCHAR(255) NOT NULL,
    public_model_norm VARCHAR(255) NOT NULL,
    endpoint VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    final_group_id BIGINT,
    channel_id BIGINT,
    review_note TEXT NOT NULL DEFAULT '',
    validation_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    reviewed_by BIGINT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT enterprise_member_model_alias_reviews_status_check
        CHECK (status IN ('pending', 'registered', 'rejected_invalid', 'obsolete', 'needs_owner_action')),
    CONSTRAINT enterprise_member_model_alias_reviews_public_model_not_blank
        CHECK (BTRIM(public_model) <> '' AND BTRIM(public_model_norm) <> ''),
    CONSTRAINT enterprise_member_model_alias_reviews_no_control_chars
        CHECK (public_model !~ '[[:cntrl:]]' AND public_model_norm !~ '[[:cntrl:]]'),
    CONSTRAINT enterprise_member_model_alias_reviews_review_note_safe
        CHECK (char_length(review_note) <= 1000 AND review_note !~ '[[:cntrl:]]'),
    CONSTRAINT enterprise_member_model_alias_reviews_group_check
        CHECK (final_group_id IS NULL OR final_group_id > 0),
    CONSTRAINT enterprise_member_model_alias_reviews_channel_check
        CHECK (channel_id IS NULL OR channel_id > 0),
    CONSTRAINT enterprise_member_model_alias_reviews_unique
        UNIQUE (public_model_norm, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_model_alias_reviews_status
    ON enterprise_member_model_alias_reviews (status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_enterprise_member_model_alias_reviews_model
    ON enterprise_member_model_alias_reviews (public_model_norm, endpoint);

COMMENT ON TABLE enterprise_member_model_alias_reviews IS
    'Admin-only review ledger for enterprise-member shadow alias migration; review state is not a routing authority';
COMMENT ON COLUMN enterprise_member_model_alias_reviews.validation_evidence IS
    'Sanitized proof captured when registering: exact publication source and stable delivery projection summary only';
