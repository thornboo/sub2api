package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageTaskMemoryStore struct {
	task    *ImageTaskRecord
	ttl     time.Duration
	saveErr error
	getErr  error
}

// imageTaskWatchRetryStore simulates Redis WATCH discarding the first closure
// result because another writer advances the task to executing. The recovery
// decision must use only the snapshot from the successful retry.
type imageTaskWatchRetryStore struct {
	imageTaskMemoryStore
	retryOnce bool
}

type imageTaskBlockingRecoveryStore struct {
	imageTaskMemoryStore
	entered chan struct{}
	release chan struct{}
}

func (s *imageTaskBlockingRecoveryStore) ListRecoverable(context.Context, time.Time, int64) ([]*ImageTaskRecord, error) {
	select {
	case s.entered <- struct{}{}:
	default:
	}
	<-s.release
	return nil, nil
}

func (s *imageTaskWatchRetryStore) Update(ctx context.Context, id string, ttl time.Duration, mutate func(*ImageTaskRecord) error) error {
	if !s.retryOnce {
		return s.imageTaskMemoryStore.Update(ctx, id, ttl, mutate)
	}
	s.retryOnce = false
	first := *s.task
	if err := mutate(&first); err != nil {
		return err
	}
	// The first mutation is discarded, matching a failed WATCH transaction.
	// A concurrent writer has durably advanced the real task to executing.
	s.task.Phase = ImageTaskPhaseExecuting
	s.task.RecoveryOriginPhase = ""
	s.task.RecoverAfter = time.Now().Add(-time.Second).Unix()
	second := *s.task
	if err := mutate(&second); err != nil {
		return err
	}
	return s.Save(ctx, &second, ttl)
}

func (s *imageTaskMemoryStore) Update(_ context.Context, _ string, ttl time.Duration, mutate func(*ImageTaskRecord) error) error {
	if s.getErr != nil {
		return s.getErr
	}
	if s.task == nil {
		return ErrImageTaskNotFound
	}
	copy := *s.task
	if err := mutate(&copy); err != nil {
		return err
	}
	return s.Save(context.Background(), &copy, ttl)
}

func (s *imageTaskMemoryStore) ListRecoverable(_ context.Context, before time.Time, _ int64) ([]*ImageTaskRecord, error) {
	recoverable := s.task != nil && (s.task.Status == ImageTaskStatusProcessing ||
		s.task.Budget != nil && s.task.Budget.Status == ImageTaskBudgetStatusNeedsReview)
	if !recoverable || s.task.RecoverAfter <= 0 || s.task.RecoverAfter > before.Unix() {
		return nil, nil
	}
	copy := *s.task
	return []*ImageTaskRecord{&copy}, nil
}

type imageTaskBudgetRecoveryStub struct {
	receipt         *EnterpriseMemberBudgetReservation
	released        atomic.Bool
	markedAmbiguous atomic.Bool
	releaseFailures int
}

