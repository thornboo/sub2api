package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	defaultRoutingEligibilityReconcileInterval = 30 * time.Second
	defaultRoutingEligibilityPublishInterval   = time.Second
	defaultEnterpriseMemberRouteSnapshotTTL    = 2 * time.Minute
	routingEligibilityOutboxRetention          = 24 * time.Hour
	routingEligibilityOutboxCleanupInterval    = time.Hour
)

var ErrRoutingEligibilityRevisionUnavailable = errors.New("routing eligibility revision is unavailable")

// RoutingEligibilityEvent is the independent cluster propagation contract for
// route eligibility. The database revision row remains authoritative; events
// only reduce cross-instance convergence latency.
type RoutingEligibilityEvent struct {
	ID       int64                   `json:"id,omitempty"`
	Scope    RoutingEligibilityScope `json:"scope"`
	Revision uint64                  `json:"revision"`
}

type RoutingEligibilityOutboxStatus struct {
	PendingCount     int64         `json:"pending_count"`
	OldestPendingAt  *time.Time    `json:"oldest_pending_at,omitempty"`
	OldestPendingAge time.Duration `json:"oldest_pending_age,omitempty"`
}

type RoutingEligibilityRevisionRepository interface {
	ListAll(ctx context.Context) ([]RoutingEligibilityScopeRevision, error)
	ListForScopes(ctx context.Context, scopes []RoutingEligibilityScope) ([]RoutingEligibilityScopeRevision, error)
	ListPendingEvents(ctx context.Context, limit int) ([]RoutingEligibilityEvent, error)
	MarkEventsPublished(ctx context.Context, eventIDs []int64) error
	DeletePublishedEventsBefore(ctx context.Context, before time.Time, limit int) (int64, error)
	GetOutboxStatus(ctx context.Context, now time.Time) (RoutingEligibilityOutboxStatus, error)
}

type RoutingEligibilityEventSubscription interface {
	Events() <-chan RoutingEligibilityEvent
	Close() error
}

type RoutingEligibilityEventBus interface {
	Publish(ctx context.Context, event RoutingEligibilityEvent) error
	Subscribe(ctx context.Context) (RoutingEligibilityEventSubscription, error)
}

// RoutingEligibilityRuntime owns the three runtime layers used by model-aware
// admission: durable revisions, a process-local atomic mirror, and bounded LKG
// snapshots. It never treats the event bus as authority.
type RoutingEligibilityRuntime struct {
	repo      RoutingEligibilityRevisionRepository
	bus       RoutingEligibilityEventBus
	mirror    *RoutingEligibilityRevisionMirror
	snapshots *EnterpriseMemberRouteSnapshotStore

	reconcileInterval time.Duration
	publishInterval   time.Duration

	ready atomic.Bool
	// lastReconciledAt is advanced only by a successful full database
	// reconciliation. Pub/Sub events can invalidate known changes quickly, but
	// they cannot prove that no event was missed, so they must not extend the
	// bounded LKG authority window.
	lastReconciledAt atomic.Int64
	mirrorMaxAge     time.Duration
	lastCleanupAt    atomic.Int64

	mu                  sync.RWMutex
	invalidationHandler func([]RoutingEligibilityScope)

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewRoutingEligibilityRuntime(repo RoutingEligibilityRevisionRepository, bus RoutingEligibilityEventBus) *RoutingEligibilityRuntime {
	return NewRoutingEligibilityRuntimeWithIntervals(
		repo,
		bus,
		defaultRoutingEligibilityReconcileInterval,
		defaultRoutingEligibilityPublishInterval,
		defaultEnterpriseMemberRouteSnapshotTTL,
	)
}

func NewRoutingEligibilityRuntimeWithIntervals(
	repo RoutingEligibilityRevisionRepository,
	bus RoutingEligibilityEventBus,
	reconcileInterval time.Duration,
	publishInterval time.Duration,
	snapshotTTL time.Duration,
) *RoutingEligibilityRuntime {
	return &RoutingEligibilityRuntime{
		repo:              repo,
		bus:               bus,
		mirror:            NewRoutingEligibilityRevisionMirror(),
		snapshots:         NewEnterpriseMemberRouteSnapshotStore(snapshotTTL),
		reconcileInterval: reconcileInterval,
		publishInterval:   publishInterval,
		mirrorMaxAge:      snapshotTTL,
		stopCh:            make(chan struct{}),
	}
}

func (r *RoutingEligibilityRuntime) SetInvalidationHandler(handler func([]RoutingEligibilityScope)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.invalidationHandler = handler
	r.mu.Unlock()
}

// Start performs one synchronous reconciliation before enabling LKG. Failure
// does not block legacy/shadow traffic; it keeps the runtime unready and LKG
// fail-closed while background reconciliation retries.
func (r *RoutingEligibilityRuntime) Start() {
	if r == nil {
		return
	}
	r.startOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := r.Reconcile(ctx); err != nil {
			logger.LegacyPrintf("service.routing_eligibility", "startup reconciliation failed; LKG disabled: %v", err)
		}
		cancel()

		if r.reconcileInterval > 0 {
			r.wg.Add(1)
			go r.runReconciler()
		}
		if r.publishInterval > 0 && r.repo != nil && r.bus != nil {
			r.wg.Add(1)
			go r.runPublisher()
			r.wg.Add(1)
			go r.runSubscriber()
		}
	})
}

