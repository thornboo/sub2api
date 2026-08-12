package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoutingEligibilityRuntimeReconcilesAndInvalidatesOnlyNewerScopes(t *testing.T) {
	repo := newRoutingEligibilityRuntimeRepo(
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 7}, Revision: 10},
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 11},
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0}, Revision: 12},
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 0}, Revision: 13},
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeComposite, ID: 0}, Revision: 14},
	)
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)
	var invalidated []RoutingEligibilityScope
	runtime.SetInvalidationHandler(func(scopes []RoutingEligibilityScope) {
		invalidated = append(invalidated, scopes...)
	})

	require.NoError(t, runtime.Reconcile(context.Background()))
	require.True(t, runtime.Ready())
	require.ElementsMatch(t, enterpriseMemberRouteEligibilityScopes(7), invalidated)

	version, ok := runtime.MirroredVersion(enterpriseMemberRouteEligibilityScopes(7))
	require.True(t, ok)
	require.True(t, runtime.SnapshotStore().Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key: EnterpriseMemberRouteSnapshotKey{
			PublicModel: "model", Endpoint: "/v1/responses", InboundProtocol: ModelProtocolOpenAIResponses,
			Intent: EnterpriseMemberRouteIntentText, GroupID: 7, Eligibility: version,
		},
		Candidates: []EnterpriseMemberRouteSnapshotCandidate{{
			GroupID: 7, ReasonCode: string(EnterpriseMemberRouteReasonEligible), PublicModel: "model", UpstreamModel: "upstream-model",
			InboundProtocol: ModelProtocolOpenAIResponses, UpstreamProtocol: ModelProtocolOpenAIResponses, DeliveryMode: ModelDeliveryModeNative,
		}},
	}))

	require.False(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 7}, Revision: 9}))
	require.Equal(t, 1, runtime.SnapshotStore().Len())
	require.True(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 7}, Revision: 11}))
	require.Zero(t, runtime.SnapshotStore().Len())
	require.False(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 7}, Revision: 11}))
}

func TestRoutingEligibilityRuntimeCurrentVersionFailsClosedOnMissingOrUnreadableAuthority(t *testing.T) {
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 9}
	repo := newRoutingEligibilityRuntimeRepo()
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)

	_, err := runtime.CurrentVersion(context.Background(), []RoutingEligibilityScope{scope})
	require.ErrorIs(t, err, ErrRoutingEligibilityRevisionUnavailable)

	repo.err = errors.New("postgres unavailable")
	_, err = runtime.CurrentVersion(context.Background(), []RoutingEligibilityScope{scope})
	require.ErrorContains(t, err, "postgres unavailable")
	require.False(t, runtime.Ready())
}

func TestRoutingEligibilityRuntimeRefusesLKGAfterFullReconciliationBecomesStale(t *testing.T) {
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 9}
	runtime := NewRoutingEligibilityRuntimeWithIntervals(
		newRoutingEligibilityRuntimeRepo(RoutingEligibilityScopeRevision{Scope: scope, Revision: 12}),
		nil,
		0,
		0,
		time.Minute,
	)
	require.NoError(t, runtime.Reconcile(context.Background()))
	_, ok := runtime.MirroredVersion([]RoutingEligibilityScope{scope})
	require.True(t, ok)

	runtime.lastReconciledAt.Store(time.Now().Add(-time.Minute - time.Second).UnixNano())
	require.False(t, runtime.Ready())
	_, ok = runtime.MirroredVersion([]RoutingEligibilityScope{scope})
	require.False(t, ok, "Pub/Sub mirror state cannot outlive the bounded full-reconciliation authority window")

	require.NoError(t, runtime.Reconcile(context.Background()))
	require.True(t, runtime.Ready())
}

func TestRoutingEligibilityRuntimePublishesOutboxBeforeAcknowledging(t *testing.T) {
	repo := newRoutingEligibilityRuntimeRepo()
	repo.pending = []RoutingEligibilityEvent{
		{ID: 1, Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 3}, Revision: 20},
		{ID: 2, Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 4}, Revision: 21},
	}
	bus := &routingEligibilityRuntimeBusFake{failAt: 2}
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, 0, time.Minute)

	runtime.publishPending()

	require.Equal(t, []int64{1}, repo.published)
	require.Equal(t, []int64{1, 2}, bus.attempted)
}

