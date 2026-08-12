package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const routingEligibilityRedisChannel = "routing:eligibility:events:v1"

type routingEligibilityRevisionRepository struct {
	db *sql.DB
}

func NewRoutingEligibilityRevisionRepository(db *sql.DB) service.RoutingEligibilityRevisionRepository {
	return &routingEligibilityRevisionRepository{db: db}
}

func (r *routingEligibilityRevisionRepository) ListAll(ctx context.Context) ([]service.RoutingEligibilityScopeRevision, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT scope_type, scope_id, revision
		FROM routing_eligibility_revisions
		ORDER BY scope_type, scope_id`)
	if err != nil {
		return nil, fmt.Errorf("list routing eligibility revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRoutingEligibilityRevisions(rows)
}

func (r *routingEligibilityRevisionRepository) ListForScopes(ctx context.Context, scopes []service.RoutingEligibilityScope) ([]service.RoutingEligibilityScopeRevision, error) {
	if len(scopes) == 0 {
		return nil, nil
	}
	types := make([]string, 0, len(scopes))
	ids := make([]int64, 0, len(scopes))
	for _, scope := range scopes {
		types = append(types, string(scope.Type))
		ids = append(ids, scope.ID)
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH requested AS (
			SELECT scope_type, scope_id
			FROM unnest($1::text[], $2::bigint[]) AS request(scope_type, scope_id)
		)
		SELECT revision.scope_type, revision.scope_id, revision.revision
		FROM requested
		JOIN routing_eligibility_revisions AS revision
		  ON revision.scope_type = requested.scope_type
		 AND revision.scope_id = requested.scope_id
		ORDER BY revision.scope_type, revision.scope_id`, pq.Array(types), pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("list routing eligibility revisions for scopes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRoutingEligibilityRevisions(rows)
}

func scanRoutingEligibilityRevisions(rows *sql.Rows) ([]service.RoutingEligibilityScopeRevision, error) {
	items := make([]service.RoutingEligibilityScopeRevision, 0)
	for rows.Next() {
		var (
			item     service.RoutingEligibilityScopeRevision
			revision int64
		)
		if err := rows.Scan(&item.Scope.Type, &item.Scope.ID, &revision); err != nil {
			return nil, fmt.Errorf("scan routing eligibility revision: %w", err)
		}
		if revision > 0 {
			item.Revision = uint64(revision)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routing eligibility revisions: %w", err)
	}
	return items, nil
}

func (r *routingEligibilityRevisionRepository) ListPendingEvents(ctx context.Context, limit int) ([]service.RoutingEligibilityEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_id, revision
		FROM routing_eligibility_outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list routing eligibility outbox: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]service.RoutingEligibilityEvent, 0, limit)
	for rows.Next() {
		var (
			event    service.RoutingEligibilityEvent
			revision int64
		)
		if err := rows.Scan(&event.ID, &event.Scope.Type, &event.Scope.ID, &revision); err != nil {
			return nil, fmt.Errorf("scan routing eligibility outbox: %w", err)
		}
		if revision > 0 {
			event.Revision = uint64(revision)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routing eligibility outbox: %w", err)
	}
	return events, nil
}

func (r *routingEligibilityRevisionRepository) MarkEventsPublished(ctx context.Context, eventIDs []int64) error {
	if len(eventIDs) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE routing_eligibility_outbox
		SET published_at = COALESCE(published_at, NOW())
		WHERE id = ANY($1)`, pq.Array(eventIDs))
	if err != nil {
		return fmt.Errorf("mark routing eligibility events published: %w", err)
	}
	return nil
}

func (r *routingEligibilityRevisionRepository) DeletePublishedEventsBefore(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	result, err := r.db.ExecContext(ctx, `
		WITH doomed AS (
			SELECT id
			FROM routing_eligibility_outbox
			WHERE published_at IS NOT NULL
			  AND published_at < $1
			ORDER BY id
			LIMIT $2
		)
		DELETE FROM routing_eligibility_outbox AS event
		USING doomed
		WHERE event.id = doomed.id`, before, limit)
	if err != nil {
		return 0, fmt.Errorf("delete published routing eligibility events: %w", err)
	}
	return result.RowsAffected()
}

func (r *routingEligibilityRevisionRepository) GetOutboxStatus(ctx context.Context, now time.Time) (service.RoutingEligibilityOutboxStatus, error) {
	var (
		status service.RoutingEligibilityOutboxStatus
		oldest sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(created_at)
		FROM routing_eligibility_outbox
		WHERE published_at IS NULL`).Scan(&status.PendingCount, &oldest)
	if err != nil {
		return service.RoutingEligibilityOutboxStatus{}, fmt.Errorf("get routing eligibility outbox status: %w", err)
	}
	if oldest.Valid {
		value := oldest.Time
		status.OldestPendingAt = &value
		if now.After(value) {
			status.OldestPendingAge = now.Sub(value)
		}
	}
	return status, nil
}

type routingEligibilityRedisEventBus struct {
	rdb *redis.Client
}

func NewRoutingEligibilityEventBus(rdb *redis.Client) service.RoutingEligibilityEventBus {
	return &routingEligibilityRedisEventBus{rdb: rdb}
}

func (b *routingEligibilityRedisEventBus) Publish(ctx context.Context, event service.RoutingEligibilityEvent) error {
	if b == nil || b.rdb == nil {
		return fmt.Errorf("routing eligibility event bus is not configured")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal routing eligibility event: %w", err)
	}
	if err := b.rdb.Publish(ctx, routingEligibilityRedisChannel, payload).Err(); err != nil {
		return fmt.Errorf("publish routing eligibility event: %w", err)
	}
	return nil
}

func (b *routingEligibilityRedisEventBus) Subscribe(ctx context.Context) (service.RoutingEligibilityEventSubscription, error) {
	if b == nil || b.rdb == nil {
		return nil, fmt.Errorf("routing eligibility event bus is not configured")
	}
	pubsub := b.rdb.Subscribe(ctx, routingEligibilityRedisChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("subscribe routing eligibility events: %w", err)
	}
	subscription := &routingEligibilityRedisSubscription{
		pubsub: pubsub,
		events: make(chan service.RoutingEligibilityEvent, 128),
		done:   make(chan struct{}),
	}
	go subscription.run(ctx)
	return subscription, nil
}

type routingEligibilityRedisSubscription struct {
	pubsub *redis.PubSub
	events chan service.RoutingEligibilityEvent
	done   chan struct{}
	once   sync.Once
}

func (s *routingEligibilityRedisSubscription) Events() <-chan service.RoutingEligibilityEvent {
	return s.events
}

func (s *routingEligibilityRedisSubscription) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.once.Do(func() {
		close(s.done)
		if s.pubsub != nil {
			err = s.pubsub.Close()
		}
	})
	return err
}

func (s *routingEligibilityRedisSubscription) run(ctx context.Context) {
	defer close(s.events)
	messages := s.pubsub.Channel()
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				return
			}
			var event service.RoutingEligibilityEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				continue
			}
			select {
			case s.events <- event:
			case <-ctx.Done():
				return
			case <-s.done:
				return
			}
		case <-ctx.Done():
			return
		case <-s.done:
			return
		}
	}
}

var _ service.RoutingEligibilityRevisionRepository = (*routingEligibilityRevisionRepository)(nil)
var _ service.RoutingEligibilityEventBus = (*routingEligibilityRedisEventBus)(nil)