func (r *RoutingEligibilityRuntime) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		close(r.stopCh)
		r.wg.Wait()
	})
}

func (r *RoutingEligibilityRuntime) Ready() bool {
	return r != nil && r.mirrorFresh(time.Now())
}

func (r *RoutingEligibilityRuntime) Reconcile(ctx context.Context) error {
	if r == nil || r.repo == nil {
		return ErrRoutingEligibilityRevisionUnavailable
	}
	items, err := r.repo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list routing eligibility revisions: %w", err)
	}
	if len(items) == 0 {
		return ErrRoutingEligibilityRevisionUnavailable
	}
	r.applyRevisions(items)
	r.lastReconciledAt.Store(time.Now().UnixNano())
	r.ready.Store(true)
	return nil
}

// CurrentVersion performs an explicit database-authority read for diagnostics,
// reconciliation tests and non-hot-path callers. Request-time LKG restore uses
// the startup-reconciled mirror; outbox/PubSub and periodic reconciliation keep
// it current without coupling the fallback to the failed projection database.
func (r *RoutingEligibilityRuntime) CurrentVersion(ctx context.Context, scopes []RoutingEligibilityScope) (RoutingEligibilityVersion, error) {
	if r == nil || r.repo == nil {
		return RoutingEligibilityVersion{}, ErrRoutingEligibilityRevisionUnavailable
	}
	normalized := normalizeRoutingEligibilityScopes(scopes)
	if len(normalized) == 0 {
		return RoutingEligibilityVersion{}, ErrRoutingEligibilityRevisionUnavailable
	}
	items, err := r.repo.ListForScopes(ctx, normalized)
	if err != nil {
		return RoutingEligibilityVersion{}, fmt.Errorf("read routing eligibility revisions: %w", err)
	}
	byScope := make(map[RoutingEligibilityScope]uint64, len(items))
	for _, item := range items {
		scope, ok := normalizeRoutingEligibilityScope(item.Scope)
		if ok && item.Revision > byScope[scope] {
			byScope[scope] = item.Revision
		}
	}
	complete := make([]RoutingEligibilityScopeRevision, 0, len(normalized))
	for _, scope := range normalized {
		revision := byScope[scope]
		if revision == 0 {
			return RoutingEligibilityVersion{}, fmt.Errorf("%w: %s:%d", ErrRoutingEligibilityRevisionUnavailable, scope.Type, scope.ID)
		}
		complete = append(complete, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
	}
	r.applyRevisions(complete)
	return NewRoutingEligibilityVersion(complete), nil
}

func (r *RoutingEligibilityRuntime) MirroredVersion(scopes []RoutingEligibilityScope) (RoutingEligibilityVersion, bool) {
	if r == nil || !r.mirrorFresh(time.Now()) {
		return RoutingEligibilityVersion{}, false
	}
	normalized := normalizeRoutingEligibilityScopes(scopes)
	if len(normalized) == 0 {
		return RoutingEligibilityVersion{}, false
	}
	for _, scope := range normalized {
		if r.mirror.Revision(scope) == 0 {
			return RoutingEligibilityVersion{}, false
		}
	}
	return r.mirror.VersionFor(normalized), true
}

func (r *RoutingEligibilityRuntime) mirrorFresh(now time.Time) bool {
	if r == nil || !r.ready.Load() || r.mirrorMaxAge <= 0 {
		return false
	}
	last := r.lastReconciledAt.Load()
	if last <= 0 {
		return false
	}
	// A backwards wall-clock adjustment must not disable an otherwise recent
	// reconciliation. Forward movement beyond the configured LKG TTL fails
	// closed until a full authority read succeeds again.
	return !now.After(time.Unix(0, last).Add(r.mirrorMaxAge))
}

func (r *RoutingEligibilityRuntime) SnapshotStore() *EnterpriseMemberRouteSnapshotStore {
	if r == nil {
		return nil
	}
	return r.snapshots
}

func (r *RoutingEligibilityRuntime) OutboxStatus(ctx context.Context) (RoutingEligibilityOutboxStatus, error) {
	if r == nil || r.repo == nil {
		return RoutingEligibilityOutboxStatus{}, ErrRoutingEligibilityRevisionUnavailable
	}
	return r.repo.GetOutboxStatus(ctx, time.Now().UTC())
}

func (r *RoutingEligibilityRuntime) ApplyEvent(event RoutingEligibilityEvent) bool {
	if r == nil || event.Revision == 0 {
		return false
	}
	scope, ok := normalizeRoutingEligibilityScope(event.Scope)
	if !ok || !r.mirror.Apply(scope, event.Revision) {
		return false
	}
	r.invalidate([]RoutingEligibilityScope{scope})
	return true
}

func (r *RoutingEligibilityRuntime) applyRevisions(items []RoutingEligibilityScopeRevision) {
	changed := make([]RoutingEligibilityScope, 0, len(items))
	for _, item := range items {
		scope, ok := normalizeRoutingEligibilityScope(item.Scope)
		if !ok || item.Revision == 0 {
			continue
		}
		if r.mirror.Apply(scope, item.Revision) {
			changed = append(changed, scope)
		}
	}
	if len(changed) > 0 {
		r.invalidate(changed)
	}
}

func (r *RoutingEligibilityRuntime) invalidate(scopes []RoutingEligibilityScope) {
	scopes = normalizeRoutingEligibilityScopes(scopes)
	if len(scopes) == 0 {
		return
	}
	if r.snapshots != nil {
		r.snapshots.InvalidateScopes(scopes)
	}
	r.mu.RLock()
	handler := r.invalidationHandler
	r.mu.RUnlock()
	if handler != nil {
		handler(scopes)
	}
}

func (r *RoutingEligibilityRuntime) runReconciler() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := r.Reconcile(ctx); err != nil {
				logger.LegacyPrintf("service.routing_eligibility", "revision reconciliation failed: %v", err)
			}
			cancel()
		case <-r.stopCh:
			return
		}
	}
}