func TestRoutingEligibilityRuntimeReportsPendingOutboxLagWithoutPurging(t *testing.T) {
	oldest := time.Now().Add(-3 * time.Minute)
	repo := newRoutingEligibilityRuntimeRepo()
	repo.outboxStatus = RoutingEligibilityOutboxStatus{PendingCount: 2, OldestPendingAt: &oldest}
	repo.pending = []RoutingEligibilityEvent{
		{ID: 1, Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 3}, Revision: 20},
		{ID: 2, Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 4}, Revision: 21},
	}
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, &routingEligibilityRuntimeBusFake{failAt: 1}, 0, 0, time.Minute)

	status, err := runtime.OutboxStatus(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, status.PendingCount)
	require.NotNil(t, status.OldestPendingAt)
	require.GreaterOrEqual(t, status.OldestPendingAge, 3*time.Minute)

	runtime.publishPending()
	require.Empty(t, repo.published, "failed publishes must leave pending outbox rows unpublished for retry")
}

func TestRoutingEligibilityRuntimeThrottlesPublishedOutboxCleanup(t *testing.T) {
	repo := newRoutingEligibilityRuntimeRepo()
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, &routingEligibilityRuntimeBusFake{}, 0, 0, time.Minute)

	runtime.publishPending()
	runtime.publishPending()

	require.Equal(t, 1, repo.cleanupCalls)
}

func TestRoutingEligibilityRevisionPropagatesAcrossServiceInstances(t *testing.T) {
	repo := newRoutingEligibilityRuntimeRepo(
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 11}, Revision: 1},
	)
	bus := newRoutingEligibilityRuntimeBusReplay()
	producer := NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, 0, time.Minute)
	consumer := NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, time.Hour, time.Minute)
	consumer.Start()
	t.Cleanup(consumer.Stop)

	require.NoError(t, consumer.Reconcile(context.Background()))
	require.Eventually(t, func() bool { return bus.subscribeCount() >= 1 }, time.Second, 10*time.Millisecond)
	producer.ApplyEvent(RoutingEligibilityEvent{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 11}, Revision: 2})
	require.NoError(t, bus.Publish(context.Background(), RoutingEligibilityEvent{
		ID:       10,
		Scope:    RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 11},
		Revision: 2,
	}))

	require.Eventually(t, func() bool {
		version, ok := consumer.MirroredVersion([]RoutingEligibilityScope{{Type: RoutingEligibilityScopeGroup, ID: 11}})
		return ok && len(version.Items) == 1 && version.Items[0].Revision == 2
	}, time.Second, 10*time.Millisecond)
}

func TestRoutingEligibilityRevisionSubscriberRecoversAfterRedisRestart(t *testing.T) {
	repo := newRoutingEligibilityRuntimeRepo(
		RoutingEligibilityScopeRevision{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 12}, Revision: 1},
	)
	bus := newRoutingEligibilityRuntimeBusReplay()
	consumer := NewRoutingEligibilityRuntimeWithIntervals(repo, bus, 0, time.Hour, time.Minute)
	consumer.Start()
	t.Cleanup(consumer.Stop)
	require.NoError(t, consumer.Reconcile(context.Background()))
	require.Eventually(t, func() bool { return bus.subscribeCount() >= 1 }, time.Second, 10*time.Millisecond)

	bus.closeSubscriptions()
	require.Eventually(t, func() bool { return bus.subscribeCount() >= 2 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, bus.Publish(context.Background(), RoutingEligibilityEvent{
		ID:       20,
		Scope:    RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 12},
		Revision: 3,
	}))

	require.Eventually(t, func() bool {
		version, ok := consumer.MirroredVersion([]RoutingEligibilityScope{{Type: RoutingEligibilityScopeGroup, ID: 12}})
		return ok && len(version.Items) == 1 && version.Items[0].Revision == 3
	}, 2*time.Second, 10*time.Millisecond)
}

func TestRoutingEligibilityRevisionReconcilesMissedEvents(t *testing.T) {
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 13}
	repo := newRoutingEligibilityRuntimeRepo(RoutingEligibilityScopeRevision{Scope: scope, Revision: 1})
	runtime := NewRoutingEligibilityRuntimeWithIntervals(repo, nil, 0, 0, time.Minute)
	require.NoError(t, runtime.Reconcile(context.Background()))

	repo.set(scope, 5)
	require.NoError(t, runtime.Reconcile(context.Background()))
	version, ok := runtime.MirroredVersion([]RoutingEligibilityScope{scope})
	require.True(t, ok)
	require.EqualValues(t, 5, version.Items[0].Revision)
}

