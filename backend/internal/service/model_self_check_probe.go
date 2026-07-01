package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	modelSelfCheckErrorConfig     = "config_error"
	modelSelfCheckErrorRateLimit  = "rate_limited"
	modelSelfCheckErrorUpstream   = "upstream_error"
	modelSelfCheckErrorConnection = "conn_error"
	modelSelfCheckErrorTimeout    = "timeout"
	modelSelfCheckErrorMissing    = "model_missing"
	modelSelfCheckErrorNoAccount  = "no_account"
	modelSelfCheckErrorParse      = "parse_error"

	modelSelfCheckProbePrompt = "Reply with ok."
)

type ModelSelfCheckProbeTask struct {
	Key       string
	Model     string
	AccountID int64
	Platform  string
}

type ModelSelfCheckProbeResult struct {
	Status     string
	LatencyMs  *int
	HTTPStatus *int
	ErrorCode  string
}

type ModelSelfCheckProbeExecutor interface {
	Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult
}

type gatewayModelSelfCheckProbeExecutor struct {
	gatewayService            *GatewayService
	openAIGatewayService      *OpenAIGatewayService
	geminiCompatService       *GeminiMessagesCompatService
	antigravityGatewayService *AntigravityGatewayService
}

func NewGatewayModelSelfCheckProbeExecutor(
	gatewayService *GatewayService,
	openAIGatewayService *OpenAIGatewayService,
	geminiCompatService *GeminiMessagesCompatService,
	antigravityGatewayService *AntigravityGatewayService,
) ModelSelfCheckProbeExecutor {
	return &gatewayModelSelfCheckProbeExecutor{
		gatewayService:            gatewayService,
		openAIGatewayService:      openAIGatewayService,
		geminiCompatService:       geminiCompatService,
		antigravityGatewayService: antigravityGatewayService,
	}
}

func (s *ModelSelfCheckService) ListProbeTasks(ctx context.Context) ([]ModelSelfCheckProbeTask, error) {
	data, err := s.loadStatusSnapshotData(ctx)
	if err != nil {
		return nil, err
	}
	tasks := make([]ModelSelfCheckProbeTask, 0)
	seen := map[string]struct{}{}
	for _, target := range data.targets {
		for _, accountID := range s.accountIDsForTarget(ctx, target, data) {
			account := data.accountsByID[accountID]
			if account == nil {
				continue
			}
			key := modelSelfCheckTaskKey(target.Model, account.ID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			tasks = append(tasks, ModelSelfCheckProbeTask{
				Key:       key,
				Model:     target.Model,
				AccountID: account.ID,
				Platform:  account.Platform,
			})
		}
	}
	return tasks, nil
}

func (s *ModelSelfCheckService) RunProbe(ctx context.Context, task ModelSelfCheckProbeTask) error {
	if s == nil {
		return fmt.Errorf("run model self check probe: nil service")
	}
	if s.accountRepo == nil {
		return fmt.Errorf("run model self check probe: account repository is not configured")
	}
	model := strings.TrimSpace(task.Model)
	if model == "" || task.AccountID <= 0 {
		return fmt.Errorf("run model self check probe: invalid task")
	}
	account, err := s.accountRepo.GetByID(ctx, task.AccountID)
	if err != nil {
		return s.RecordHistory(ctx, &ModelSelfCheckHistory{
			Model:     model,
			AccountID: task.AccountID,
			Platform:  strings.TrimSpace(task.Platform),
			Status:    MonitorStatusFailed,
			ErrorCode: modelSelfCheckErrorNoAccount,
		})
	}
	if account == nil || !isAccountEligibleForSelfCheck(account) {
		return s.RecordHistory(ctx, &ModelSelfCheckHistory{
			Model:     model,
			AccountID: task.AccountID,
			Platform:  strings.TrimSpace(task.Platform),
			Status:    MonitorStatusFailed,
			ErrorCode: modelSelfCheckErrorNoAccount,
		})
	}
	if !s.isModelSupportedBySelfCheckAccount(ctx, account, model) {
		return s.RecordHistory(ctx, &ModelSelfCheckHistory{
			Model:     model,
			AccountID: account.ID,
			Platform:  account.Platform,
			Status:    MonitorStatusFailed,
			ErrorCode: modelSelfCheckErrorMissing,
		})
	}
	if s.probeExecutor == nil {
		return fmt.Errorf("run model self check probe: probe executor is not configured")
	}
	result := s.probeExecutor.Probe(ctx, account, model)
	return s.RecordHistory(ctx, &ModelSelfCheckHistory{
		Model:      model,
		AccountID:  account.ID,
		Platform:   account.Platform,
		Status:     result.Status,
		LatencyMs:  result.LatencyMs,
		HTTPStatus: result.HTTPStatus,
		ErrorCode:  result.ErrorCode,
	})
}

func (s *ModelSelfCheckService) isModelSupportedBySelfCheckAccount(ctx context.Context, account *Account, model string) bool {
	if account == nil {
		return false
	}
	if s != nil && s.gatewayServiceForModelSupport() != nil {
		return s.gatewayServiceForModelSupport().isModelSupportedByAccountWithContext(ctx, account, model)
	}
	return account.IsModelSupported(model)
}

func (s *ModelSelfCheckService) gatewayServiceForModelSupport() *GatewayService {
	if s == nil || s.probeExecutor == nil {
		return nil
	}
	if executor, ok := s.probeExecutor.(*gatewayModelSelfCheckProbeExecutor); ok {
		return executor.gatewayService
	}
	return nil
}

func isAccountEligibleForSelfCheck(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Status != "" && account.Status != StatusActive {
		return false
	}
	if !account.Schedulable {
		return false
	}
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(time.Now().UTC()) {
		return false
	}
	return true
}

func uniqueSelfCheckAccountIDs(accounts []ModelSelfCheckTargetAccount) []int64 {
	ids := make([]int64, 0, len(accounts))
	seen := map[int64]struct{}{}
	for _, account := range accounts {
		if account.AccountID <= 0 {
			continue
		}
		if _, ok := seen[account.AccountID]; ok {
			continue
		}
		seen[account.AccountID] = struct{}{}
		ids = append(ids, account.AccountID)
	}
	return ids
}

func modelSelfCheckTaskKey(model string, accountID int64) string {
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(model)), accountID)
}

