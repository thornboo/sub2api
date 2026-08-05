//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRoutingEligibilityRevisionTriggersAreAtomicAndIgnoreTransientAccountChurn(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	var accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra)
		VALUES ('routing-eligibility-test', 'openai', 'api_key',
		        '{"model_mapping":{"public":"upstream"}}'::jsonb, '{}'::jsonb)
		RETURNING id`).Scan(&accountID))

	var initialRevision int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = 'account' AND scope_id = $1`, accountID).Scan(&initialRevision))
	require.Positive(t, initialRevision)

	var eventCount int
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM routing_eligibility_outbox
		WHERE scope_type = 'account' AND scope_id = $1 AND revision = $2`, accountID, initialRevision).Scan(&eventCount))
	require.Equal(t, 1, eventCount)

	_, err := tx.ExecContext(ctx, `UPDATE accounts SET last_used_at = NOW() WHERE id = $1`, accountID)
	require.NoError(t, err)
	var afterTransient int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = 'account' AND scope_id = $1`, accountID).Scan(&afterTransient))
	require.Equal(t, initialRevision, afterTransient)

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET credentials = jsonb_set(credentials, '{model_mapping}', '{"public":"next"}'::jsonb)
		WHERE id = $1`, accountID)
	require.NoError(t, err)
	var afterMapping int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = 'account' AND scope_id = $1`, accountID).Scan(&afterMapping))
	require.Greater(t, afterMapping, initialRevision)

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET extra = jsonb_set(extra, '{privacy_mode}', '"training_off"'::jsonb)
		WHERE id = $1`, accountID)
	require.NoError(t, err)
	var afterPrivacy int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = 'account' AND scope_id = $1`, accountID).Scan(&afterPrivacy))
	require.Greater(t, afterPrivacy, afterMapping)

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET extra = jsonb_set(extra, '{unrelated_runtime_note}', '"unchanged-eligibility"'::jsonb)
		WHERE id = $1`, accountID)
	require.NoError(t, err)
	var afterUnrelatedExtra int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = 'account' AND scope_id = $1`, accountID).Scan(&afterUnrelatedExtra))
	require.Equal(t, afterPrivacy, afterUnrelatedExtra)
}

func TestRoutingEligibilityRevisionCoversAllStableWriterScopesWithoutAPIKeyEnumeration(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO groups (name, platform) VALUES ('routing-writer-group', 'composite') RETURNING id`).Scan(&groupID))
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "group", groupID))

	var channelID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO channels (name, status, model_mapping)
		VALUES ('routing-writer-channel', 'active', '{"openai":{"alias":"upstream"}}'::jsonb)
		RETURNING id`).Scan(&channelID))
	_, err := tx.ExecContext(ctx, `INSERT INTO channel_groups (channel_id, group_id) VALUES ($1, $2)`, channelID, groupID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO channel_model_pricing (channel_id, platform, models)
		VALUES ($1, 'openai', '["alias"]'::jsonb)`, channelID)
	require.NoError(t, err)
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "channel", channelID))

	var accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra)
		VALUES ('routing-writer-account', 'openai', 'api_key',
		        '{"model_mapping":{"alias":"upstream"}}'::jsonb, '{}'::jsonb)
		RETURNING id`).Scan(&accountID))
	_, err = tx.ExecContext(ctx, `INSERT INTO account_groups (account_id, group_id) VALUES ($1, $2)`, accountID, groupID)
	require.NoError(t, err)
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "account", accountID))

	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_model_protocol_capabilities (
			account_id, upstream_model, protocol, override_state, observed_state
		) VALUES ($1, 'upstream', 'openai_responses', 'supported', 'unknown')`, accountID)
	require.NoError(t, err)
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "protocol", accountID))

	_, err = tx.ExecContext(ctx, `
		INSERT INTO composite_model_routes (
			group_id, public_model, match_type, target_platform, upstream_model, endpoint
		) VALUES ($1, 'alias', 'exact', 'openai', 'upstream', 'responses')`, groupID)
	require.NoError(t, err)
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "composite", groupID))

	var apiKeyCount int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE group_id = $1`, groupID).Scan(&apiKeyCount))
	require.Zero(t, apiKeyCount, "revision triggers must not depend on API key enumeration")
}