func (s *imageTaskBudgetRecoveryStub) GetReservationByRequestID(_ context.Context, requestID string) (*EnterpriseMemberBudgetReservation, error) {
	if s.receipt == nil || s.receipt.RequestID != requestID {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	copy := *s.receipt
	return &copy, nil
}

func (s *imageTaskBudgetRecoveryStub) GetReservationByTaskID(_ context.Context, taskID string) (*EnterpriseMemberBudgetReservation, error) {
	if s.receipt == nil || s.receipt.TaskID != taskID {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	copy := *s.receipt
	return &copy, nil
}

func (s *imageTaskBudgetRecoveryStub) ReleaseImageTaskReservationByRequestID(_ context.Context, requestID, taskID string) (*EnterpriseMemberBudgetReservation, error) {
	if s.receipt == nil || s.receipt.RequestID != requestID {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if s.receipt.TaskID != taskID {
		return nil, ErrEnterpriseMemberBudgetConflict
	}
	if s.releaseFailures > 0 {
		s.releaseFailures--
		return nil, errors.New("temporary budget repository failure")
	}
	s.released.Store(true)
	s.receipt.Status = "released"
	copy := *s.receipt
	return &copy, nil
}

func (s *imageTaskBudgetRecoveryStub) MarkImageTaskReservationAmbiguousByRequestID(_ context.Context, requestID, taskID, _ string) (*EnterpriseMemberBudgetReservation, error) {
	if s.receipt == nil || s.receipt.RequestID != requestID {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if s.receipt.TaskID != taskID {
		return nil, ErrEnterpriseMemberBudgetConflict
	}
	s.markedAmbiguous.Store(true)
	s.receipt.Status = "ambiguous"
	copy := *s.receipt
	return &copy, nil
}

func (s *imageTaskMemoryStore) Save(_ context.Context, task *ImageTaskRecord, ttl time.Duration) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	copy := *task
	s.task = &copy
	s.ttl = ttl
	return nil
}

func (s *imageTaskMemoryStore) Get(_ context.Context, _ string) (*ImageTaskRecord, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.task == nil {
		return nil, ErrImageTaskNotFound
	}
	copy := *s.task
	return &copy, nil
}

func TestImageTaskServiceLifecycleAndOwnership(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, 10*time.Minute)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}

	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: "9:client:test", MemberID: 12, HeldUSD: 4.5,
	})
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusProcessing, created.Status)
	require.Equal(t, "queued", created.Phase)
	require.Equal(t, ImageTaskBudgetStatusHeld, created.Budget.Status)
	require.Equal(t, 4.5, created.Budget.TaskHoldUSD)
	require.Contains(t, created.Budget.Message, "not settled usage")
	require.Equal(t, created.ID, created.TaskID)
	require.Equal(t, "image.generation.task", created.Object)
	require.Equal(t, time.Hour, store.ttl)
	require.Equal(t, owner.UserID, store.task.UserID)
	require.Equal(t, owner.APIKeyID, store.task.APIKeyID)
	require.Equal(t, "9:client:test", store.task.Budget.RequestID)

	store.task.ExpiresAt = 1
	require.NoError(t, svc.MarkExecuting(context.Background(), created.ID))
	require.Greater(t, store.task.ExpiresAt, time.Now().Unix(), "public expiry must track the refreshed Redis TTL")
	running, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, "running", running.Phase)
	actualGroupID := int64(19)
	require.NoError(t, svc.MarkFinalizingWithGroup(context.Background(), created.ID, &actualGroupID))
	require.Equal(t, actualGroupID, *store.task.Budget.GroupID)

	_, err = svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)

	result := json.RawMessage(`{"created":123,"data":[{"url":"https://example.test/image.png"}]}`)
	require.NoError(t, svc.CompleteWithBudgetStatus(context.Background(), created.ID, http.StatusOK, result, ImageTaskBudgetStatusSettled))

	completed, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, completed.Status)
	require.Equal(t, http.StatusOK, completed.HTTPStatus)
	require.Equal(t, "https://example.test/image.png", completed.ImageURL)
	require.JSONEq(t, string(result), string(completed.Result))
	require.NotNil(t, completed.CompletedAt)
	require.Equal(t, ImageTaskBudgetStatusSettled, completed.Budget.Status)
	require.Contains(t, completed.Budget.Message, "actual usage")
}

func TestImageTaskServiceRecoveryReleasesQueuedHold(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:queued", ReservedUSD: 6, Status: "reserved",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskID: "linked", TaskPhase: EnterpriseMemberAsyncTaskPhaseQueued,
	}}
	svc.ConfigureBudgetRecovery(recovery)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	recovery.receipt.TaskID = created.ID
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	recovered, err := svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.True(t, recovery.released.Load())
	require.False(t, recovery.markedAmbiguous.Load())
	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, ImageTaskBudgetStatusReleased, got.Budget.Status)
	require.Contains(t, string(got.Error), "not dispatched")
}

func TestImageTaskServiceRecoveryProtectsExecutingHoldAsAmbiguous(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:running", ReservedUSD: 6, Status: "reserved",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskID: "linked", TaskPhase: EnterpriseMemberAsyncTaskPhaseExecuting,
	}}
	svc.ConfigureBudgetRecovery(recovery)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	recovery.receipt.TaskID = created.ID
	require.NoError(t, svc.MarkExecuting(context.Background(), created.ID))
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	recovered, err := svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.False(t, recovery.released.Load())
	require.True(t, recovery.markedAmbiguous.Load())
	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, ImageTaskBudgetStatusNeedsReview, got.Budget.Status)
	require.Contains(t, string(got.Error), "pending reconciliation")

	recovery.receipt.Status = "settled"
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()
	recovered, err = svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	got, err = svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status, "lost result remains failed even after billing resolves")
	require.Equal(t, ImageTaskBudgetStatusSettled, got.Budget.Status)
}

func TestImageTaskServiceRecoveryPostgresFencePreventsReleaseAfterRedisExecutionStateLoss(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:redis-loss", ReservedUSD: 6, Status: "reserved",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskPhase: EnterpriseMemberAsyncTaskPhaseExecuting,
	}}
	svc.ConfigureBudgetRecovery(recovery)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	recovery.receipt.TaskID = created.ID
	// Simulate Redis losing an acknowledged queued -> executing transition
	// while PostgreSQL retains the pre-dispatch durability fence.
	store.task.Phase = ImageTaskPhaseQueued
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	recovered, err := svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.False(t, recovery.released.Load())
	require.True(t, recovery.markedAmbiguous.Load())
	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskBudgetStatusNeedsReview, got.Budget.Status)
}

