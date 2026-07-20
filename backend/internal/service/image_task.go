package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	ImageTaskStatusProcessing = "processing"
	ImageTaskStatusCompleted  = "completed"
	ImageTaskStatusFailed     = "failed"

	ImageTaskPhaseQueued     = "queued"
	ImageTaskPhaseExecuting  = "executing"
	ImageTaskPhaseFinalizing = "finalizing"
	ImageTaskPhaseRecovering = "recovering"
	ImageTaskPhaseTerminal   = "terminal"

	ImageTaskBudgetStatusHeld        = "held"
	ImageTaskBudgetStatusNotRequired = "not_required"
	ImageTaskBudgetStatusSettled     = "settled"
	ImageTaskBudgetStatusReleased    = "released"
	ImageTaskBudgetStatusNeedsReview = "needs_review"

	defaultImageTaskTTL                 = 24 * time.Hour
	defaultImageTaskExecutionTimeout    = 30 * time.Minute
	defaultImageTaskFinalizationTimeout = 10 * time.Minute
	defaultImageTaskDispatchTimeout     = 2 * time.Minute
	defaultImageTaskRecoveryGrace       = time.Minute
	defaultImageTaskRecoveryInterval    = time.Minute
	defaultImageTaskRecoveryRetry       = 2 * time.Minute
	defaultImageTaskBudgetRecheck       = 10 * time.Minute
)

var (
	ErrImageTaskNotFound      = infraerrors.New(http.StatusNotFound, "IMAGE_TASK_NOT_FOUND", "image task not found")
	ErrImageTaskForbidden     = infraerrors.New(http.StatusForbidden, "IMAGE_TASK_FORBIDDEN", "image task does not belong to this API key")
	ErrImageTaskUnavailable   = infraerrors.New(http.StatusServiceUnavailable, "IMAGE_TASK_UNAVAILABLE", "image task storage is unavailable")
	ErrImageTaskStateConflict = infraerrors.New(http.StatusConflict, "IMAGE_TASK_STATE_CONFLICT", "image task lifecycle has already advanced")
)

// ImageTaskBudgetLink is the durable link between an asynchronous task and its
// enterprise member budget receipt. RequestID is already scoped by API key and
// must never be returned in the public task response.
type ImageTaskBudgetLink struct {
	RequestID string
	MemberID  int64
	GroupID   *int64
	HeldUSD   float64
}

// ImageTaskBudgetRecord is private task state persisted in Redis. It is kept
// separate from ImageTaskBudget so internal receipt identifiers never leak.
type ImageTaskBudgetRecord struct {
	RequestID string  `json:"request_id"`
	MemberID  int64   `json:"member_id"`
	GroupID   *int64  `json:"group_id,omitempty"`
	HeldUSD   float64 `json:"held_usd"`
	Status    string  `json:"status"`
	UpdatedAt int64   `json:"updated_at"`
}

// ImageTaskBudget is the customer-safe explanation of the task's temporary
// budget hold. HeldUSD is an authorization hold, not settled usage.
type ImageTaskBudget struct {
	TaskHoldUSD float64 `json:"task_hold_usd"`
	Status      string  `json:"status"`
	Message     string  `json:"message"`
}

// ImageTaskRecord is the private Redis representation of an asynchronous image
// request. Ownership fields are intentionally omitted from the public view.
type ImageTaskRecord struct {
	ID                  string                 `json:"id"`
	UserID              int64                  `json:"user_id"`
	APIKeyID            int64                  `json:"api_key_id"`
	Status              string                 `json:"status"`
	Phase               string                 `json:"phase"`
	RecoveryOriginPhase string                 `json:"recovery_origin_phase,omitempty"`
	RecoverAfter        int64                  `json:"recover_after"`
	Budget              *ImageTaskBudgetRecord `json:"budget,omitempty"`
	HTTPStatus          int                    `json:"http_status,omitempty"`
	Result              json.RawMessage        `json:"result,omitempty"`
	Error               json.RawMessage        `json:"error,omitempty"`
	CreatedAt           int64                  `json:"created_at"`
	CompletedAt         *int64                 `json:"completed_at,omitempty"`
	ExpiresAt           int64                  `json:"expires_at"`
}