func (r *RoutingEligibilityRuntime) runPublisher() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.publishInterval)
	defer ticker.Stop()
	r.publishPending()
	for {
		select {
		case <-ticker.C:
			r.publishPending()
		case <-r.stopCh:
			return
		}
	}
}

func (r *RoutingEligibilityRuntime) publishPending() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := r.repo.ListPendingEvents(ctx, 200)
	if err != nil {
		logger.LegacyPrintf("service.routing_eligibility", "outbox poll failed: %v", err)
		return
	}
	published := make([]int64, 0, len(events))
	for _, event := range events {
		if err := r.bus.Publish(ctx, event); err != nil {
			logger.LegacyPrintf("service.routing_eligibility", "event publish failed: id=%d err=%v", event.ID, err)
			break
		}
		published = append(published, event.ID)
	}
	if len(published) > 0 {
		if err := r.repo.MarkEventsPublished(ctx, published); err != nil {
			logger.LegacyPrintf("service.routing_eligibility", "mark events published failed: %v", err)
		}
	}
	r.cleanupPublishedEvents(ctx, time.Now())
}

func (r *RoutingEligibilityRuntime) cleanupPublishedEvents(ctx context.Context, now time.Time) {
	last := r.lastCleanupAt.Load()
	if last > 0 && now.Before(time.Unix(0, last).Add(routingEligibilityOutboxCleanupInterval)) {
		return
	}
	if !r.lastCleanupAt.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	// Cleanup is deliberately conservative and recoverable from revision rows.
	if _, err := r.repo.DeletePublishedEventsBefore(ctx, now.Add(-routingEligibilityOutboxRetention), 1000); err != nil {
		r.lastCleanupAt.Store(0)
		logger.LegacyPrintf("service.routing_eligibility", "delete published outbox events failed: %v", err)
	}
}

func (r *RoutingEligibilityRuntime) runSubscriber() {
	defer r.wg.Done()
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}
		ctx, cancel := context.WithCancel(context.Background())
		subscription, err := r.bus.Subscribe(ctx)
		if err != nil {
			cancel()
			logger.LegacyPrintf("service.routing_eligibility", "event subscribe failed: %v", err)
			if !r.waitForRetry() {
				return
			}
			continue
		}
		closed := false
		for !closed {
			select {
			case event, ok := <-subscription.Events():
				if !ok {
					closed = true
					continue
				}
				r.ApplyEvent(event)
			case <-r.stopCh:
				_ = subscription.Close()
				cancel()
				return
			}
		}
		_ = subscription.Close()
		cancel()
		if !r.waitForRetry() {
			return
		}
	}
}

func (r *RoutingEligibilityRuntime) waitForRetry() bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.stopCh:
		return false
	}
}
