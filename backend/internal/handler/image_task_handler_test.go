package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type asyncImageMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*service.ImageTaskRecord
}

type asyncImageBudgetRepoSpy struct {
	service.EnterpriseMemberBudgetRepository
	reservation         *service.EnterpriseMemberBudgetReservation
	reserveRequestID    string
	releasedRequestID   string
	releasedTaskID      string
	genericReleaseCalls int
	ambiguousRequestID  string
	ambiguousReason     string
	attachedRequestID   string
	attachedTaskID      string
	attachErr           error
	releaseAsyncErr     error
	executingRequestID  string
	executingTaskID     string
	executingErr        error
}

func (s *asyncImageBudgetRepoSpy) Reserve(_ context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, expiresAt time.Time) (*service.EnterpriseMemberBudgetReservation, error) {
	s.reserveRequestID = requestID
	if s.reservation != nil {
		copy := *s.reservation
		return &copy, nil
	}
	return &service.EnterpriseMemberBudgetReservation{
		RequestID: requestID, MemberID: memberID, GroupID: groupID, PayloadHash: payloadHash,
		ReservedUSD: amount, Status: "reserved", ExpiresAt: expiresAt,
	}, nil
}

func (s *asyncImageBudgetRepoSpy) Release(_ context.Context, requestID string) error {
	s.genericReleaseCalls++
	s.releasedRequestID = requestID
	return nil
}

func (s *asyncImageBudgetRepoSpy) MarkAmbiguous(_ context.Context, requestID, outcomeReason string) error {
	s.ambiguousRequestID = requestID
	s.ambiguousReason = outcomeReason
	return nil
}

func (s *asyncImageBudgetRepoSpy) ReleaseAsyncTask(_ context.Context, requestID, taskID string) (*service.EnterpriseMemberBudgetReservation, error) {
	s.releasedRequestID = requestID
	s.releasedTaskID = taskID
	if s.releaseAsyncErr != nil {
		return nil, s.releaseAsyncErr
	}
	return &service.EnterpriseMemberBudgetReservation{RequestID: requestID, TaskID: taskID, Status: "released"}, nil
}

func (s *asyncImageBudgetRepoSpy) MarkAsyncTaskAmbiguous(_ context.Context, requestID, taskID, outcomeReason string) (*service.EnterpriseMemberBudgetReservation, error) {
	s.ambiguousRequestID = requestID
	s.ambiguousReason = outcomeReason
	return &service.EnterpriseMemberBudgetReservation{RequestID: requestID, TaskID: taskID, Status: "ambiguous"}, nil
}

func (s *asyncImageBudgetRepoSpy) AttachAsyncTask(_ context.Context, requestID, taskID string, _ time.Time) error {
	s.attachedRequestID = requestID
	s.attachedTaskID = taskID
	return s.attachErr
}

func (s *asyncImageBudgetRepoSpy) MarkAsyncTaskExecuting(_ context.Context, requestID, taskID string) error {
	s.executingRequestID = requestID
	s.executingTaskID = taskID
	return s.executingErr
}

func (s *asyncImageMemoryStore) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	if task.Budget != nil {
		budgetCopy := *task.Budget
		if task.Budget.GroupID != nil {
			groupID := *task.Budget.GroupID
			budgetCopy.GroupID = &groupID
		}
		copy.Budget = &budgetCopy
	}
	s.tasks[task.ID] = &copy
	return nil
}

func (s *asyncImageMemoryStore) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := s.tasks[id]
	if task == nil {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	if task.Budget != nil {
		budgetCopy := *task.Budget
		if task.Budget.GroupID != nil {
			groupID := *task.Budget.GroupID
			budgetCopy.GroupID = &groupID
		}
		copy.Budget = &budgetCopy
	}
	return &copy, nil
}

func TestAsyncImageHandlerSubmitAndPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	release := make(chan struct{})
	budgetRepo := &asyncImageBudgetRepoSpy{}
	h := &AsyncImageHandler{
		tasks:               tasks,
		memberBudgetService: service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil),
	}
	h.execute = func(_ string, c *gin.Context) {
		<-release
		c.JSON(http.StatusOK, gin.H{"created": 123, "data": []gin.H{{"url": "https://example.test/image.png"}}})
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	requestCtx := context.WithValue(context.Background(), ctxkey.MemberBudgetReservation, &service.EnterpriseMemberBudgetReservation{
		RequestID: "9:client:submit-and-poll", MemberID: 12, ReservedUSD: 4, Status: "reserved",
	})
	requestCtx, cancelRequest := context.WithCancel(requestCtx)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))

	var accepted struct {
		TaskID  string                   `json:"task_id"`
		Status  string                   `json:"status"`
		Phase   string                   `json:"phase"`
		Budget  *service.ImageTaskBudget `json:"budget"`
		PollURL string                   `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Equal(t, service.ImageTaskStatusProcessing, accepted.Status)
	require.Equal(t, "queued", accepted.Phase)
	require.NotNil(t, accepted.Budget)
	require.Equal(t, service.ImageTaskBudgetStatusHeld, accepted.Budget.Status)
	require.Equal(t, 4.0, accepted.Budget.TaskHoldUSD)
	require.Contains(t, accepted.Budget.Message, "not settled usage")
	require.Equal(t, "/v1/images/tasks/"+accepted.TaskID, accepted.PollURL)
	require.Equal(t, accepted.PollURL, w.Header().Get("Location"))

	// The detached background request must survive completion of/cancellation
	// from the short submission request.
	cancelRequest()
	close(release)
	require.Eventually(t, func() bool {
		got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusCompleted && got.Budget.Status == service.ImageTaskBudgetStatusSettled
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "9:client:submit-and-poll", budgetRepo.attachedRequestID)
	require.Equal(t, accepted.TaskID, budgetRepo.attachedTaskID)
	require.Equal(t, "9:client:submit-and-poll", budgetRepo.executingRequestID)
	require.Equal(t, accepted.TaskID, budgetRepo.executingTaskID)

	pollReq := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Equal(t, "no-store", pollWriter.Header().Get("Cache-Control"))
	require.Empty(t, pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), "https://example.test/image.png")
}

func TestAsyncImageHandlerMiddlewareCannotReleaseOriginalTaskHoldAfterAttachConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	memberID := int64(12)
	groupID := int64(3)
	requestID := "9:client:duplicate-async"
	budgetRepo := &asyncImageBudgetRepoSpy{
		reservation: &service.EnterpriseMemberBudgetReservation{
			RequestID: requestID, MemberID: memberID, GroupID: &groupID, ReservedUSD: 4,
			Status: "reserved", ReceiptKind: service.EnterpriseMemberReceiptKindAsyncImage,
			TaskID: "imgtask_original", TaskPhase: service.EnterpriseMemberAsyncTaskPhaseExecuting,
		},
		attachErr:       service.ErrEnterpriseMemberBudgetConflict,
		releaseAsyncErr: service.ErrEnterpriseMemberBudgetConflict,
	}
	budgetService := service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil)
	h := &AsyncImageHandler{tasks: tasks, memberBudgetService: budgetService, execute: func(_ string, _ *gin.Context) {
		t.Fatal("upstream execution must not start when the receipt is already bound to another task")
	}}
	key := &service.APIKey{
		ID: 9, UserID: 7, MemberID: &memberID, GroupID: &groupID,
		Member: &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 7, Status: service.EnterpriseMemberStatusActive},
		Group:  &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), key)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "duplicate-async"))
		c.Next()
	})
	router.Use(middleware2.EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, middleware2.AnthropicErrorWriter))
	router.POST("/v1/images/generations/async", h.Submit)

	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Equal(t, requestID, budgetRepo.reserveRequestID)
	require.Equal(t, requestID, budgetRepo.attachedRequestID)
	require.NotEmpty(t, budgetRepo.attachedTaskID)
	require.NotEqual(t, "imgtask_original", budgetRepo.attachedTaskID)
	require.Equal(t, budgetRepo.attachedTaskID, budgetRepo.releasedTaskID,
		"handler cleanup must use the new task ID and be rejected by the task fence")
	require.Zero(t, budgetRepo.genericReleaseCalls,
		"outer middleware must not fall back to request-ID-only release after task creation")
	require.Len(t, store.tasks, 1)
	for _, task := range store.tasks {
		require.Equal(t, service.ImageTaskStatusFailed, task.Status)
		require.Equal(t, service.ImageTaskBudgetStatusNeedsReview, task.Budget.Status)
	}
}

// When object storage is not configured, new submissions are disabled but
// previously accepted tasks remain pollable from durable task storage.
func TestAsyncImageHandlerDisabledRejectsSubmitButKeepsExistingTasksPollable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithOptions(store, time.Hour, time.Minute) // enabled == false
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)
	existing, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not enabled")

	pollReq := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/"+existing.ID, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Contains(t, pollWriter.Body.String(), existing.ID)

	// The rejected submission did not create an additional task.
	require.Len(t, store.tasks, 1)
}

func TestAsyncImageHandlerRunReleasesMemberBudgetHoldOnDefinitiveFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	task, err := tasks.CreateWithBudget(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, &service.ImageTaskBudgetLink{
		RequestID: "9:client:async-image-failure", MemberID: 12, HeldUSD: 4,
	})
	require.NoError(t, err)
	budgetRepo := &asyncImageBudgetRepoSpy{}
	h := &AsyncImageHandler{
		tasks:               tasks,
		memberBudgetService: service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil),
	}
	h.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid prompt"}})
	}

	original, _ := gin.CreateTestContext(httptest.NewRecorder())
	groupID := int64(3)
	original.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, GroupID: &groupID})
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2"}`))
	requestCtx := context.WithValue(request.Context(), ctxkey.ClientRequestID, "async-image-failure")
	requestCtx = context.WithValue(requestCtx, ctxkey.MemberBudgetReservation, &service.EnterpriseMemberBudgetReservation{RequestID: "9:client:async-image-failure", ReservedUSD: 4})
	original.Request = request.WithContext(requestCtx)
	taskCtx, recorder, cancel := newAsyncImageContext(original, []byte(`{"model":"gpt-image-2"}`), time.Minute)

	h.run(task.ID, service.PlatformOpenAI, taskCtx, recorder, cancel)

	require.Equal(t, "9:client:async-image-failure", budgetRepo.releasedRequestID)
	require.Equal(t, task.ID, budgetRepo.releasedTaskID)
	require.Empty(t, budgetRepo.ambiguousRequestID)
	stored, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskBudgetStatusReleased, stored.Budget.Status)
}

