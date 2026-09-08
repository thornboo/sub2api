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
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
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
	GroupID   int64
	Model     string
	AccountID int64
	Platform  string
}

type ModelSelfCheckProbeResult struct {
	Status            string
	LatencyMs         *int
	HTTPStatus        *int
	ErrorCode         string
	InputTokens       int
	OutputTokens      int
	Recovered         bool
	Transient         bool
	RetryCount        int
	InitialHTTPStatus *int
}

type modelSelfCheckSessionKey struct{}

var errModelSelfCheckIncomplete = errors.New("model self check response did not complete")

// Only an explicit HTTP rejection before any response stream is safe to retry.
// Stream failures and unknown transport outcomes must not be replayed here.
type modelSelfCheckRetryableHTTPError struct{ *UpstreamFailoverError }

func (e *modelSelfCheckRetryableHTTPError) Unwrap() error { return e.UpstreamFailoverError }

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
	if s.roundRepo() != nil {
		targets, err := s.repo.ListStatusTargets(ctx)
		if err != nil {
			return nil, fmt.Errorf("list model self check targets: %w", err)
		}
		sortSelfCheckTargets(targets)
		tasks := make([]ModelSelfCheckProbeTask, 0, len(targets))
		seen := map[string]struct{}{}
		for _, target := range targets {
			key := modelSelfCheckTaskKey(target.GroupID, target.Model)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			tasks = append(tasks, ModelSelfCheckProbeTask{
				Key:     key,
				GroupID: target.GroupID,
				Model:   target.Model,
			})
		}
		return tasks, nil
	}
	data, err := s.loadStatusSnapshotData(ctx)
	if err != nil {
		return nil, err
	}
	tasks := make([]ModelSelfCheckProbeTask, 0)
	seen := map[string]struct{}{}
	for _, target := range data.targets {
		candidates := eligibleModelSelfCheckCandidates(s.modelSelfCheckCandidates(ctx, target, data))
		key := modelSelfCheckTaskKey(target.GroupID, target.Model)
		if _, ok := seen[key]; ok || len(candidates) == 0 {
			continue
		}
		seen[key] = struct{}{}
		tasks = append(tasks, ModelSelfCheckProbeTask{
			Key:     key,
			GroupID: target.GroupID,
			Model:   target.Model,
		})
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
		if errors.Is(err, ErrAccountNotFound) {
			return nil
		}
		return fmt.Errorf("run model self check probe: load account %d: %w", task.AccountID, err)
	}
	parentLookup, err := s.selfCheckParentLookup(ctx, account)
	if err != nil {
		return err
	}
	if account == nil || !isAccountEligibleForSelfCheck(ctx, account, model, parentLookup) {
		return nil
	}
	if !s.isModelSupportedBySelfCheckAccount(ctx, account, model) {
		return nil
	}
	if s.probeExecutor == nil {
		return fmt.Errorf("run model self check probe: probe executor is not configured")
	}
	result := s.probeExecutor.Probe(ctx, account, model)
	return s.RecordHistory(ctx, &ModelSelfCheckHistory{
		Model:        model,
		AccountID:    account.ID,
		Platform:     account.Platform,
		Status:       result.Status,
		LatencyMs:    result.LatencyMs,
		HTTPStatus:   result.HTTPStatus,
		ErrorCode:    result.ErrorCode,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
	})
}

func (s *ModelSelfCheckService) selfCheckParentLookup(ctx context.Context, account *Account) (func(int64) *Account, error) {
	if account == nil || !account.IsShadow() {
		return func(int64) *Account { return nil }, nil
	}
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("run model self check probe: account repository is not configured")
	}
	parentID := *account.ParentAccountID
	parent, err := s.accountRepo.GetByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return func(int64) *Account { return nil }, nil
		}
		return nil, fmt.Errorf("run model self check probe: load parent account %d: %w", parentID, err)
	}
	return func(id int64) *Account {
		if id == parentID {
			return parent
		}
		return nil
	}, nil
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