func TestRoutingEligibilityRevisionPublishesForGroupWithoutAPIKeys(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO groups (name, platform) VALUES ('routing-no-key-group', 'openai') RETURNING id`).Scan(&groupID))
	var apiKeyCount int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE group_id = $1`, groupID).Scan(&apiKeyCount))
	require.Zero(t, apiKeyCount)

	revision := routingEligibilityRevisionInTx(t, tx, "group", groupID)
	var eventCount int
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM routing_eligibility_outbox
		WHERE scope_type = 'group' AND scope_id = $1 AND revision = $2`, groupID, revision).Scan(&eventCount))
	require.Equal(t, 1, eventCount, "group eligibility changes must publish even before any API key exists")
}

func TestRoutingEligibilityRevisionDoesNotDependOnAPIKeyEnumeration(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO groups (name, platform) VALUES ('routing-no-enumeration-group', 'openai') RETURNING id`).Scan(&groupID))
	before := routingEligibilityRevisionInTx(t, tx, "group", groupID)

	_, err := tx.ExecContext(ctx, `UPDATE groups SET allow_live = TRUE WHERE id = $1`, groupID)
	require.NoError(t, err)
	after := routingEligibilityRevisionInTx(t, tx, "group", groupID)
	require.Greater(t, after, before)

	var apiKeyCount int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE group_id = $1`, groupID).Scan(&apiKeyCount))
	require.Zero(t, apiKeyCount, "revision propagation must be scoped by changed config rows, not API key fanout")
}

func TestRoutingEligibilityRevisionDoesNotSurviveRolledBackConfiguration(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO groups (name, platform) VALUES ('routing-rollback-group', 'openai') RETURNING id`).Scan(&groupID))
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "group", groupID))
	require.NoError(t, tx.Rollback())

	var revisionCount, eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM routing_eligibility_revisions
		WHERE scope_type = 'group' AND scope_id = $1`, groupID).Scan(&revisionCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM routing_eligibility_outbox
		WHERE scope_type = 'group' AND scope_id = $1`, groupID).Scan(&eventCount))
	require.Zero(t, revisionCount)
	require.Zero(t, eventCount)
}

func routingEligibilityRevisionInTx(t *testing.T, tx *sql.Tx, scopeType string, scopeID int64) int64 {
	t.Helper()
	var revision int64
	require.NoError(t, tx.QueryRowContext(context.Background(), `
		SELECT revision FROM routing_eligibility_revisions
		WHERE scope_type = $1 AND scope_id = $2`, scopeType, scopeID).Scan(&revision))
	return revision
}

func TestRoutingEligibilityRepositoryAndRedisBusRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	items, err := repo.ListForScopes(ctx, []service.RoutingEligibilityScope{
		{Type: service.RoutingEligibilityScopeChannel, ID: 0},
		{Type: service.RoutingEligibilityScopeAccount, ID: 0},
	})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Positive(t, items[0].Revision)
	require.Positive(t, items[1].Revision)

	bus := NewRoutingEligibilityEventBus(integrationRedis)
	subscription, err := bus.Subscribe(ctx)
	require.NoError(t, err)
	defer func() { _ = subscription.Close() }()
	event := service.RoutingEligibilityEvent{
		ID:       77,
		Scope:    service.RoutingEligibilityScope{Type: service.RoutingEligibilityScopeGroup, ID: 42},
		Revision: 9001,
	}
	require.NoError(t, bus.Publish(ctx, event))

	select {
	case received := <-subscription.Events():
		require.Equal(t, event, received)
	case <-ctx.Done():
		t.Fatal("timed out waiting for routing eligibility event")
	}
}

