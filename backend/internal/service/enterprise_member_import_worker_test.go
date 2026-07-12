//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type importLeaseRepo struct {
	importPreviewRepoCapture
	renew func(call int32) (bool, error)
	calls atomic.Int32
}

type importWorkerFailureRecord struct {
	workerID    string
	contextLive bool
}

type importBlockingWorkerRepo struct {
	importPreviewRepoCapture
	job           *EnterpriseMemberImportJob
	claimed       atomic.Bool
	commitStarted chan struct{}
	commitExited  chan struct{}
	failed        chan importWorkerFailureRecord
}

func newImportBlockingWorkerRepo() *importBlockingWorkerRepo {
	idempotencyHash := HashIdempotencyKey("worker-lifecycle")
	return &importBlockingWorkerRepo{
		job: &EnterpriseMemberImportJob{
			ID: 91, EnterpriseUserID: 7, TokenHash: hashEnterpriseMemberImportToken("worker-lifecycle-token"),
			Status: "processing", IdempotencyKeyHash: &idempotencyHash, SelectedRows: []int{1},
			Preview: EnterpriseMemberImportPreview{Rows: []EnterpriseMemberImportRow{{
				RowNumber: 1, MemberCode: "member-lifecycle", MemberName: "Lifecycle", Valid: true,
			}}},
		},
		commitStarted: make(chan struct{}),
		commitExited:  make(chan struct{}),
		failed:        make(chan importWorkerFailureRecord, 1),
	}
}

func (r *importBlockingWorkerRepo) ClaimNextCommitJob(context.Context, string, time.Duration) (*EnterpriseMemberImportJob, error) {
	if !r.claimed.CompareAndSwap(false, true) {
		return nil, ErrEnterpriseMemberImportQueueEmpty
	}
	copyJob := *r.job
	return &copyJob, nil
}

func (r *importBlockingWorkerRepo) Commit(ctx context.Context, _ *EnterpriseMemberImportJob, _ []EnterpriseMemberImportRow, _ map[int]string, _, _ string) (*EnterpriseMemberImportResult, error) {
	close(r.commitStarted)
	<-ctx.Done()
	close(r.commitExited)
	return nil, ctx.Err()
}

func (r *importBlockingWorkerRepo) MarkCommitFailed(ctx context.Context, _ int64, workerID, _, _ string) error {
	r.failed <- importWorkerFailureRecord{workerID: workerID, contextLive: ctx.Err() == nil}
	return nil
}

func (r *importLeaseRepo) RenewCommitLease(context.Context, int64, string) (bool, error) {
	call := r.calls.Add(1)
	if r.renew == nil {
		return true, nil
	}
	return r.renew(call)
}

func TestEnterpriseMemberImportWorkerOptionsKeepLeaseAliveBeyondProcessingWindow(t *testing.T) {
	opts := normalizeEnterpriseMemberImportWorkerOptions(EnterpriseMemberImportWorkerOptions{})
	require.Greater(t, opts.ProcessingTimeout, opts.LeaseTTL)
	require.Greater(t, opts.LeaseTTL, opts.HeartbeatInterval)
	require.Positive(t, opts.ClaimTimeout)
	require.Positive(t, opts.FailureWriteTimeout)
}

func TestEnterpriseMemberImportWorkerHeartbeatRenewsUntilStopped(t *testing.T) {
	repo := &importLeaseRepo{}
	worker := newImportHeartbeatTestWorker(repo, 80*time.Millisecond, 5*time.Millisecond)
	processCtx, cancelProcess := context.WithCancel(context.Background())
	defer cancelProcess()
	stop := make(chan struct{})
	done := make(chan struct{})
	go worker.runLeaseHeartbeat(processCtx, 41, stop, done, cancelProcess)

	require.Eventually(t, func() bool { return repo.calls.Load() >= 2 }, time.Second, 5*time.Millisecond)
	close(stop)
	<-done
	select {
	case <-processCtx.Done():
		t.Fatal("a healthy heartbeat must not cancel processing")
	default:
	}
}

func TestEnterpriseMemberImportWorkerHeartbeatCancelsImmediatelyWhenLeaseIsLost(t *testing.T) {
	repo := &importLeaseRepo{renew: func(int32) (bool, error) { return false, nil }}
	worker := newImportHeartbeatTestWorker(repo, 80*time.Millisecond, 5*time.Millisecond)
	processCtx, cancelProcess := context.WithCancel(context.Background())
	defer cancelProcess()
	done := make(chan struct{})
	go worker.runLeaseHeartbeat(processCtx, 42, make(chan struct{}), done, cancelProcess)

	require.Eventually(t, func() bool { return processCtx.Err() != nil }, time.Second, 5*time.Millisecond)
	<-done
	require.Equal(t, int32(1), repo.calls.Load())
}