func isAccountEligibleForSelfCheck(ctx context.Context, account *Account, model string, lookup func(int64) *Account) bool {
	if account == nil {
		return false
	}
	if !account.IsSchedulableForModelWithContext(ctx, model) {
		return false
	}
	if lookup == nil {
		lookup = func(int64) *Account { return nil }
	}
	return parentHealthyForShadow(account, lookup)
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

func modelSelfCheckTaskKey(parts ...any) string {
	switch len(parts) {
	case 2:
		if groupID, ok := parts[0].(int64); ok {
			return fmt.Sprintf("%d:%s", groupID, strings.ToLower(strings.TrimSpace(fmt.Sprint(parts[1]))))
		}
		return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(fmt.Sprint(parts[0]))), parts[1])
	default:
		return strings.ToLower(strings.TrimSpace(fmt.Sprint(parts...)))
	}
}

func (e *gatewayModelSelfCheckProbeExecutor) Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult {
	if account == nil {
		return failedSelfCheckProbeResult(0, modelSelfCheckErrorNoAccount)
	}
	ctx = withModelSelfCheckProbeContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, modelSelfCheckProbeAttemptTimeout)
	defer cancel()
	if _, ok := ctx.Value(modelSelfCheckSessionKey{}).(string); !ok {
		ctx = context.WithValue(ctx, modelSelfCheckSessionKey{}, "self-check-"+uuid.NewString())
	}
	start := time.Now()
	var status int
	var err error
	var duration time.Duration
	var usage modelSelfCheckTokenUsage
	var initialHTTPStatus *int
	retryCount := 0

	switch strings.ToLower(strings.TrimSpace(account.Platform)) {
	case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek:
		status, duration, usage, err = e.probeOpenAI(ctx, account, model)
		var retryErr *modelSelfCheckRetryableHTTPError
		// Diagnostics get at most one recovery attempt within the original
		// deadline. The account may disable retries, but cannot expand this cap.
		if errors.As(err, &retryErr) && account.GetPoolModeRetryCount() > 0 {
			delay := retryErr.SameAccountRetryDelay
			if delay < 250*time.Millisecond {
				delay = 250 * time.Millisecond
			}
			deadline, bounded := ctx.Deadline()
			if (!bounded || time.Until(deadline) > delay) && (retryErr.SameAccountRetryDeadline.IsZero() || time.Until(retryErr.SameAccountRetryDeadline) > delay) {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
				case <-timer.C:
					initialHTTPStatus = optionalHTTPStatus(status)
					retryCount++
					status, _, usage, err = e.probeOpenAI(ctx, account, model)
					duration = time.Since(start)
					if err == nil && status < 400 {
						latency := int(duration.Milliseconds())
						return ModelSelfCheckProbeResult{
							Status: MonitorStatusDegraded, HTTPStatus: optionalHTTPStatus(status), LatencyMs: &latency,
							ErrorCode: "retry_succeeded", Recovered: true,
							RetryCount: retryCount, InitialHTTPStatus: initialHTTPStatus,
							InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
						}
					}
				}
			}
		}
	case PlatformGemini:
		status, duration, usage, err = e.probeGemini(ctx, account, model)
	case PlatformAntigravity:
		status, duration, usage, err = e.probeAntigravity(ctx, account, model)
	case PlatformAnthropic:
		status, duration, usage, err = e.probeAnthropic(ctx, account, model)
	default:
		if account.IsBedrock() {
			status, duration, usage, err = e.probeAnthropic(ctx, account, model)
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
	result := normalizeSelfCheckProbeResult(status, err, latency)
	var failoverErr *UpstreamFailoverError
	result.Transient = errors.As(err, &failoverErr) && failoverErr.RequestScopedTransient
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens
	result.RetryCount = retryCount
	result.InitialHTTPStatus = initialHTTPStatus
	return result
}