// ImageTask is the API-safe task representation returned to callers.
type ImageTask struct {
	ID          string           `json:"id"`
	TaskID      string           `json:"task_id"`
	Object      string           `json:"object"`
	Status      string           `json:"status"`
	Phase       string           `json:"phase,omitempty"`
	Budget      *ImageTaskBudget `json:"budget,omitempty"`
	HTTPStatus  int              `json:"http_status,omitempty"`
	ImageURL    string           `json:"image_url,omitempty"`
	Result      json.RawMessage  `json:"result,omitempty"`
	Error       json.RawMessage  `json:"error,omitempty"`
	CreatedAt   int64            `json:"created_at"`
	CompletedAt *int64           `json:"completed_at,omitempty"`
	ExpiresAt   int64            `json:"expires_at"`
}

type ImageTaskOwner struct {
	UserID   int64
	APIKeyID int64
}

type ImageTaskStore interface {
	Save(ctx context.Context, task *ImageTaskRecord, ttl time.Duration) error
	Get(ctx context.Context, id string) (*ImageTaskRecord, error)
}

// ImageTaskRecoveryStore provides compare-and-swap lifecycle updates and a
// time-ordered recovery index. The production Redis store implements it; the
// smaller ImageTaskStore interface remains useful for isolated unit tests.
type ImageTaskRecoveryStore interface {
	ImageTaskStore
	Update(ctx context.Context, id string, ttl time.Duration, mutate func(*ImageTaskRecord) error) error
	ListRecoverable(ctx context.Context, before time.Time, limit int64) ([]*ImageTaskRecord, error)
}

type ImageTaskBudgetRecovery interface {
	GetReservationByRequestID(ctx context.Context, requestID string) (*EnterpriseMemberBudgetReservation, error)
	ReleaseImageTaskReservationByRequestID(ctx context.Context, requestID, taskID string) (*EnterpriseMemberBudgetReservation, error)
	MarkImageTaskReservationAmbiguousByRequestID(ctx context.Context, requestID, taskID, outcomeReason string) (*EnterpriseMemberBudgetReservation, error)
}

type ImageTaskBudgetTaskLookup interface {
	GetReservationByTaskID(ctx context.Context, taskID string) (*EnterpriseMemberBudgetReservation, error)
}

// ImageStorageResolver reports the currently effective object-storage binding.
// It exists so the async image feature can be switched on and off from the admin
// UI without a restart: the wiring below is fixed at startup, but the answer to
// "is object storage configured right now" is re-read (and cached) per call.
type ImageStorageResolver func() (uploader *ImageResultUploader, enabled bool)

type ImageTaskService struct {
	store            ImageTaskStore
	uploader         *ImageResultUploader
	enabled          bool
	resolve          ImageStorageResolver
	ttl              time.Duration
	executionTimeout time.Duration
	budgetRecovery   ImageTaskBudgetRecovery
	recoveryInterval time.Duration
	recoveryMu       sync.Mutex
	recoveryCancel   context.CancelFunc
	recoveryDone     chan struct{}
}

func NewImageTaskService(store ImageTaskStore) *ImageTaskService {
	return NewImageTaskServiceWithOptions(store, defaultImageTaskTTL, defaultImageTaskExecutionTimeout)
}

func NewImageTaskServiceWithOptions(store ImageTaskStore, ttl, executionTimeout time.Duration) *ImageTaskService {
	if ttl <= 0 {
		ttl = defaultImageTaskTTL
	}
	if executionTimeout <= 0 {
		executionTimeout = defaultImageTaskExecutionTimeout
	}
	return &ImageTaskService{
		store: store, ttl: ttl, executionTimeout: executionTimeout,
		recoveryInterval: defaultImageTaskRecoveryInterval,
	}
}

// NewImageTaskServiceWithUploader 构造一个已启用的图片任务服务：结果会先经 uploader
// 转存到对象存储再落 Redis。uploader 为 nil 时不做转存（仅用于测试）。
func NewImageTaskServiceWithUploader(store ImageTaskStore, uploader *ImageResultUploader, ttl, executionTimeout time.Duration) *ImageTaskService {
	s := NewImageTaskServiceWithOptions(store, ttl, executionTimeout)
	s.uploader = uploader
	s.enabled = true
	return s
}

