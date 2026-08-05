-- Durable generation authority for enterprise-member model-aware routing.
--
-- Configuration facts and their revision/outbox records are changed by the
-- same database transaction through row triggers. Redis Pub/Sub is only the
-- fast propagation path; these tables remain the recovery authority.

CREATE SEQUENCE IF NOT EXISTS routing_eligibility_revision_seq;

CREATE TABLE IF NOT EXISTS routing_eligibility_revisions (
    scope_type VARCHAR(32) NOT NULL,
    scope_id BIGINT NOT NULL,
    revision BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope_type, scope_id),
    CONSTRAINT routing_eligibility_revisions_scope_type_check
        CHECK (scope_type IN ('channel', 'group', 'account', 'protocol', 'composite')),
    CONSTRAINT routing_eligibility_revisions_scope_id_check CHECK (scope_id >= 0),
    CONSTRAINT routing_eligibility_revisions_revision_check CHECK (revision > 0)
);

CREATE TABLE IF NOT EXISTS routing_eligibility_outbox (
    id BIGSERIAL PRIMARY KEY,
    scope_type VARCHAR(32) NOT NULL,
    scope_id BIGINT NOT NULL,
    revision BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CONSTRAINT routing_eligibility_outbox_scope_type_check
        CHECK (scope_type IN ('channel', 'group', 'account', 'protocol', 'composite')),
    CONSTRAINT routing_eligibility_outbox_scope_id_check CHECK (scope_id >= 0),
    CONSTRAINT routing_eligibility_outbox_revision_check CHECK (revision > 0),
    CONSTRAINT routing_eligibility_outbox_event_unique UNIQUE (scope_type, scope_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_routing_eligibility_outbox_pending
    ON routing_eligibility_outbox (id)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_routing_eligibility_outbox_published_at
    ON routing_eligibility_outbox (published_at)
    WHERE published_at IS NOT NULL;

CREATE OR REPLACE FUNCTION bump_routing_eligibility_revision(
    p_scope_type TEXT,
    p_scope_id BIGINT
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_revision BIGINT;
BEGIN
    IF p_scope_type NOT IN ('channel', 'group', 'account', 'protocol', 'composite') THEN
        RAISE EXCEPTION 'invalid routing eligibility scope type: %', p_scope_type;
    END IF;
    IF p_scope_id < 0 THEN
        RAISE EXCEPTION 'invalid routing eligibility scope id: %', p_scope_id;
    END IF;

    v_revision := nextval('routing_eligibility_revision_seq');
    INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision, updated_at)
    VALUES (p_scope_type, p_scope_id, v_revision, NOW())
    ON CONFLICT (scope_type, scope_id) DO UPDATE SET
        revision = EXCLUDED.revision,
        updated_at = EXCLUDED.updated_at;

    INSERT INTO routing_eligibility_outbox (scope_type, scope_id, revision)
    VALUES (p_scope_type, p_scope_id, v_revision)
    ON CONFLICT (scope_type, scope_id, revision) DO NOTHING;

    RETURN v_revision;
END;
$$;

CREATE OR REPLACE FUNCTION bump_routing_eligibility_scope_and_global(
    p_scope_type TEXT,
    p_scope_id BIGINT
) RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM bump_routing_eligibility_revision(p_scope_type, p_scope_id);
    IF p_scope_id <> 0 THEN
        PERFORM bump_routing_eligibility_revision(p_scope_type, 0);
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_channel()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.status IS NOT DISTINCT FROM NEW.status AND
       OLD.model_mapping IS NOT DISTINCT FROM NEW.model_mapping THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_id := OLD.id; ELSE v_id := NEW.id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('channel', v_id);
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_channels_insert_delete ON channels;
CREATE TRIGGER routing_eligibility_channels_insert_delete
AFTER INSERT OR DELETE ON channels
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_channel();

DROP TRIGGER IF EXISTS routing_eligibility_channels_update ON channels;
CREATE TRIGGER routing_eligibility_channels_update
AFTER UPDATE OF status, model_mapping ON channels
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_channel();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_channel_group()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND
	   OLD.channel_id IS NOT DISTINCT FROM NEW.channel_id AND
	   OLD.group_id IS NOT DISTINCT FROM NEW.group_id THEN
		RETURN NEW;
	END IF;
	IF TG_OP = 'INSERT' THEN
		PERFORM bump_routing_eligibility_scope_and_global('channel', NEW.channel_id);
		PERFORM bump_routing_eligibility_scope_and_global('group', NEW.group_id);
	ELSIF TG_OP = 'DELETE' THEN
		PERFORM bump_routing_eligibility_scope_and_global('channel', OLD.channel_id);
		PERFORM bump_routing_eligibility_scope_and_global('group', OLD.group_id);
	ELSE
		PERFORM bump_routing_eligibility_scope_and_global('channel', OLD.channel_id);
		IF NEW.channel_id IS DISTINCT FROM OLD.channel_id THEN
			PERFORM bump_routing_eligibility_scope_and_global('channel', NEW.channel_id);
		END IF;
		PERFORM bump_routing_eligibility_scope_and_global('group', OLD.group_id);
		IF NEW.group_id IS DISTINCT FROM OLD.group_id THEN
			PERFORM bump_routing_eligibility_scope_and_global('group', NEW.group_id);
		END IF;
	END IF;
	RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_channel_groups ON channel_groups;
CREATE TRIGGER routing_eligibility_channel_groups
AFTER INSERT OR UPDATE OF channel_id, group_id OR DELETE ON channel_groups
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_channel_group();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_channel_pricing()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_channel_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.channel_id IS NOT DISTINCT FROM NEW.channel_id AND
       OLD.platform IS NOT DISTINCT FROM NEW.platform AND
       OLD.models IS NOT DISTINCT FROM NEW.models THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_channel_id := OLD.channel_id; ELSE v_channel_id := NEW.channel_id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('channel', v_channel_id);
    IF TG_OP = 'UPDATE' AND OLD.channel_id IS DISTINCT FROM NEW.channel_id THEN
        PERFORM bump_routing_eligibility_scope_and_global('channel', OLD.channel_id);
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_channel_model_pricing_insert_delete ON channel_model_pricing;
CREATE TRIGGER routing_eligibility_channel_model_pricing_insert_delete
AFTER INSERT OR DELETE ON channel_model_pricing
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_channel_pricing();

DROP TRIGGER IF EXISTS routing_eligibility_channel_model_pricing_update ON channel_model_pricing;
CREATE TRIGGER routing_eligibility_channel_model_pricing_update
AFTER UPDATE OF channel_id, platform, models ON channel_model_pricing
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_channel_pricing();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_group()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.platform IS NOT DISTINCT FROM NEW.platform AND
       OLD.status IS NOT DISTINCT FROM NEW.status AND
       OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at AND
       OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation AND
       OLD.allow_batch_image_generation IS NOT DISTINCT FROM NEW.allow_batch_image_generation AND
       OLD.allow_messages_dispatch IS NOT DISTINCT FROM NEW.allow_messages_dispatch AND
       OLD.allow_live IS NOT DISTINCT FROM NEW.allow_live AND
       OLD.model_routing_enabled IS NOT DISTINCT FROM NEW.model_routing_enabled AND
       OLD.model_routing IS NOT DISTINCT FROM NEW.model_routing AND
       OLD.supported_model_scopes IS NOT DISTINCT FROM NEW.supported_model_scopes AND
       OLD.default_mapped_model IS NOT DISTINCT FROM NEW.default_mapped_model AND
       OLD.messages_dispatch_model_config IS NOT DISTINCT FROM NEW.messages_dispatch_model_config AND
       OLD.require_oauth_only IS NOT DISTINCT FROM NEW.require_oauth_only AND
       OLD.require_privacy_set IS NOT DISTINCT FROM NEW.require_privacy_set THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_id := OLD.id; ELSE v_id := NEW.id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('group', v_id);
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_groups_insert_delete ON groups;
CREATE TRIGGER routing_eligibility_groups_insert_delete
AFTER INSERT OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_group();

DROP TRIGGER IF EXISTS routing_eligibility_groups_update ON groups;
CREATE TRIGGER routing_eligibility_groups_update
AFTER UPDATE OF platform, status, deleted_at, allow_image_generation,
    allow_batch_image_generation, allow_messages_dispatch, allow_live,
    model_routing_enabled, model_routing, supported_model_scopes,
    default_mapped_model, messages_dispatch_model_config,
    require_oauth_only, require_privacy_set ON groups
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_group();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_account()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.platform IS NOT DISTINCT FROM NEW.platform AND
       OLD.type IS NOT DISTINCT FROM NEW.type AND
       OLD.status IS NOT DISTINCT FROM NEW.status AND
       OLD.schedulable IS NOT DISTINCT FROM NEW.schedulable AND
       OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at AND
       (OLD.credentials -> 'model_mapping') IS NOT DISTINCT FROM (NEW.credentials -> 'model_mapping') AND
       (OLD.credentials -> 'openai_capabilities') IS NOT DISTINCT FROM (NEW.credentials -> 'openai_capabilities') AND
       (OLD.extra -> 'openai_responses_mode') IS NOT DISTINCT FROM (NEW.extra -> 'openai_responses_mode') AND
       (OLD.extra -> 'openai_responses_supported') IS NOT DISTINCT FROM (NEW.extra -> 'openai_responses_supported') AND
       (OLD.extra -> 'openai_passthrough') IS NOT DISTINCT FROM (NEW.extra -> 'openai_passthrough') AND
       (OLD.extra -> 'openai_oauth_passthrough') IS NOT DISTINCT FROM (NEW.extra -> 'openai_oauth_passthrough') AND
       (OLD.extra -> 'privacy_mode') IS NOT DISTINCT FROM (NEW.extra -> 'privacy_mode') THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_id := OLD.id; ELSE v_id := NEW.id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('account', v_id);
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_accounts_insert_delete ON accounts;
CREATE TRIGGER routing_eligibility_accounts_insert_delete
AFTER INSERT OR DELETE ON accounts
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_account();

DROP TRIGGER IF EXISTS routing_eligibility_accounts_update ON accounts;
CREATE TRIGGER routing_eligibility_accounts_update
AFTER UPDATE OF platform, type, credentials, extra, status, schedulable, deleted_at ON accounts
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_account();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_account_group()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND
	   OLD.account_id IS NOT DISTINCT FROM NEW.account_id AND
	   OLD.group_id IS NOT DISTINCT FROM NEW.group_id THEN
		RETURN NEW;
	END IF;
	IF TG_OP = 'INSERT' THEN
		PERFORM bump_routing_eligibility_scope_and_global('account', NEW.account_id);
		PERFORM bump_routing_eligibility_scope_and_global('group', NEW.group_id);
	ELSIF TG_OP = 'DELETE' THEN
		PERFORM bump_routing_eligibility_scope_and_global('account', OLD.account_id);
		PERFORM bump_routing_eligibility_scope_and_global('group', OLD.group_id);
	ELSE
		PERFORM bump_routing_eligibility_scope_and_global('account', OLD.account_id);
		IF NEW.account_id IS DISTINCT FROM OLD.account_id THEN
			PERFORM bump_routing_eligibility_scope_and_global('account', NEW.account_id);
		END IF;
		PERFORM bump_routing_eligibility_scope_and_global('group', OLD.group_id);
		IF NEW.group_id IS DISTINCT FROM OLD.group_id THEN
			PERFORM bump_routing_eligibility_scope_and_global('group', NEW.group_id);
		END IF;
	END IF;
	RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_account_groups ON account_groups;
CREATE TRIGGER routing_eligibility_account_groups
AFTER INSERT OR UPDATE OF account_id, group_id OR DELETE ON account_groups
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_account_group();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_protocol()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_account_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.account_id IS NOT DISTINCT FROM NEW.account_id AND
       OLD.upstream_model IS NOT DISTINCT FROM NEW.upstream_model AND
       OLD.protocol IS NOT DISTINCT FROM NEW.protocol AND
       OLD.override_state IS NOT DISTINCT FROM NEW.override_state AND
       OLD.observed_state IS NOT DISTINCT FROM NEW.observed_state THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_account_id := OLD.account_id; ELSE v_account_id := NEW.account_id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('protocol', v_account_id);
    PERFORM bump_routing_eligibility_scope_and_global('account', v_account_id);
    IF TG_OP = 'UPDATE' AND OLD.account_id IS DISTINCT FROM NEW.account_id THEN
        PERFORM bump_routing_eligibility_scope_and_global('protocol', OLD.account_id);
        PERFORM bump_routing_eligibility_scope_and_global('account', OLD.account_id);
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_protocol_insert_delete ON account_model_protocol_capabilities;
CREATE TRIGGER routing_eligibility_protocol_insert_delete
AFTER INSERT OR DELETE ON account_model_protocol_capabilities
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_protocol();

DROP TRIGGER IF EXISTS routing_eligibility_protocol_update ON account_model_protocol_capabilities;
CREATE TRIGGER routing_eligibility_protocol_update
AFTER UPDATE OF account_id, upstream_model, protocol, override_state, observed_state
ON account_model_protocol_capabilities
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_protocol();

CREATE OR REPLACE FUNCTION trigger_routing_eligibility_composite()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_group_id BIGINT;
BEGIN
    IF TG_OP = 'UPDATE' AND
       OLD.group_id IS NOT DISTINCT FROM NEW.group_id AND
       OLD.public_model IS NOT DISTINCT FROM NEW.public_model AND
       OLD.match_type IS NOT DISTINCT FROM NEW.match_type AND
       OLD.target_platform IS NOT DISTINCT FROM NEW.target_platform AND
       OLD.upstream_model IS NOT DISTINCT FROM NEW.upstream_model AND
       OLD.endpoint IS NOT DISTINCT FROM NEW.endpoint AND
       OLD.priority IS NOT DISTINCT FROM NEW.priority AND
       OLD.enabled IS NOT DISTINCT FROM NEW.enabled AND
       OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN v_group_id := OLD.group_id; ELSE v_group_id := NEW.group_id; END IF;
    PERFORM bump_routing_eligibility_scope_and_global('composite', v_group_id);
    PERFORM bump_routing_eligibility_scope_and_global('group', v_group_id);
    IF TG_OP = 'UPDATE' AND OLD.group_id IS DISTINCT FROM NEW.group_id THEN
        PERFORM bump_routing_eligibility_scope_and_global('composite', OLD.group_id);
        PERFORM bump_routing_eligibility_scope_and_global('group', OLD.group_id);
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS routing_eligibility_composite_insert_delete ON composite_model_routes;
CREATE TRIGGER routing_eligibility_composite_insert_delete
AFTER INSERT OR DELETE ON composite_model_routes
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_composite();

DROP TRIGGER IF EXISTS routing_eligibility_composite_update ON composite_model_routes;
CREATE TRIGGER routing_eligibility_composite_update
AFTER UPDATE OF group_id, public_model, match_type, target_platform,
    upstream_model, endpoint, priority, enabled, deleted_at ON composite_model_routes
FOR EACH ROW EXECUTE FUNCTION trigger_routing_eligibility_composite();

-- Seed the durable mirror without emitting historical outbox events. Exact
-- scopes support targeted cache invalidation; scope_id=0 is the coarse safety
-- generation used when a failed projection cannot rediscover exact children.
INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT 'channel', id, nextval('routing_eligibility_revision_seq') FROM channels
ON CONFLICT (scope_type, scope_id) DO NOTHING;

INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT 'group', id, nextval('routing_eligibility_revision_seq') FROM groups
ON CONFLICT (scope_type, scope_id) DO NOTHING;

INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT 'account', id, nextval('routing_eligibility_revision_seq') FROM accounts
ON CONFLICT (scope_type, scope_id) DO NOTHING;

INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT 'protocol', account_id, nextval('routing_eligibility_revision_seq')
FROM account_model_protocol_capabilities
GROUP BY account_id
ON CONFLICT (scope_type, scope_id) DO NOTHING;

INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT 'composite', group_id, nextval('routing_eligibility_revision_seq')
FROM composite_model_routes
GROUP BY group_id
ON CONFLICT (scope_type, scope_id) DO NOTHING;

INSERT INTO routing_eligibility_revisions (scope_type, scope_id, revision)
SELECT scope_type, 0, nextval('routing_eligibility_revision_seq')
FROM unnest(ARRAY['channel', 'group', 'account', 'protocol', 'composite']) AS scope_type
ON CONFLICT (scope_type, scope_id) DO NOTHING;