func (e *gatewayModelSelfCheckProbeExecutor) Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult {
	if account == nil {
		return failedSelfCheckProbeResult(0, modelSelfCheckErrorNoAccount)
	}
	ctx = withModelSelfCheckProbeContext(ctx)
	start := time.Now()
	var status int
	var err error
	var duration time.Duration

	switch strings.ToLower(strings.TrimSpace(account.Platform)) {
	case PlatformOpenAI:
		status, duration, err = e.probeOpenAI(ctx, account, model)
	case PlatformGemini:
		status, duration, err = e.probeGemini(ctx, account, model)
	case PlatformAntigravity:
		status, duration, err = e.probeAntigravity(ctx, account, model)
	case PlatformAnthropic:
		status, duration, err = e.probeAnthropic(ctx, account, model)
	default:
		if account.IsBedrock() {
			status, duration, err = e.probeAnthropic(ctx, account, model)
			break
		}
		return failedSelfCheckProbeResult(0, modelSelfCheckErrorConfig)
	}
	if duration <= 0 {
		duration = time.Since(start)
	}
	latency := int(duration.Milliseconds())
	if latency < 0 {
		latency = 0
	}
	return normalizeSelfCheckProbeResult(status, err, latency)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeAnthropic(ctx context.Context, account *Account, model string) (int, time.Duration, error) {
	if e == nil || e.gatewayService == nil {
		return 0, 0, fmt.Errorf("anthropic gateway service is not configured")
	}
	body, err := buildAnthropicSelfCheckBody(model)
	if err != nil {
		return 0, 0, err
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), domain.PlatformAnthropic)
	if err != nil {
		return 0, 0, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/messages", body)
	result, err := e.gatewayService.Forward(ctx, c, account, parsed)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, modelSelfCheckProbeError(c, err)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeOpenAI(ctx context.Context, account *Account, model string) (int, time.Duration, error) {
	if e == nil || e.openAIGatewayService == nil {
		return 0, 0, fmt.Errorf("openai gateway service is not configured")
	}
	body, err := buildChatCompletionsSelfCheckBody(model)
	if err != nil {
		return 0, 0, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/chat/completions", body)
	result, err := e.openAIGatewayService.ForwardAsChatCompletions(ctx, c, account, body, "", "")
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, modelSelfCheckProbeError(c, err)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeGemini(ctx context.Context, account *Account, model string) (int, time.Duration, error) {
	if e == nil || e.geminiCompatService == nil {
		return 0, 0, fmt.Errorf("gemini compat service is not configured")
	}
	body, err := buildChatCompletionsSelfCheckBody(model)
	if err != nil {
		return 0, 0, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/chat/completions", body)
	result, err := e.geminiCompatService.ForwardAsChatCompletions(ctx, c, account, body)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, modelSelfCheckProbeError(c, err)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeAntigravity(ctx context.Context, account *Account, model string) (int, time.Duration, error) {
	if e == nil || e.antigravityGatewayService == nil {
		return 0, 0, fmt.Errorf("antigravity gateway service is not configured")
	}
	body, err := buildAnthropicSelfCheckBody(model)
	if err != nil {
		return 0, 0, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/messages", body)
	result, err := e.antigravityGatewayService.Forward(ctx, c, account, body, false)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, modelSelfCheckProbeError(c, err)
}

func buildAnthropicSelfCheckBody(model string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": modelSelfCheckProbePrompt}},
		"max_tokens": 1,
		"stream":     false,
	})
}

func buildChatCompletionsSelfCheckBody(model string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": modelSelfCheckProbePrompt}},
		"max_tokens": 1,
		"stream":     false,
	})
}

func withModelSelfCheckProbeContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if isModelSelfCheckProbeContext(ctx) {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ModelSelfCheckProbe, true)
}

func isModelSelfCheckProbeContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(ctxkey.ModelSelfCheckProbe).(bool)
	return v
}

func newModelSelfCheckGinContext(ctx context.Context, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if ctx == nil {
		ctx = context.Background()
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-model-self-check/1.0")
	c.Request = req
	return c, recorder
}

func modelSelfCheckHTTPStatus(c *gin.Context, recorder *httptest.ResponseRecorder, err error) int {
	if c != nil {
		if v, ok := c.Get(OpsUpstreamStatusCodeKey); ok {
			switch status := v.(type) {
			case int:
				if status > 0 {
					return status
				}
			case int64:
				if status > 0 {
					return int(status)
				}
			case float64:
				if status > 0 {
					return int(status)
				}
			}
		}
		if v, ok := c.Get(OpsUpstreamErrorsKey); ok {
			if events, ok := v.([]*OpsUpstreamErrorEvent); ok {
				for i := len(events) - 1; i >= 0; i-- {
					if events[i] != nil && events[i].UpstreamStatusCode > 0 {
						return events[i].UpstreamStatusCode
					}
				}
			}
		}
	}
	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) && failoverErr.StatusCode > 0 {
		return failoverErr.StatusCode
	}
	if recorder == nil {
		return 0
	}
	return recorder.Result().StatusCode
}

func modelSelfCheckProbeError(c *gin.Context, err error) error {
	event := latestModelSelfCheckOpsError(c)
	if event != nil && event.UpstreamStatusCode <= 0 && strings.EqualFold(strings.TrimSpace(event.Kind), "request_error") {
		msg := strings.TrimSpace(event.Message)
		if msg == "" {
			msg = "upstream request failed"
		}
		return fmt.Errorf("%s", msg)
	}
	return err
}

func latestModelSelfCheckOpsError(c *gin.Context) *OpsUpstreamErrorEvent {
	if c == nil {
		return nil
	}
	v, ok := c.Get(OpsUpstreamErrorsKey)
	if !ok {
		return nil
	}
	events, ok := v.([]*OpsUpstreamErrorEvent)
	if !ok {
		return nil
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i] != nil {
			return events[i]
		}
	}
	return nil
}