// Enabled reports whether this instance can accept new asynchronous image
// submissions. Existing tasks remain readable and recoverable when uploads are
// temporarily disabled, so an object-storage configuration regression cannot
// strand accepted tasks or hide their budget state.

// NewImageTaskServiceWithResolver 构造一个由 resolver 决定启用状态的服务：
// 开关与凭证来自后台设置，保存后立即生效，无需重启。
func NewImageTaskServiceWithResolver(store ImageTaskStore, resolve ImageStorageResolver, ttl, executionTimeout time.Duration) *ImageTaskService {
	s := NewImageTaskServiceWithOptions(store, ttl, executionTimeout)
	s.resolve = resolve
	return s
}

// current 返回当前生效的 uploader 与启用状态。
// 注入了 resolver 时以 resolver 为准（后台设置可热切换），否则回落到构造时固定的值。
func (s *ImageTaskService) current() (*ImageResultUploader, bool) {
	if s == nil {
		return nil, false
	}
	if s.resolve != nil {
		return s.resolve()
	}
	return s.uploader, s.enabled
}

func (s *ImageTaskService) Enabled() bool {
	if s == nil || s.store == nil {
		return false
	}
	_, enabled := s.current()
	return enabled
}

// Pollable 表示已创建的任务能否被查询。
// 比 Enabled 弱：只要 store 可用即可，从而在功能被关掉后仍能取回进行中的任务结果。
func (s *ImageTaskService) Pollable() bool {
	return s != nil && s.store != nil
}

// Available reports whether durable task state can be read and reconciled.
// Unlike Enabled, this deliberately does not depend on the uploader.
func (s *ImageTaskService) Available() bool {
	return s != nil && s.store != nil
}

func (s *ImageTaskService) ExecutionTimeout() time.Duration {
	if s == nil || s.executionTimeout <= 0 {
		return defaultImageTaskExecutionTimeout
	}
	return s.executionTimeout
}

func (s *ImageTaskService) FinalizationTimeout() time.Duration {
	return defaultImageTaskFinalizationTimeout
}

func (s *ImageTaskService) Create(ctx context.Context, owner ImageTaskOwner) (*ImageTask, error) {
	return s.CreateWithBudget(ctx, owner, nil)
}

func (s *ImageTaskService) CreateWithBudget(ctx context.Context, owner ImageTaskOwner, budget *ImageTaskBudgetLink) (*ImageTask, error) {
	if s == nil || s.store == nil {
		return nil, ErrImageTaskUnavailable
	}
	now := time.Now().UTC()
	task := &ImageTaskRecord{
		ID:           "imgtask_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:       owner.UserID,
		APIKeyID:     owner.APIKeyID,
		Status:       ImageTaskStatusProcessing,
		Phase:        ImageTaskPhaseQueued,
		RecoverAfter: now.Add(defaultImageTaskDispatchTimeout).Unix(),
		CreatedAt:    now.Unix(),
		ExpiresAt:    now.Add(s.ttl).Unix(),
	}
	if budget != nil && strings.TrimSpace(budget.RequestID) != "" {
		budgetStatus := ImageTaskBudgetStatusNotRequired
		if budget.HeldUSD > 0 {
			budgetStatus = ImageTaskBudgetStatusHeld
		}
		task.Budget = &ImageTaskBudgetRecord{
			RequestID: strings.TrimSpace(budget.RequestID), MemberID: budget.MemberID,
			GroupID: budget.GroupID, HeldUSD: budget.HeldUSD, Status: budgetStatus,
			UpdatedAt: now.Unix(),
		}
	}
	if err := s.store.Save(ctx, task, s.ttl); err != nil {
		return nil, ErrImageTaskUnavailable.WithCause(err)
	}
	return imageTaskToPublic(task), nil
}