func (e *gatewayModelSelfCheckProbeExecutor) probeAnthropic(ctx context.Context, account *Account, model string) (int, time.Duration, modelSelfCheckTokenUsage, error) {
	if e == nil || e.gatewayService == nil {
		return 0, 0, modelSelfCheckTokenUsage{}, fmt.Errorf("anthropic gateway service is not configured")
	}
	body, err := buildAnthropicSelfCheckBody(model)
	if err != nil {
		return 0, 0, modelSelfCheckTokenUsage{}, err
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), domain.PlatformAnthropic)
	if err != nil {
		return 0, 0, modelSelfCheckTokenUsage{}, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/messages", body)
	result, err := e.gatewayService.Forward(ctx, c, account, parsed)
	usage := parseModelSelfCheckTokenUsage(recorder)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, usage, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, usage, modelSelfCheckProbeError(c, err)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeOpenAI(ctx context.Context, account *Account, model string) (int, time.Duration, modelSelfCheckTokenUsage, error) {
	if e == nil || e.openAIGatewayService == nil {
		return 0, 0, modelSelfCheckTokenUsage{}, fmt.Errorf("openai gateway service is not configured")
	}
	selectedProtocol, strictRouting := e.openAISelfCheckUpstreamProtocol(ctx, account, model)
	if strictRouting && selectedProtocol == "" {
		return 0, 0, modelSelfCheckTokenUsage{}, fmt.Errorf("model %q has no confirmed upstream protocol capability", model)
	}
	if selectedProtocol == ModelProtocolAnthropicMessages {
		body, err := buildAnthropicSelfCheckBody(model)
		if err != nil {
			return 0, 0, modelSelfCheckTokenUsage{}, err
		}
		c, recorder := newModelSelfCheckGinContext(ctx, "/v1/messages", body)
		result, err := e.openAIGatewayService.ForwardNativeAnthropicMessages(ctx, c, account, body, model)
		usage := parseModelSelfCheckTokenUsage(recorder)
		if result != nil && result.Duration > 0 {
			return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, usage, modelSelfCheckProbeError(c, err)
		}
		return modelSelfCheckHTTPStatus(c, recorder, err), 0, usage, modelSelfCheckProbeError(c, err)
	}

	body, err := buildChatCompletionsSelfCheckBody(model)
	if err != nil {
		return 0, 0, modelSelfCheckTokenUsage{}, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/chat/completions", body)
	session, _ := ctx.Value(modelSelfCheckSessionKey{}).(string)
	result, err := e.openAIGatewayService.ForwardAsChatCompletionsWithSelectedProtocol(
		ctx,
		c,
		account,
		body,
		session,
		"",
		selectedProtocol,
	)
	usage := parseModelSelfCheckTokenUsage(recorder)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, usage, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, usage, modelSelfCheckProbeError(c, err)
}

// openAISelfCheckUpstreamProtocol keeps the health probe on one exact,
// model-level protocol supported by the account. The boolean reports whether
// strict routing owns the decision: in strict mode an empty protocol fails
// closed, while legacy mode retains the historical account-derived path.
func (e *gatewayModelSelfCheckProbeExecutor) openAISelfCheckUpstreamProtocol(
	ctx context.Context,
	account *Account,
	model string,
) (ModelProtocol, bool) {
	if e == nil || e.openAIGatewayService == nil {
		return "", false
	}
	var routingSettings NativeModelProtocolRoutingSettingReader
	if e.openAIGatewayService.settingService != nil {
		routingSettings = e.openAIGatewayService.settingService
	}
	if !nativeModelProtocolRoutingEnabled(ctx, routingSettings, e.openAIGatewayService.cfg) {
		return "", false
	}
	strictRouting := strictOpenAIAPIKeyProtocolRouting(ModelDeliveryCandidateInput{
		Account:              account,
		NativeRoutingEnabled: true,
	})
	if !strictRouting {
		return "", false
	}
	if e.openAIGatewayService.modelProtocolCapability == nil {
		return "", true
	}
	capabilities, err := e.openAIGatewayService.modelProtocolCapability.List(ctx, account.ID)
	if err != nil {
		return "", true
	}
	for _, protocol := range []ModelProtocol{
		ModelProtocolOpenAIChat,
		ModelProtocolOpenAIResponses,
		ModelProtocolAnthropicMessages,
	} {
		decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
			Account:               account,
			PublicModel:           model,
			ChannelMappedModel:    model,
			GroupPlatform:         account.Platform,
			AllowMessagesDispatch: true,
			InboundProtocol:       protocol,
			NativeRoutingEnabled:  true,
			Capabilities:          capabilities,
		})
		if decision.Eligible && decision.Mode == ModelDeliveryModeNative && decision.UpstreamProtocol == protocol {
			return protocol, true
		}
	}
	return "", true
}