func TestAsyncImageHandlerRunDoesNotDispatchWhenPostgresExecutionFenceFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	task, err := tasks.CreateWithBudget(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, &service.ImageTaskBudgetLink{
		RequestID: "9:client:fence-failure", MemberID: 12, HeldUSD: 4,
	})
	require.NoError(t, err)
	budgetRepo := &asyncImageBudgetRepoSpy{executingErr: errors.New("postgres unavailable")}
	executed := false
	h := &AsyncImageHandler{
		tasks:               tasks,
		memberBudgetService: service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil),
		execute:             func(_ string, _ *gin.Context) { executed = true },
	}
	original, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2"}`))
	requestCtx := context.WithValue(request.Context(), ctxkey.MemberBudgetReservation, &service.EnterpriseMemberBudgetReservation{
		RequestID: "9:client:fence-failure", ReservedUSD: 4,
	})
	original.Request = request.WithContext(requestCtx)
	taskCtx, recorder, cancel := newAsyncImageContext(original, []byte(`{"model":"gpt-image-2"}`), time.Minute)

	h.run(task.ID, service.PlatformOpenAI, taskCtx, recorder, cancel)

	require.False(t, executed, "upstream execution must not start without the PostgreSQL durability fence")
	require.Equal(t, "9:client:fence-failure", budgetRepo.releasedRequestID)
	require.Equal(t, task.ID, budgetRepo.releasedTaskID)
	stored, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusFailed, stored.Status)
	require.Equal(t, service.ImageTaskBudgetStatusReleased, stored.Budget.Status)
	require.Contains(t, string(stored.Error), "durable execution fence")
}