func TestRoutingEligibilityRevisionPropagatesAcrossServiceInstances(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	bus := NewRoutingEligibilityEventBus(integrationRedis)
	consumer := service.NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, time.Hour, time.Minute)
	consumer.Start()
	t.Cleanup(consumer.Stop)
	require.NoError(t, consumer.Reconcile(ctx))

	scope := service.RoutingEligibilityScope{Type: service.RoutingEligibilityScopeGroup, ID: time.Now().UnixNano() % 1_000_000_000}
	event := service.RoutingEligibilityEvent{ID: 1001, Scope: scope, Revision: uint64(time.Now().UnixNano())}

	require.Eventually(t, func() bool {
		_ = bus.Publish(ctx, event)
		version, ok := consumer.MirroredVersion([]service.RoutingEligibilityScope{scope})
		return ok && len(version.Items) == 1 && version.Items[0].Revision == event.Revision
	}, 5*time.Second, 25*time.Millisecond)
}

func TestRoutingEligibilityRevisionSubscriberRecoversAfterRedisRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	bus := NewRoutingEligibilityEventBus(integrationRedis)
	consumer := service.NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, time.Hour, time.Minute)
	consumer.Start()
	t.Cleanup(consumer.Stop)
	require.NoError(t, consumer.Reconcile(ctx))

	scope := service.RoutingEligibilityScope{Type: service.RoutingEligibilityScopeGroup, ID: time.Now().UnixNano()%1_000_000_000 + 1_000_000_000}
	first := service.RoutingEligibilityEvent{ID: 1101, Scope: scope, Revision: uint64(time.Now().UnixNano())}
	require.Eventually(t, func() bool {
		_ = bus.Publish(ctx, first)
		version, ok := consumer.MirroredVersion([]service.RoutingEligibilityScope{scope})
		return ok && len(version.Items) == 1 && version.Items[0].Revision == first.Revision
	}, 5*time.Second, 25*time.Millisecond)

	// Close the live Pub/Sub connection from Redis' side. The runtime must
	// resubscribe and then consume later events; Pub/Sub is still not authority,
	// so missing events are recovered by database reconciliation separately.
	_, _ = integrationRedis.Do(ctx, "CLIENT", "KILL", "TYPE", "pubsub").Result()
	second := service.RoutingEligibilityEvent{ID: 1102, Scope: scope, Revision: first.Revision + 1}
	require.Eventually(t, func() bool {
		_ = bus.Publish(ctx, second)
		version, ok := consumer.MirroredVersion([]service.RoutingEligibilityScope{scope})
		return ok && len(version.Items) == 1 && version.Items[0].Revision == second.Revision
	}, 8*time.Second, 100*time.Millisecond)
}

