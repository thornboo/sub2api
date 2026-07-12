package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	defaultEnterpriseMemberImportPollInterval        = time.Second
	defaultEnterpriseMemberImportClaimTimeout        = 15 * time.Second
	defaultEnterpriseMemberImportLeaseTTL            = 3 * time.Minute
	defaultEnterpriseMemberImportProcessingTimeout   = 15 * time.Minute
	defaultEnterpriseMemberImportFailureWriteTimeout = 10 * time.Second
)

type EnterpriseMemberImportWorkerOptions struct {
	PollInterval        time.Duration
	ClaimTimeout        time.Duration
	LeaseTTL            time.Duration
	HeartbeatInterval   time.Duration
	ProcessingTimeout   time.Duration
	FailureWriteTimeout time.Duration
}

type EnterpriseMemberImportWorker struct {
	repo      EnterpriseMemberImportRepository
	service   *EnterpriseMemberImportService
	workerID  string
	opts      EnterpriseMemberImportWorkerOptions
	cancel    context.CancelFunc
	waitGroup sync.WaitGroup
}

func NewEnterpriseMemberImportWorker(repo EnterpriseMemberImportRepository, importService *EnterpriseMemberImportService) *EnterpriseMemberImportWorker {
	return NewEnterpriseMemberImportWorkerWithOptions(repo, importService, EnterpriseMemberImportWorkerOptions{})

}

func NewEnterpriseMemberImportWorkerWithOptions(repo EnterpriseMemberImportRepository, importService *EnterpriseMemberImportService, opts EnterpriseMemberImportWorkerOptions) *EnterpriseMemberImportWorker {
	return &EnterpriseMemberImportWorker{
		repo: repo, service: importService, workerID: "enterprise-import-" + uuid.NewString(), opts: normalizeEnterpriseMemberImportWorkerOptions(opts),
	}
}

func normalizeEnterpriseMemberImportWorkerOptions(opts EnterpriseMemberImportWorkerOptions) EnterpriseMemberImportWorkerOptions {
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultEnterpriseMemberImportPollInterval
	}
	if opts.ClaimTimeout <= 0 {
		opts.ClaimTimeout = defaultEnterpriseMemberImportClaimTimeout
	}
	if opts.LeaseTTL <= 0 {
		opts.LeaseTTL = defaultEnterpriseMemberImportLeaseTTL
	}
	if opts.HeartbeatInterval <= 0 || opts.HeartbeatInterval >= opts.LeaseTTL {
		opts.HeartbeatInterval = opts.LeaseTTL / 3
	}
	if opts.ProcessingTimeout <= 0 {
		opts.ProcessingTimeout = defaultEnterpriseMemberImportProcessingTimeout
	}
	if opts.FailureWriteTimeout <= 0 {
		opts.FailureWriteTimeout = defaultEnterpriseMemberImportFailureWriteTimeout
	}
	return opts
}

func (w *EnterpriseMemberImportWorker) Start() {
	if w == nil || w.repo == nil || w.service == nil || w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.waitGroup.Add(1)
	go func() {
		defer w.waitGroup.Done()
		ticker := time.NewTicker(w.opts.PollInterval)
		defer ticker.Stop()
		for {
			w.processAvailable(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (w *EnterpriseMemberImportWorker) processAvailable(ctx context.Context) {
	for {
		claimCtx, cancelClaim := context.WithTimeout(ctx, w.opts.ClaimTimeout)
		job, err := w.repo.ClaimNextCommitJob(claimCtx, w.workerID, w.opts.LeaseTTL)
		cancelClaim()
		if errors.Is(err, ErrEnterpriseMemberImportQueueEmpty) {
			return
		}
		if err != nil {
			return
		}

		processCtx, cancelProcess := context.WithTimeout(ctx, w.opts.ProcessingTimeout)
		heartbeatStop := make(chan struct{})
		heartbeatDone := make(chan struct{})
		go w.runLeaseHeartbeat(processCtx, job.ID, heartbeatStop, heartbeatDone, cancelProcess)

		_, processErr := w.service.ProcessClaimedJob(processCtx, job)
		close(heartbeatStop)
		<-heartbeatDone
		cancelProcess()
		if processErr != nil {
			code, summary := enterpriseMemberImportFailure(processErr)
			failureCtx, cancelFailure := context.WithTimeout(ctx, w.opts.FailureWriteTimeout)
			_ = w.repo.MarkCommitFailed(failureCtx, job.ID, w.workerID, code, summary)
			cancelFailure()
		}
	}
}

func (w *EnterpriseMemberImportWorker) runLeaseHeartbeat(
	ctx context.Context,
	jobID int64,
	stop <-chan struct{},
	done chan<- struct{},
	cancelProcess context.CancelFunc,
) {
	defer close(done)
	ticker := time.NewTicker(w.opts.HeartbeatInterval)
	defer ticker.Stop()
	leaseDeadline := time.Now().Add(w.opts.LeaseTTL)
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewed, err := w.repo.RenewCommitLease(ctx, jobID, w.workerID)
			if err != nil {
				RecordEnterpriseMemberImportLeaseRenewal(false, false)
				slog.Warn("enterprise member import lease renewal failed", "job_id", jobID, "error", err)
				if !time.Now().Before(leaseDeadline) {
					RecordEnterpriseMemberImportLeaseRenewal(false, true)
					cancelProcess()
					return
				}
				continue
			}
			if !renewed {
				RecordEnterpriseMemberImportLeaseRenewal(false, true)
				cancelProcess()
				return
			}
			RecordEnterpriseMemberImportLeaseRenewal(true, false)
			leaseDeadline = time.Now().Add(w.opts.LeaseTTL)
		}
	}
}

func (w *EnterpriseMemberImportWorker) Stop() {
	if w == nil || w.cancel == nil {
		return
	}
	w.cancel()
	w.waitGroup.Wait()
	w.cancel = nil
}

func ProvideEnterpriseMemberImportWorker(repo EnterpriseMemberImportRepository, importService *EnterpriseMemberImportService) *EnterpriseMemberImportWorker {
	worker := NewEnterpriseMemberImportWorker(repo, importService)
	worker.Start()
	return worker
}

func enterpriseMemberImportFailure(err error) (string, string) {
	code := "ENTERPRISE_MEMBER_IMPORT_FAILED"
	summary := "enterprise member import transaction failed"
	if appErr := infraerrors.FromError(err); appErr != nil {
		if value := strings.TrimSpace(appErr.Reason); value != "" {
			code = value
		}
		if value := strings.TrimSpace(appErr.Message); value != "" {
			summary = value
		}
	}
	if len(summary) > 1000 {
		summary = summary[:1000]
	}
	return code, summary
}