func TestEnterpriseMemberImportWorkerHeartbeatRecoversAfterTransientRenewalError(t *testing.T) {
	repo := &importLeaseRepo{renew: func(call int32) (bool, error) {
		if call == 1 {
			return false, errors.New("temporary database failure")
		}
		return true, nil
	}}
	worker := newImportHeartbeatTestWorker(repo, 80*time.Millisecond, 5*time.Millisecond)
	processCtx, cancelProcess := context.WithCancel(context.Background())
	defer cancelProcess()
	stop := make(chan struct{})
	done := make(chan struct{})
	go worker.runLeaseHeartbeat(processCtx, 43, stop, done, cancelProcess)

	require.Eventually(t, func() bool { return repo.calls.Load() >= 2 }, time.Second, 5*time.Millisecond)
	select {
	case <-processCtx.Done():
		t.Fatal("one transient renewal error must not abandon a still-valid lease")
	default:
	}
	close(stop)
	<-done
}

func TestEnterpriseMemberImportWorkerHeartbeatCancelsAfterErrorsOutliveLease(t *testing.T) {
	repo := &importLeaseRepo{renew: func(int32) (bool, error) {
		return false, errors.New("database unavailable")
	}}
	worker := newImportHeartbeatTestWorker(repo, 20*time.Millisecond, 5*time.Millisecond)
	processCtx, cancelProcess := context.WithCancel(context.Background())
	defer cancelProcess()
	done := make(chan struct{})
	go worker.runLeaseHeartbeat(processCtx, 44, make(chan struct{}), done, cancelProcess)

	require.Eventually(t, func() bool { return processCtx.Err() != nil }, time.Second, 5*time.Millisecond)
	<-done
	require.GreaterOrEqual(t, repo.calls.Load(), int32(3))
}

func TestEnterpriseMemberImportWorkerStopCancelsActiveProcessingAndWaitsForExit(t *testing.T) {
	repo := newImportBlockingWorkerRepo()
	importService := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	worker := NewEnterpriseMemberImportWorkerWithOptions(repo, importService, EnterpriseMemberImportWorkerOptions{
		PollInterval: time.Millisecond, ClaimTimeout: 100 * time.Millisecond,
		LeaseTTL: 100 * time.Millisecond, HeartbeatInterval: 20 * time.Millisecond,
		ProcessingTimeout: time.Minute, FailureWriteTimeout: 100 * time.Millisecond,
	})
	worker.Start()
	require.Eventually(t, func() bool {
		select {
		case <-repo.commitStarted:
			return true
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		worker.Stop()
		close(stopped)
	}()
	require.Eventually(t, func() bool {
		select {
		case <-stopped:
			return true
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond, "Stop must wait until the active import and heartbeat goroutines exit")
	select {
	case <-repo.commitExited:
	default:
		t.Fatal("Stop returned before the active commit observed cancellation")
	}
	worker.Stop() // idempotent after the first complete shutdown
}

func TestEnterpriseMemberImportWorkerProcessingTimeoutUsesFreshFailureContext(t *testing.T) {
	repo := newImportBlockingWorkerRepo()
	importService := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	worker := NewEnterpriseMemberImportWorkerWithOptions(repo, importService, EnterpriseMemberImportWorkerOptions{
		PollInterval: time.Millisecond, ClaimTimeout: 100 * time.Millisecond,
		LeaseTTL: 100 * time.Millisecond, HeartbeatInterval: 20 * time.Millisecond,
		ProcessingTimeout: 25 * time.Millisecond, FailureWriteTimeout: 100 * time.Millisecond,
	})
	worker.Start()
	t.Cleanup(worker.Stop)

	var failure importWorkerFailureRecord
	require.Eventually(t, func() bool {
		select {
		case failure = <-repo.failed:
			return true
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond)
	require.True(t, failure.contextLive, "failure persistence must not reuse the expired processing context")
	require.Equal(t, worker.workerID, failure.workerID)
}

func newImportHeartbeatTestWorker(repo EnterpriseMemberImportRepository, leaseTTL, heartbeatInterval time.Duration) *EnterpriseMemberImportWorker {
	return &EnterpriseMemberImportWorker{
		repo:     repo,
		workerID: "heartbeat-test-worker",
		opts: normalizeEnterpriseMemberImportWorkerOptions(EnterpriseMemberImportWorkerOptions{
			LeaseTTL:          leaseTTL,
			HeartbeatInterval: heartbeatInterval,
		}),
	}
}