func (e *gatewayModelSelfCheckProbeExecutor) probeGemini(ctx context.Context, account *Account, model string) (int, time.Duration, modelSelfCheckTokenUsage, error) {
	if e == nil || e.geminiCompatService == nil {
		return 0, 0, modelSelfCheckTokenUsage{}, fmt.Errorf("gemini compat service is not configured")
	}
	body, err := buildChatCompletionsSelfCheckBody(model)
	if err != nil {
		return 0, 0, modelSelfCheckTokenUsage{}, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/chat/completions", body)
	result, err := e.geminiCompatService.ForwardAsChatCompletions(ctx, c, account, body)
	usage := parseModelSelfCheckTokenUsage(recorder)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, usage, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, usage, modelSelfCheckProbeError(c, err)
}

func (e *gatewayModelSelfCheckProbeExecutor) probeAntigravity(ctx context.Context, account *Account, model string) (int, time.Duration, modelSelfCheckTokenUsage, error) {
	if e == nil || e.antigravityGatewayService == nil {
		return 0, 0, modelSelfCheckTokenUsage{}, fmt.Errorf("antigravity gateway service is not configured")
	}
	body, err := buildAnthropicSelfCheckBody(model)
	if err != nil {
		return 0, 0, modelSelfCheckTokenUsage{}, err
	}
	c, recorder := newModelSelfCheckGinContext(ctx, "/v1/messages", body)
	result, err := e.antigravityGatewayService.Forward(ctx, c, account, body, false)
	usage := parseModelSelfCheckTokenUsage(recorder)
	if result != nil && result.Duration > 0 {
		return modelSelfCheckHTTPStatus(c, recorder, err), result.Duration, usage, modelSelfCheckProbeError(c, err)
	}
	return modelSelfCheckHTTPStatus(c, recorder, err), 0, usage, modelSelfCheckProbeError(c, err)
}

type modelSelfCheckTokenUsage struct {
	InputTokens  int
	OutputTokens int
}

func parseModelSelfCheckTokenUsage(recorder *httptest.ResponseRecorder) modelSelfCheckTokenUsage {
	if recorder == nil || recorder.Body == nil {
		return modelSelfCheckTokenUsage{}
	}
	return parseModelSelfCheckTokenUsageBody(recorder.Body.Bytes())
}

func parseModelSelfCheckTokenUsageBody(body []byte) modelSelfCheckTokenUsage {
	payload := strings.TrimSpace(string(body))
	if payload == "" {
		return modelSelfCheckTokenUsage{}
	}
	if usage := parseModelSelfCheckTokenUsageJSON(payload); usage.InputTokens > 0 || usage.OutputTokens > 0 {
		return usage
	}
	var usage modelSelfCheckTokenUsage
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		part := parseModelSelfCheckTokenUsageJSON(data)
		usage.InputTokens += part.InputTokens
		usage.OutputTokens += part.OutputTokens
	}
	return usage
}

func parseModelSelfCheckTokenUsageJSON(payload string) modelSelfCheckTokenUsage {
	if !gjson.Valid(payload) {
		return modelSelfCheckTokenUsage{}
	}
	candidates := []struct {
		inputPath  string
		outputPath string
	}{
		{inputPath: "usage.input_tokens", outputPath: "usage.output_tokens"},
		{inputPath: "usage.prompt_tokens", outputPath: "usage.completion_tokens"},
		{inputPath: "usageMetadata.promptTokenCount", outputPath: "usageMetadata.candidatesTokenCount"},
		{inputPath: "usageMetadata.promptTokenCount", outputPath: "usageMetadata.outputTokenCount"},
		{inputPath: "response.usageMetadata.promptTokenCount", outputPath: "response.usageMetadata.candidatesTokenCount"},
		{inputPath: "response.usageMetadata.promptTokenCount", outputPath: "response.usageMetadata.outputTokenCount"},
	}
	for _, candidate := range candidates {
		input := nonNegativeGJSONInt(gjson.Get(payload, candidate.inputPath))
		output := nonNegativeGJSONInt(gjson.Get(payload, candidate.outputPath))
		if input > 0 || output > 0 {
			return modelSelfCheckTokenUsage{InputTokens: input, OutputTokens: output}
		}
	}
	return modelSelfCheckTokenUsage{}
}

func nonNegativeGJSONInt(value gjson.Result) int {
	if !value.Exists() {
		return 0
	}
	n := value.Int()
	if n <= 0 {
		return 0
	}
	return int(n)
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
	if errors.Is(err, errModelSelfCheckIncomplete) {
		return ModelSelfCheckProbeResult{Status: UserModelStatusUnknown, LatencyMs: &latencyMs, HTTPStatus: statusPtr, ErrorCode: modelSelfCheckProbeReasonIncomplete}
	}
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