func (s *ImageTaskService) MarkExecuting(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return s.update(ctx, id, func(task *ImageTaskRecord) error {
		if task.Status != ImageTaskStatusProcessing || task.Phase != ImageTaskPhaseQueued {
			return ErrImageTaskStateConflict
		}
		task.Phase = ImageTaskPhaseExecuting
		task.RecoverAfter = now.Add(s.ExecutionTimeout() + defaultImageTaskRecoveryGrace).Unix()
		return nil
	})
}

func (s *ImageTaskService) MarkFinalizing(ctx context.Context, id string) error {
	return s.MarkFinalizingWithGroup(ctx, id, nil)
}

func (s *ImageTaskService) MarkFinalizingWithGroup(ctx context.Context, id string, groupID *int64) error {
	now := time.Now().UTC()
	return s.update(ctx, id, func(task *ImageTaskRecord) error {
		if task.Status != ImageTaskStatusProcessing || task.Phase == ImageTaskPhaseRecovering {
			return ErrImageTaskStateConflict
		}
		task.Phase = ImageTaskPhaseFinalizing
		task.RecoverAfter = now.Add(s.FinalizationTimeout() + defaultImageTaskRecoveryGrace).Unix()
		if task.Budget != nil && groupID != nil && *groupID > 0 {
			actualGroupID := *groupID
			task.Budget.GroupID = &actualGroupID
			task.Budget.UpdatedAt = now.Unix()
		}
		return nil
	})
}

func (s *ImageTaskService) Get(ctx context.Context, owner ImageTaskOwner, id string) (*ImageTask, error) {
	if s == nil || s.store == nil {
		return nil, ErrImageTaskUnavailable
	}
	task, err := s.store.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, ErrImageTaskNotFound) {
			return s.getBudgetBackedTaskTombstone(ctx, owner, strings.TrimSpace(id))
		}
		return nil, ErrImageTaskUnavailable.WithCause(err)
	}
	if task.UserID != owner.UserID || task.APIKeyID != owner.APIKeyID {
		// Do not reveal whether a random task ID exists for another caller.
		return nil, ErrImageTaskNotFound
	}
	return imageTaskToPublic(task), nil
}

func (s *ImageTaskService) getBudgetBackedTaskTombstone(ctx context.Context, owner ImageTaskOwner, taskID string) (*ImageTask, error) {
	lookup, ok := s.budgetRecovery.(ImageTaskBudgetTaskLookup)
	if !ok || owner.APIKeyID <= 0 || taskID == "" {
		return nil, ErrImageTaskNotFound
	}
	receipt, err := lookup.GetReservationByTaskID(ctx, taskID)
	if errors.Is(err, ErrEnterpriseMemberBudgetReceiptNotFound) {
		return nil, ErrImageTaskNotFound
	}
	if err != nil {
		return nil, ErrImageTaskUnavailable.WithCause(err)
	}
	if receipt == nil || receipt.ReceiptKind != EnterpriseMemberReceiptKindAsyncImage ||
		receipt.TaskID != taskID || !strings.HasPrefix(receipt.RequestID, strconv.FormatInt(owner.APIKeyID, 10)+":") {
		return nil, ErrImageTaskNotFound
	}
	now := time.Now().UTC()
	expiresAt := receipt.CreatedAt.Add(s.ttl)
	if receipt.CreatedAt.IsZero() {
		return nil, ErrImageTaskNotFound
	}
	unresolved := receipt.Status == "reserved" || receipt.Status == "ambiguous"
	if !unresolved && !now.Before(expiresAt) {
		return nil, ErrImageTaskNotFound
	}
	if unresolved {
		if receipt.ExpiresAt.After(expiresAt) {
			expiresAt = receipt.ExpiresAt
		}
		if !now.Before(expiresAt) {
			expiresAt = now.Add(defaultImageTaskBudgetRecheck)
		}
	}

	budgetStatus := ImageTaskBudgetStatusNeedsReview
	message := "image task state is unavailable and the upstream outcome is pending budget reconciliation"
	switch receipt.Status {
	case "reserved":
		budgetStatus = ImageTaskBudgetStatusHeld
		if receipt.TaskPhase == EnterpriseMemberAsyncTaskPhaseQueued {
			message = "image task state is unavailable before dispatch confirmation; its task budget hold will be released automatically by recovery"
		} else {
			message = "image task state is unavailable after dispatch began; its task budget hold remains active pending automatic reconciliation"
		}
	case "settled":
		budgetStatus = ImageTaskBudgetStatusSettled
		message = "image generation billing completed, but the task result is no longer available"
	case "released", "expired":
		budgetStatus = ImageTaskBudgetStatusReleased
		message = "the image task result is unavailable and its task budget hold was released"
	}
	return &ImageTask{
		ID: taskID, TaskID: taskID, Object: "image.generation.task",
		Status: ImageTaskStatusFailed, HTTPStatus: http.StatusServiceUnavailable,
		Budget: imageTaskBudgetToPublic(&ImageTaskBudgetRecord{HeldUSD: receipt.ReservedUSD, Status: budgetStatus}),
		Error:  imageTaskErrorJSON("recovery_error", message), CreatedAt: receipt.CreatedAt.Unix(),
		CompletedAt: unixTimestampPointer(now.Unix()), ExpiresAt: expiresAt.Unix(),
	}, nil
}