func TestAsyncImageHandlerRunKeepsMemberBudgetHoldWhenSuccessResponseIsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	task, err := tasks.CreateWithBudget(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, &service.ImageTaskBudgetLink{
		RequestID: "9:client:async-image-invalid-success", MemberID: 12, HeldUSD: 4,
	})
	require.NoError(t, err)
	budgetRepo := &asyncImageBudgetRepoSpy{}
	h := &AsyncImageHandler{
		tasks:               tasks,
		memberBudgetService: service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil),
	}
	h.execute = func(_ string, c *gin.Context) {
		c.Status(http.StatusOK)
	}

	original, _ := gin.CreateTestContext(httptest.NewRecorder())
	groupID := int64(3)
	original.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, GroupID: &groupID})
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2"}`))
	requestCtx := context.WithValue(request.Context(), ctxkey.ClientRequestID, "async-image-invalid-success")
	requestCtx = context.WithValue(requestCtx, ctxkey.MemberBudgetReservation, &service.EnterpriseMemberBudgetReservation{RequestID: "9:client:async-image-invalid-success", ReservedUSD: 4})
	original.Request = request.WithContext(requestCtx)
	taskCtx, recorder, cancel := newAsyncImageContext(original, []byte(`{"model":"gpt-image-2"}`), time.Minute)

	h.run(task.ID, service.PlatformOpenAI, taskCtx, recorder, cancel)

	require.Empty(t, budgetRepo.releasedRequestID)
	require.Equal(t, "9:client:async-image-invalid-success", budgetRepo.ambiguousRequestID)
	require.Equal(t, "async_image_invalid_success_response", budgetRepo.ambiguousReason)
	stored, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskBudgetStatusNeedsReview, stored.Budget.Status)
}

func TestAsyncImageHandlerRunMarksValidSuccessAmbiguousWhenUsagePersistenceFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	task, err := tasks.CreateWithBudget(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, &service.ImageTaskBudgetLink{
		RequestID: "9:client:async-image-usage-persistence", MemberID: 12, HeldUSD: 4,
	})
	require.NoError(t, err)
	budgetRepo := &asyncImageBudgetRepoSpy{}
	h := &AsyncImageHandler{
		tasks:               tasks,
		memberBudgetService: service.NewEnterpriseMemberBudgetService(budgetRepo, nil, nil),
	}
	h.execute = func(_ string, c *gin.Context) {
		service.MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "usage_persistence_failed")
		c.JSON(http.StatusOK, gin.H{"created": 123, "data": []gin.H{{"url": "https://example.test/image.png"}}})
	}

	original, _ := gin.CreateTestContext(httptest.NewRecorder())
	groupID := int64(3)
	original.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, GroupID: &groupID})
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2"}`))
	requestCtx := context.WithValue(request.Context(), ctxkey.ClientRequestID, "async-image-usage-persistence")
	requestCtx = context.WithValue(requestCtx, ctxkey.MemberBudgetReservation, &service.EnterpriseMemberBudgetReservation{RequestID: "9:client:async-image-usage-persistence", ReservedUSD: 4})
	original.Request = request.WithContext(requestCtx)
	taskCtx, recorder, cancel := newAsyncImageContext(original, []byte(`{"model":"gpt-image-2"}`), time.Minute)

	h.run(task.ID, service.PlatformOpenAI, taskCtx, recorder, cancel)

	require.Empty(t, budgetRepo.releasedRequestID)
	require.Equal(t, "9:client:async-image-usage-persistence", budgetRepo.ambiguousRequestID)
	require.Equal(t, "usage_persistence_failed", budgetRepo.ambiguousReason)
	stored, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.TaskID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusCompleted, stored.Status)
	require.Equal(t, service.ImageTaskBudgetStatusNeedsReview, stored.Budget.Status)
}