func TestRoutingEligibilityRevisionIgnoresDuplicateAndOlderEventsAtRuntime(t *testing.T) {
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 14}
	runtime := NewRoutingEligibilityRuntimeWithIntervals(
		newRoutingEligibilityRuntimeRepo(RoutingEligibilityScopeRevision{Scope: scope, Revision: 10}),
		nil,
		0,
		0,
		time.Minute,
	)
	require.NoError(t, runtime.Reconcile(context.Background()))
	invalidations := 0
	runtime.SetInvalidationHandler(func([]RoutingEligibilityScope) { invalidations++ })

	require.False(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: scope, Revision: 10}))
	require.False(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: scope, Revision: 9}))
	require.True(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: scope, Revision: 11}))
	require.False(t, runtime.ApplyEvent(RoutingEligibilityEvent{Scope: scope, Revision: 11}))
	require.Equal(t, 1, invalidations)
}

func TestRoutingEligibilityRevisionRestartCannotMatchOldSharedSnapshot(t *testing.T) {
	before := NewRoutingEligibilityRuntimeWithIntervals(
		newRoutingEligibilityRuntimeRepo(fullRoutingEligibilityScopeRevisionsForTest(15, 1)...),
		nil,
		0,
		0,
		time.Minute,
	)
	require.NoError(t, before.Reconcile(context.Background()))
	version, ok := before.MirroredVersion(enterpriseMemberRouteEligibilityScopes(15))
	require.True(t, ok)
	require.True(t, before.SnapshotStore().Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key: EnterpriseMemberRouteSnapshotKey{
			PublicModel: "model", Endpoint: "/v1/responses", InboundProtocol: ModelProtocolOpenAIResponses,
			Intent: EnterpriseMemberRouteIntentText, GroupID: 15, Eligibility: version,
		},
		Candidates: []EnterpriseMemberRouteSnapshotCandidate{{
			GroupID: 15, ReasonCode: string(EnterpriseMemberRouteReasonEligible), PublicModel: "model", UpstreamModel: "model",
			InboundProtocol: ModelProtocolOpenAIResponses, UpstreamProtocol: ModelProtocolOpenAIResponses, DeliveryMode: ModelDeliveryModeNative,
		}},
	}))

	after := NewRoutingEligibilityRuntimeWithIntervals(
		newRoutingEligibilityRuntimeRepo(fullRoutingEligibilityScopeRevisionsForTest(15, 2)...),
		nil,
		0,
		0,
		time.Minute,
	)
	require.NoError(t, after.Reconcile(context.Background()))
	afterVersion, ok := after.MirroredVersion(enterpriseMemberRouteEligibilityScopes(15))
	require.True(t, ok)
	_, restored := after.SnapshotStore().Restore(EnterpriseMemberRouteSnapshotKey{
		PublicModel: "model", Endpoint: "/v1/responses", InboundProtocol: ModelProtocolOpenAIResponses,
		Intent: EnterpriseMemberRouteIntentText, GroupID: 15, Eligibility: afterVersion,
	}, []int64{15})
	require.False(t, restored, "a fresh runtime must not restore a previous process' in-memory LKG snapshot")
}