func unixTimestampPointer(value int64) *int64 { return &value }

func (s *ImageTaskService) Complete(ctx context.Context, id string, statusCode int, result json.RawMessage) error {
	return s.CompleteWithBudgetStatus(ctx, id, statusCode, result, "")
}

func (s *ImageTaskService) CompleteWithBudgetStatus(ctx context.Context, id string, statusCode int, result json.RawMessage, budgetStatus string) error {
	if !json.Valid(result) {
		return s.FailWithBudgetStatus(ctx, id, http.StatusBadGateway, imageTaskErrorJSON("api_error", "upstream returned a non-JSON image response"), budgetStatus)
	}
	if uploader, _ := s.current(); uploader != nil {
		rewritten, err := uploader.Rewrite(ctx, id, result)
		if err != nil {
			// 转存失败不回退存 base64，避免大 blob 撑爆 Redis：直接把任务标记为失败。
			logger.L().Error("image_task.offload_failed", zap.String("task_id", id), zap.Error(err))
			return s.FailWithBudgetStatus(ctx, id, http.StatusBadGateway, imageTaskErrorJSON("api_error", "failed to store generated image to object storage"), budgetStatus)
		}
		result = rewritten
	}
	return s.finish(ctx, id, ImageTaskStatusCompleted, statusCode, result, nil, budgetStatus, false)
}

func (s *ImageTaskService) Fail(ctx context.Context, id string, statusCode int, taskErr json.RawMessage) error {
	return s.FailWithBudgetStatus(ctx, id, statusCode, taskErr, "")
}

func (s *ImageTaskService) FailWithBudgetStatus(ctx context.Context, id string, statusCode int, taskErr json.RawMessage, budgetStatus string) error {
	if !json.Valid(taskErr) {
		taskErr = imageTaskErrorJSON("api_error", "image generation failed")
	}
	return s.finish(ctx, id, ImageTaskStatusFailed, statusCode, nil, taskErr, budgetStatus, false)
}

func (s *ImageTaskService) finish(ctx context.Context, id, status string, statusCode int, result, taskErr json.RawMessage, budgetStatus string, recovering bool) error {
	if s == nil || s.store == nil {
		return ErrImageTaskUnavailable
	}
	now := time.Now().UTC()
	completedAt := now.Unix()
	return s.update(ctx, id, func(task *ImageTaskRecord) error {
		if task.Status != ImageTaskStatusProcessing {
			return ErrImageTaskStateConflict
		}
		if recovering != (task.Phase == ImageTaskPhaseRecovering) {
			return ErrImageTaskStateConflict
		}
		task.Status = status
		task.Phase = ImageTaskPhaseTerminal
		task.RecoveryOriginPhase = ""
		task.RecoverAfter = 0
		task.HTTPStatus = statusCode
		task.Result = result
		task.Error = taskErr
		task.CompletedAt = &completedAt
		task.ExpiresAt = now.Add(s.ttl).Unix()
		setImageTaskBudgetStatus(task, budgetStatus, now)
		if task.Budget != nil && task.Budget.Status == ImageTaskBudgetStatusNeedsReview {
			task.RecoverAfter = now.Add(defaultImageTaskBudgetRecheck).Unix()
		}
		return nil
	})
}

