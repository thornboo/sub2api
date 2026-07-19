package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestImageTaskStoreRoundTripAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_123",
		UserID:    7,
		APIKeyID:  9,
		Status:    service.ImageTaskStatusProcessing,
		CreatedAt: 100,
		ExpiresAt: 200,
	}

	require.NoError(t, store.Save(context.Background(), task, 24*time.Hour))
	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, task, got)
	require.Equal(t, 24*time.Hour, mr.TTL(imageTaskKey(task.ID)))
}

func TestImageTaskStoreMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)

	_, err := store.Get(context.Background(), "imgtask_missing")
	require.ErrorIs(t, err, service.ErrImageTaskNotFound)
}

func TestImageTaskStoreIndexesAndClaimsRecoverableTasks(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store, ok := NewImageTaskStore(rdb).(service.ImageTaskRecoveryStore)
	require.True(t, ok)
	now := time.Now().UTC()
	task := &service.ImageTaskRecord{
		ID: "imgtask_recoverable", UserID: 7, APIKeyID: 9,
		Status: service.ImageTaskStatusProcessing, Phase: service.ImageTaskPhaseQueued,
		RecoverAfter: now.Add(-time.Minute).Unix(), CreatedAt: now.Add(-time.Hour).Unix(),
		ExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	recoverable, err := store.ListRecoverable(context.Background(), now, 10)
	require.NoError(t, err)
	require.Len(t, recoverable, 1)
	require.Equal(t, task.ID, recoverable[0].ID)

	require.NoError(t, store.Update(context.Background(), task.ID, time.Hour, func(current *service.ImageTaskRecord) error {
		current.Phase = service.ImageTaskPhaseRecovering
		current.RecoverAfter = now.Add(time.Minute).Unix()
		return nil
	}))
	recoverable, err = store.ListRecoverable(context.Background(), now, 10)
	require.NoError(t, err)
	require.Empty(t, recoverable)

	require.NoError(t, store.Update(context.Background(), task.ID, time.Hour, func(current *service.ImageTaskRecord) error {
		current.Status = service.ImageTaskStatusFailed
		current.Phase = service.ImageTaskPhaseTerminal
		current.RecoverAfter = 0
		return nil
	}))
	score, err := rdb.ZScore(context.Background(), imageTaskRecoverableIndex, task.ID).Result()
	require.ErrorIs(t, err, redis.Nil)
	require.Zero(t, score)
}

func TestImageTaskStoreUpdatePreservesConcurrentLifecycleWinner(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store, ok := NewImageTaskStore(rdb).(service.ImageTaskRecoveryStore)
	require.True(t, ok)
	task := &service.ImageTaskRecord{ID: "imgtask_terminal", Status: service.ImageTaskStatusCompleted, Phase: service.ImageTaskPhaseTerminal}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	err := store.Update(context.Background(), task.ID, time.Hour, func(current *service.ImageTaskRecord) error {
		if current.Status != service.ImageTaskStatusProcessing {
			return service.ErrImageTaskStateConflict
		}
		return nil
	})
	require.ErrorIs(t, err, service.ErrImageTaskStateConflict)
}

func TestImageTaskStoreKeepsTerminalBudgetReviewIndexedUntilReceiptResolves(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store, ok := NewImageTaskStore(rdb).(service.ImageTaskRecoveryStore)
	require.True(t, ok)
	now := time.Now().UTC()
	task := &service.ImageTaskRecord{
		ID: "imgtask_budget_review", Status: service.ImageTaskStatusFailed,
		Phase: service.ImageTaskPhaseTerminal, RecoverAfter: now.Add(-time.Minute).Unix(),
		Budget: &service.ImageTaskBudgetRecord{
			RequestID: "9:client:review", HeldUSD: 6, Status: service.ImageTaskBudgetStatusNeedsReview,
		},
	}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	recoverable, err := store.ListRecoverable(context.Background(), now, 10)
	require.NoError(t, err)
	require.Len(t, recoverable, 1)

	require.NoError(t, store.Update(context.Background(), task.ID, time.Hour, func(current *service.ImageTaskRecord) error {
		current.Budget.Status = service.ImageTaskBudgetStatusSettled
		current.RecoverAfter = 0
		return nil
	}))
	_, err = rdb.ZScore(context.Background(), imageTaskRecoverableIndex, task.ID).Result()
	require.ErrorIs(t, err, redis.Nil)
}