func fullRoutingEligibilityScopeRevisionsForTest(groupID int64, revision uint64) []RoutingEligibilityScopeRevision {
	items := make([]RoutingEligibilityScopeRevision, 0, len(enterpriseMemberRouteEligibilityScopes(groupID)))
	for _, scope := range enterpriseMemberRouteEligibilityScopes(groupID) {
		items = append(items, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
	}
	return items
}

type routingEligibilityRuntimeRepo struct {
	mu           sync.Mutex
	items        map[RoutingEligibilityScope]uint64
	pending      []RoutingEligibilityEvent
	published    []int64
	outboxStatus RoutingEligibilityOutboxStatus
	cleanupCalls int
	err          error
}

func newRoutingEligibilityRuntimeRepo(items ...RoutingEligibilityScopeRevision) *routingEligibilityRuntimeRepo {
	repo := &routingEligibilityRuntimeRepo{items: make(map[RoutingEligibilityScope]uint64)}
	for _, item := range items {
		repo.items[item.Scope] = item.Revision
	}
	return repo
}

func (r *routingEligibilityRuntimeRepo) ListAll(context.Context) ([]RoutingEligibilityScopeRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	result := make([]RoutingEligibilityScopeRevision, 0, len(r.items))
	for scope, revision := range r.items {
		result = append(result, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
	}
	return result, nil
}

func (r *routingEligibilityRuntimeRepo) set(scope RoutingEligibilityScope, revision uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[scope] = revision
}

func (r *routingEligibilityRuntimeRepo) ListForScopes(_ context.Context, scopes []RoutingEligibilityScope) ([]RoutingEligibilityScopeRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	result := make([]RoutingEligibilityScopeRevision, 0, len(scopes))
	for _, scope := range scopes {
		if revision := r.items[scope]; revision > 0 {
			result = append(result, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
		}
	}
	return result, nil
}

func (r *routingEligibilityRuntimeRepo) ListPendingEvents(context.Context, int) ([]RoutingEligibilityEvent, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]RoutingEligibilityEvent(nil), r.pending...), nil
}

func (r *routingEligibilityRuntimeRepo) MarkEventsPublished(_ context.Context, eventIDs []int64) error {
	r.published = append(r.published, eventIDs...)
	return r.err
}

func (r *routingEligibilityRuntimeRepo) DeletePublishedEventsBefore(context.Context, time.Time, int) (int64, error) {
	r.cleanupCalls++
	return 0, r.err
}

func (r *routingEligibilityRuntimeRepo) GetOutboxStatus(_ context.Context, now time.Time) (RoutingEligibilityOutboxStatus, error) {
	if r.err != nil {
		return RoutingEligibilityOutboxStatus{}, r.err
	}
	status := r.outboxStatus
	if status.OldestPendingAt != nil && now.After(*status.OldestPendingAt) {
		status.OldestPendingAge = now.Sub(*status.OldestPendingAt)
	}
	return status, nil
}

type routingEligibilityRuntimeBusFake struct {
	attempted []int64
	failAt    int64
}

func (b *routingEligibilityRuntimeBusFake) Publish(_ context.Context, event RoutingEligibilityEvent) error {
	b.attempted = append(b.attempted, event.ID)
	if event.ID == b.failAt {
		return errors.New("redis unavailable")
	}
	return nil
}

func (b *routingEligibilityRuntimeBusFake) Subscribe(context.Context) (RoutingEligibilityEventSubscription, error) {
	return nil, errors.New("not implemented")
}

type routingEligibilityRuntimeBusReplay struct {
	mu            sync.Mutex
	subscriptions []*routingEligibilityRuntimeSubscriptionFake
}

func newRoutingEligibilityRuntimeBusReplay() *routingEligibilityRuntimeBusReplay {
	return &routingEligibilityRuntimeBusReplay{}
}

func (b *routingEligibilityRuntimeBusReplay) Publish(_ context.Context, event RoutingEligibilityEvent) error {
	b.mu.Lock()
	subs := append([]*routingEligibilityRuntimeSubscriptionFake(nil), b.subscriptions...)
	b.mu.Unlock()
	for _, sub := range subs {
		sub.send(event)
	}
	return nil
}

func (b *routingEligibilityRuntimeBusReplay) Subscribe(context.Context) (RoutingEligibilityEventSubscription, error) {
	sub := &routingEligibilityRuntimeSubscriptionFake{events: make(chan RoutingEligibilityEvent, 8)}
	b.mu.Lock()
	b.subscriptions = append(b.subscriptions, sub)
	b.mu.Unlock()
	return sub, nil
}

func (b *routingEligibilityRuntimeBusReplay) closeSubscriptions() {
	b.mu.Lock()
	subs := append([]*routingEligibilityRuntimeSubscriptionFake(nil), b.subscriptions...)
	b.mu.Unlock()
	for _, sub := range subs {
		_ = sub.Close()
	}
}

func (b *routingEligibilityRuntimeBusReplay) subscribeCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subscriptions)
}

type routingEligibilityRuntimeSubscriptionFake struct {
	once   sync.Once
	events chan RoutingEligibilityEvent
}

func (s *routingEligibilityRuntimeSubscriptionFake) Events() <-chan RoutingEligibilityEvent {
	return s.events
}

func (s *routingEligibilityRuntimeSubscriptionFake) Close() error {
	s.once.Do(func() { close(s.events) })
	return nil
}

func (s *routingEligibilityRuntimeSubscriptionFake) send(event RoutingEligibilityEvent) {
	defer func() { _ = recover() }()
	s.events <- event
}