func TestRoutingEligibilityRevisionReconcilesMissedEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	runtime := service.NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)
	require.NoError(t, runtime.Reconcile(ctx))

	scopeID := time.Now().UnixNano()%1_000_000_000 + 2_000_000_000
	var revision int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT bump_routing_eligibility_revision('group', $1)`, scopeID).Scan(&revision))
	scope := service.RoutingEligibilityScope{Type: service.RoutingEligibilityScopeGroup, ID: scopeID}
	require.NoError(t, runtime.Reconcile(ctx))
	version, ok := runtime.MirroredVersion([]service.RoutingEligibilityScope{scope})
	require.True(t, ok)
	require.EqualValues(t, revision, version.Items[0].Revision)
}

func TestRoutingEligibilityRevisionIgnoresDuplicateAndOlderEvents(t *testing.T) {
	scope := service.RoutingEligibilityScope{Type: service.RoutingEligibilityScopeGroup, ID: time.Now().UnixNano()%1_000_000_000 + 3_000_000_000}
	runtime := service.NewRoutingEligibilityRuntimeWithIntervals(NewRoutingEligibilityRevisionRepository(integrationDB), nil, 0, 0, time.Minute)
	require.False(t, runtime.ApplyEvent(service.RoutingEligibilityEvent{Scope: scope, Revision: 0}))
	require.True(t, runtime.ApplyEvent(service.RoutingEligibilityEvent{Scope: scope, Revision: 7}))
	require.False(t, runtime.ApplyEvent(service.RoutingEligibilityEvent{Scope: scope, Revision: 7}))
	require.False(t, runtime.ApplyEvent(service.RoutingEligibilityEvent{Scope: scope, Revision: 6}))
	_, ok := runtime.MirroredVersion([]service.RoutingEligibilityScope{scope})
	require.False(t, ok, "Pub/Sub-only events must not create a fresh authority window without full reconciliation")
}

func TestRoutingEligibilityRevisionDoesNotPublishRolledBackChange(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO groups (name, platform) VALUES ('routing-rollback-publish-group', 'openai') RETURNING id`).Scan(&groupID))
	require.Positive(t, routingEligibilityRevisionInTx(t, tx, "group", groupID))
	require.NoError(t, tx.Rollback())

	var eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM routing_eligibility_outbox
		WHERE scope_type = 'group' AND scope_id = $1`, groupID).Scan(&eventCount))
	require.Zero(t, eventCount)
}

func TestRoutingEligibilityRevisionOutboxRetriesAfterPublishFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventScopeID := time.Now().UnixNano()%1_000_000_000 + 4_000_000_000
	revision := time.Now().UnixNano()
	var eventID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO routing_eligibility_outbox (scope_type, scope_id, revision)
		VALUES ('group', $1, $2)
		RETURNING id`, eventScopeID, revision).Scan(&eventID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM routing_eligibility_outbox WHERE id = $1`, eventID)
	})

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	bus := &routingEligibilityFailOnceBus{}
	runtime := service.NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, 20*time.Millisecond, time.Minute)
	runtime.Start()
	t.Cleanup(runtime.Stop)

	require.Eventually(t, func() bool {
		var publishedAt sql.NullTime
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT published_at FROM routing_eligibility_outbox WHERE id = $1`, eventID).Scan(&publishedAt))
		return publishedAt.Valid && bus.attempts.Load() >= 2
	}, 5*time.Second, 25*time.Millisecond)
}

func TestRoutingEligibilityRevisionOutboxStatusReportsPendingLag(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventScopeID := time.Now().UnixNano()%1_000_000_000 + 4_500_000_000
	oldest := time.Now().UTC().Add(-5 * time.Minute)
	var eventID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO routing_eligibility_outbox (scope_type, scope_id, revision, created_at)
		VALUES ('group', $1, $2, $3)
		RETURNING id`, eventScopeID, time.Now().UnixNano(), oldest).Scan(&eventID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM routing_eligibility_outbox WHERE id = $1`, eventID)
	})

	status, err := NewRoutingEligibilityRevisionRepository(integrationDB).GetOutboxStatus(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.GreaterOrEqual(t, status.PendingCount, int64(1))
	require.NotNil(t, status.OldestPendingAt)
	require.GreaterOrEqual(t, status.OldestPendingAge, 5*time.Minute)

	require.NoError(t, NewRoutingEligibilityRevisionRepository(integrationDB).MarkEventsPublished(ctx, []int64{eventID}))
	after, err := NewRoutingEligibilityRevisionRepository(integrationDB).GetOutboxStatus(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Less(t, after.PendingCount, status.PendingCount)
}

func TestRoutingEligibilityRevisionRestartCannotMatchOldSharedSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewRoutingEligibilityRevisionRepository(integrationDB)
	before := service.NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)
	require.NoError(t, before.Reconcile(ctx))
	require.NotNil(t, before.SnapshotStore())

	after := service.NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)
	require.NoError(t, after.Reconcile(ctx))
	require.Zero(t, after.SnapshotStore().Len(), "LKG snapshots are process-local and must not survive restart into a new runtime")
}

type routingEligibilityFailOnceBus struct {
	attempts atomic.Int64
}

func (b *routingEligibilityFailOnceBus) Publish(context.Context, service.RoutingEligibilityEvent) error {
	if b.attempts.Add(1) == 1 {
		return errors.New("redis unavailable")
	}
	return nil
}

func (b *routingEligibilityFailOnceBus) Subscribe(context.Context) (service.RoutingEligibilityEventSubscription, error) {
	return nil, errors.New("not used")
}