func normalizeSelfCheckProbeResult(httpStatus int, err error, latencyMs int) ModelSelfCheckProbeResult {
	statusPtr := optionalHTTPStatus(httpStatus)
	if err == nil && (httpStatus == 0 || httpStatus < 400) {
		return ModelSelfCheckProbeResult{
			Status:     MonitorStatusOperational,
			LatencyMs:  &latencyMs,
			HTTPStatus: statusPtr,
		}
	}
	code := modelSelfCheckErrorCode(err, httpStatus)
	status := MonitorStatusFailed
	if code == modelSelfCheckErrorRateLimit {
		status = MonitorStatusDegraded
	}
	return ModelSelfCheckProbeResult{
		Status:     status,
		LatencyMs:  &latencyMs,
		HTTPStatus: statusPtr,
		ErrorCode:  code,
	}
}

func failedSelfCheckProbeResult(httpStatus int, code string) ModelSelfCheckProbeResult {
	return ModelSelfCheckProbeResult{
		Status:     MonitorStatusFailed,
		HTTPStatus: optionalHTTPStatus(httpStatus),
		ErrorCode:  code,
	}
}

func optionalHTTPStatus(status int) *int {
	if status <= 0 {
		return nil
	}
	return &status
}

func modelSelfCheckErrorCode(err error, httpStatus int) string {
	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) && failoverErr.StatusCode > 0 {
		httpStatus = failoverErr.StatusCode
	}
	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return modelSelfCheckErrorConfig
	case http.StatusTooManyRequests:
		return modelSelfCheckErrorRateLimit
	case http.StatusNotFound:
		return modelSelfCheckErrorMissing
	}
	if err == nil {
		if httpStatus >= 500 {
			return modelSelfCheckErrorUpstream
		}
		if httpStatus >= 400 {
			return modelSelfCheckErrorUpstream
		}
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return modelSelfCheckErrorTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return modelSelfCheckErrorTimeout
		}
		return modelSelfCheckErrorConnection
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return modelSelfCheckErrorTimeout
	case strings.Contains(msg, "model") && (strings.Contains(msg, "not found") || strings.Contains(msg, "missing") || strings.Contains(msg, "unsupported")):
		return modelSelfCheckErrorMissing
	case strings.Contains(msg, "parse"), strings.Contains(msg, "invalid request body"), strings.Contains(msg, "unmarshal"):
		return modelSelfCheckErrorParse
	case strings.Contains(msg, "connection"), strings.Contains(msg, "connect:"), strings.Contains(msg, "no such host"):
		return modelSelfCheckErrorConnection
	default:
		if httpStatus >= 500 {
			return modelSelfCheckErrorUpstream
		}
		return modelSelfCheckErrorUpstream
	}
}