func TestImageTaskServiceRecoveryRetryPreservesProofThatQueuedTaskWasNotDispatched(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	recovery := &imageTaskBudgetRecoveryStub{
		receipt: &EnterpriseMemberBudgetReservation{
			RequestID: "9:client:queued-retry", ReservedUSD: 6, Status: "reserved",
			ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskID: "linked", TaskPhase: EnterpriseMemberAsyncTaskPhaseQueued,
		},
		releaseFailures: 1,
	}
	svc.ConfigureBudgetRecovery(recovery)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	recovery.receipt.TaskID = created.ID
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	recovered, err := svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, recovered)
	require.Equal(t, ImageTaskPhaseQueued, store.task.RecoveryOriginPhase)
	require.False(t, recovery.markedAmbiguous.Load())

	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()
	recovered, err = svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.True(t, recovery.released.Load())
	require.False(t, recovery.markedAmbiguous.Load())
	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskBudgetStatusReleased, got.Budget.Status)
}

func TestImageTaskServiceRecoveryWatchRetryDoesNotCarryQueuedProofAcrossSnapshots(t *testing.T) {
	store := &imageTaskWatchRetryStore{retryOnce: true}
	svc := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:watch-retry", ReservedUSD: 6, Status: "reserved",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskPhase: EnterpriseMemberAsyncTaskPhaseQueued,
	}}
	svc.ConfigureBudgetRecovery(recovery)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.CreateWithBudget(context.Background(), owner, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	recovery.receipt.TaskID = created.ID
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	recovered, err := svc.RecoverStale(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.False(t, recovery.released.Load(), "queued proof from a discarded WATCH attempt must not release the hold")
	require.True(t, recovery.markedAmbiguous.Load())
}

func TestImageTaskServiceReturnsBudgetBackedTombstoneWhenRedisTaskIsLost(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskService(store)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:redis-key-lost", ReservedUSD: 6, Status: "ambiguous",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskID: "imgtask_lost",
		TaskPhase: EnterpriseMemberAsyncTaskPhaseExecuting, CreatedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	svc.ConfigureBudgetRecovery(recovery)

	got, err := svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9}, "imgtask_lost")
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusServiceUnavailable, got.HTTPStatus)
	require.Equal(t, ImageTaskBudgetStatusNeedsReview, got.Budget.Status)
	require.Contains(t, string(got.Error), "pending budget reconciliation")
	require.NotContains(t, string(got.Error), recovery.receipt.RequestID)

	_, err = svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, "imgtask_lost")
	require.ErrorIs(t, err, ErrImageTaskNotFound)
}

func TestImageTaskServiceStartsRecoveryWhenNewSubmissionsAreDisabled(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskService(store)
	recovery := &imageTaskBudgetRecoveryStub{receipt: &EnterpriseMemberBudgetReservation{
		RequestID: "9:client:disabled-uploader", ReservedUSD: 6, Status: "reserved",
		ReceiptKind: EnterpriseMemberReceiptKindAsyncImage, TaskPhase: EnterpriseMemberAsyncTaskPhaseQueued,
	}}
	svc.ConfigureBudgetRecovery(recovery)
	created, err := svc.CreateWithBudget(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9}, &ImageTaskBudgetLink{
		RequestID: recovery.receipt.RequestID, MemberID: 12, HeldUSD: 6,
	})
	require.NoError(t, err)
	require.False(t, svc.Enabled())
	require.True(t, svc.Available())
	recovery.receipt.TaskID = created.ID
	store.task.RecoverAfter = time.Now().Add(-time.Second).Unix()

	svc.Start()
	t.Cleanup(svc.Stop)
	require.Eventually(t, recovery.released.Load, time.Second, 10*time.Millisecond)
}

func TestImageTaskServiceStopWaitsForRecoveryLoopExit(t *testing.T) {
	store := &imageTaskBlockingRecoveryStore{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	svc := NewImageTaskService(store)
	svc.ConfigureBudgetRecovery(&imageTaskBudgetRecoveryStub{})
	svc.Start()
	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("recovery loop did not start")
	}

	stopped := make(chan struct{})
	go func() {
		svc.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("Stop returned while recovery was still using its stores")
	case <-time.After(50 * time.Millisecond):
	}
	close(store.release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after recovery loop exited")
	}
}

func TestImageTaskServiceInvalidResultBecomesFailed(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, json.RawMessage(`not-json`)))
	got, err := svc.Get(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2}, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadGateway, got.HTTPStatus)
	require.Contains(t, string(got.Error), "non-JSON")
}

func TestImageTaskServiceMapsStoreFailures(t *testing.T) {
	store := &imageTaskMemoryStore{saveErr: errors.New("redis down")}
	svc := NewImageTaskService(store)

	_, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
}