func (s *ImageTaskService) update(ctx context.Context, id string, mutate func(*ImageTaskRecord) error) error {
	if s == nil || s.store == nil {
		return ErrImageTaskUnavailable
	}
	refreshExpiry := func(task *ImageTaskRecord) error {
		if err := mutate(task); err != nil {
			return err
		}
		task.ExpiresAt = time.Now().UTC().Add(s.ttl).Unix()
		return nil
	}
	if recoveryStore, ok := s.store.(ImageTaskRecoveryStore); ok {
		if err := recoveryStore.Update(ctx, id, s.ttl, refreshExpiry); err != nil {
			if errors.Is(err, ErrImageTaskNotFound) || errors.Is(err, ErrImageTaskStateConflict) {
				return err
			}
			return ErrImageTaskUnavailable.WithCause(err)
		}
		return nil
	}
	task, err := s.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrImageTaskNotFound) {
			return ErrImageTaskNotFound
		}
		return ErrImageTaskUnavailable.WithCause(err)
	}
	if err := refreshExpiry(task); err != nil {
		return err
	}
	if err := s.store.Save(ctx, task, s.ttl); err != nil {
		return ErrImageTaskUnavailable.WithCause(err)
	}
	return nil
}

func setImageTaskBudgetStatus(task *ImageTaskRecord, status string, now time.Time) {
	status = strings.TrimSpace(status)
	if task == nil || task.Budget == nil || status == "" {
		return
	}
	task.Budget.Status = status
	task.Budget.UpdatedAt = now.Unix()
}

// ConfigureBudgetRecovery wires the durable enterprise receipt reconciler.
// Start is intentionally separate so tests can configure recovery deterministically.
func (s *ImageTaskService) ConfigureBudgetRecovery(recovery ImageTaskBudgetRecovery) {
	if s != nil {
		s.budgetRecovery = recovery
	}
}

func (s *ImageTaskService) Start() {
	if s == nil || !s.Available() || s.budgetRecovery == nil {
		return
	}
	if _, ok := s.store.(ImageTaskRecoveryStore); !ok {
		return
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if s.recoveryCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.recoveryCancel = cancel
	s.recoveryDone = done
	go func() {
		defer close(done)
		s.recoveryLoop(ctx)
	}()
}

func (s *ImageTaskService) Stop() {
	if s == nil {
		return
	}
	s.recoveryMu.Lock()
	cancel := s.recoveryCancel
	done := s.recoveryDone
	s.recoveryMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	s.recoveryMu.Lock()
	if s.recoveryDone == done {
		s.recoveryCancel = nil
		s.recoveryDone = nil
	}
	s.recoveryMu.Unlock()
}

func (s *ImageTaskService) recoveryLoop(ctx context.Context) {
	s.recoverOnceAndLog(ctx)
	ticker := time.NewTicker(s.recoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.recoverOnceAndLog(ctx)
		}
	}
}

func (s *ImageTaskService) recoverOnceAndLog(ctx context.Context) {
	recovered, err := s.RecoverStale(ctx, 100)
	if err != nil {
		logger.L().Error("image_task.recovery_failed", zap.Error(err))
		return
	}
	if recovered > 0 {
		logger.L().Info("image_task.recovery_completed", zap.Int("recovered", recovered))
	}
}

// RecoverStale claims overdue task records before reconciling their durable
// budget receipts. A queued task is proven not to have reached upstream and can
// release its hold; any later phase is conservatively marked ambiguous.
func (s *ImageTaskService) RecoverStale(ctx context.Context, limit int64) (int, error) {
	store, ok := s.store.(ImageTaskRecoveryStore)
	if !ok || s.budgetRecovery == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	tasks, err := store.ListRecoverable(ctx, time.Now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, task := range tasks {
		var ok bool
		if task.Status == ImageTaskStatusProcessing {
			ok, err = s.recoverTask(ctx, task)
		} else {
			ok, err = s.reconcileTerminalBudget(ctx, task)
		}
		if err != nil {
			logger.L().Error("image_task.recovery_task_failed", zap.String("task_id", task.ID), zap.Error(err))
			continue
		}
		if ok {
			recovered++
		}
	}
	return recovered, nil
}

func (s *ImageTaskService) reconcileTerminalBudget(ctx context.Context, snapshot *ImageTaskRecord) (bool, error) {
	if snapshot == nil || snapshot.Budget == nil || snapshot.Budget.Status != ImageTaskBudgetStatusNeedsReview {
		return false, nil
	}
	receipt, err := s.budgetRecovery.GetReservationByRequestID(ctx, snapshot.Budget.RequestID)
	if err != nil && !errors.Is(err, ErrEnterpriseMemberBudgetReceiptNotFound) {
		return false, err
	}
	status := ImageTaskBudgetStatusNeedsReview
	if err == nil {
		switch receipt.Status {
		case "settled":
			status = ImageTaskBudgetStatusSettled
		case "released", "expired":
			status = ImageTaskBudgetStatusReleased
		}
	}
	now := time.Now().UTC()
	updated := false
	err = s.update(ctx, snapshot.ID, func(task *ImageTaskRecord) error {
		updated = false
		if task.Status == ImageTaskStatusProcessing || task.Budget == nil || task.Budget.Status != ImageTaskBudgetStatusNeedsReview {
			return nil
		}
		setImageTaskBudgetStatus(task, status, now)
		if status == ImageTaskBudgetStatusNeedsReview {
			task.RecoverAfter = now.Add(defaultImageTaskBudgetRecheck).Unix()
		} else {
			task.RecoverAfter = 0
			updated = true
		}
		return nil
	})
	return updated && err == nil, err
}

func (s *ImageTaskService) recoverTask(ctx context.Context, snapshot *ImageTaskRecord) (bool, error) {
	if snapshot == nil {
		return false, nil
	}
	now := time.Now().UTC()
	previousPhase := ""
	claimed := false
	err := s.update(ctx, snapshot.ID, func(task *ImageTaskRecord) error {
		// Redis WATCH may retry this closure after another writer wins. Proof
		// from a failed attempt must never authorize the next snapshot.
		previousPhase = ""
		claimed = false
		if task.Status != ImageTaskStatusProcessing || task.RecoverAfter <= 0 || task.RecoverAfter > now.Unix() {
			return nil
		}
		previousPhase = task.Phase
		if task.Phase == ImageTaskPhaseRecovering && task.RecoveryOriginPhase != "" {
			previousPhase = task.RecoveryOriginPhase
		} else {
			task.RecoveryOriginPhase = task.Phase
		}
		task.Phase = ImageTaskPhaseRecovering
		task.RecoverAfter = now.Add(defaultImageTaskRecoveryRetry).Unix()
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return false, err
	}

	budgetStatus, err := s.recoverBudget(ctx, snapshot.ID, snapshot.Budget, previousPhase)
	if err != nil {
		return false, err
	}
	message := "image generation task could not be resumed after service recovery"
	switch budgetStatus {
	case ImageTaskBudgetStatusReleased:
		message = "image generation was not dispatched before service recovery; the task budget hold was released"
	case ImageTaskBudgetStatusSettled:
		message = "image generation billing completed, but the task result was unavailable after service recovery"
	case ImageTaskBudgetStatusNeedsReview:
		message = "image generation outcome is unknown after service recovery; the task budget hold is pending reconciliation"
	}
	err = s.finish(ctx, snapshot.ID, ImageTaskStatusFailed, http.StatusServiceUnavailable, nil,
		imageTaskErrorJSON("recovery_error", message), budgetStatus, true)
	return err == nil, err
}

func (s *ImageTaskService) recoverBudget(ctx context.Context, taskID string, budget *ImageTaskBudgetRecord, previousPhase string) (string, error) {
	if budget == nil || strings.TrimSpace(budget.RequestID) == "" {
		return "", nil
	}
	receipt, err := s.budgetRecovery.GetReservationByRequestID(ctx, budget.RequestID)
	if err != nil {
		if errors.Is(err, ErrEnterpriseMemberBudgetReceiptNotFound) {
			return ImageTaskBudgetStatusNeedsReview, nil
		}
		return "", err
	}
	if receipt.Status == "reserved" {
		provenNotDispatched := previousPhase == ImageTaskPhaseQueued &&
			receipt.ReceiptKind == EnterpriseMemberReceiptKindAsyncImage &&
			strings.TrimSpace(receipt.TaskID) == strings.TrimSpace(taskID) &&
			receipt.TaskPhase == EnterpriseMemberAsyncTaskPhaseQueued
		if provenNotDispatched {
			if _, err := s.budgetRecovery.ReleaseImageTaskReservationByRequestID(ctx, budget.RequestID, taskID); err != nil {
				return "", err
			}
		} else if _, err := s.budgetRecovery.MarkImageTaskReservationAmbiguousByRequestID(ctx, budget.RequestID, taskID, "async_image_process_recovered"); err != nil && !errors.Is(err, ErrEnterpriseMemberBudgetConflict) {
			return "", err
		}
		receipt, err = s.budgetRecovery.GetReservationByRequestID(ctx, budget.RequestID)
		if err != nil {
			return "", err
		}
	}
	switch receipt.Status {
	case "settled":
		return ImageTaskBudgetStatusSettled, nil
	case "released", "expired":
		return ImageTaskBudgetStatusReleased, nil
	case "ambiguous", "reserved":
		return ImageTaskBudgetStatusNeedsReview, nil
	default:
		return ImageTaskBudgetStatusNeedsReview, nil
	}
}

func imageTaskToPublic(task *ImageTaskRecord) *ImageTask {
	if task == nil {
		return nil
	}
	return &ImageTask{
		ID:          task.ID,
		TaskID:      task.ID,
		Object:      "image.generation.task",
		Status:      task.Status,
		Phase:       imageTaskPublicPhase(task.Phase),
		Budget:      imageTaskBudgetToPublic(task.Budget),
		HTTPStatus:  task.HTTPStatus,
		ImageURL:    firstImageTaskURL(task.Result),
		Result:      task.Result,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt,
		CompletedAt: task.CompletedAt,
		ExpiresAt:   task.ExpiresAt,
	}
}

func imageTaskPublicPhase(phase string) string {
	switch phase {
	case ImageTaskPhaseQueued:
		return "queued"
	case ImageTaskPhaseExecuting:
		return "running"
	case ImageTaskPhaseFinalizing:
		return "finalizing"
	case ImageTaskPhaseRecovering:
		return "recovering"
	default:
		return ""
	}
}

func imageTaskBudgetToPublic(budget *ImageTaskBudgetRecord) *ImageTaskBudget {
	if budget == nil {
		return nil
	}
	message := "No task budget hold was required."
	switch budget.Status {
	case ImageTaskBudgetStatusHeld:
		message = fmt.Sprintf("US$%.2f is temporarily held for this asynchronous task; it is not settled usage.", budget.HeldUSD)
	case ImageTaskBudgetStatusSettled:
		message = "The task was billed from actual usage and its temporary hold was closed."
	case ImageTaskBudgetStatusReleased:
		message = "The task did not produce a confirmed charge and its temporary hold was released."
	case ImageTaskBudgetStatusNeedsReview:
		message = "The upstream outcome is unknown; the task hold remains protected until reconciliation completes."
	}
	return &ImageTaskBudget{TaskHoldUSD: budget.HeldUSD, Status: budget.Status, Message: message}
}

func firstImageTaskURL(result json.RawMessage) string {
	if len(result) == 0 || !json.Valid(result) {
		return ""
	}
	var response struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if json.Unmarshal(result, &response) != nil || len(response.Data) == 0 {
		return ""
	}
	return strings.TrimSpace(response.Data[0].URL)
}

func imageTaskErrorJSON(errorType, message string) json.RawMessage {
	data, _ := json.Marshal(map[string]string{"type": errorType, "message": message})
	return data
}
