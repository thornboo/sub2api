package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const openAINativeProtocolUnavailableReason GatewayFailureReason = "openai_native_protocol_unavailable"

var ErrNativeAnthropicStreamErrorForwarded = errors.New("native Anthropic stream error already forwarded")

// IsNativeProtocolUnavailable reports endpoint-specific incompatibility. The
// account may still be healthy and usable through the existing compatibility path.
func (e *UpstreamFailoverError) IsNativeProtocolUnavailable() bool {
	return e != nil && e.Reason == openAINativeProtocolUnavailableReason
}

// SelectAccountWithSchedulerForNativeProtocol preserves the current scheduler's
// ordering while excluding candidates that cannot form the requested native route.
func (s *OpenAIGatewayService) SelectAccountWithSchedulerForNativeProtocol(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	channelMappedModel string,
	excludedIDs map[int64]struct{},
	protocol ModelProtocol,
	platform string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, ModelDeliveryDecision, error) {
	decision := OpenAIAccountScheduleDecision{}
	delivery := ModelDeliveryDecision{InboundProtocol: protocol}
	if s == nil || s.modelProtocolCapability == nil {
		return nil, decision, delivery, ErrNoAvailableAccounts
	}
	var routingSettings NativeModelProtocolRoutingSettingReader
	if s.settingService != nil {
		routingSettings = s.settingService
	}
	if !nativeModelProtocolRoutingEnabled(ctx, routingSettings, s.cfg) {
		return nil, decision, delivery, ErrNoAvailableAccounts
	}

	effectiveExcluded := cloneExcludedAccountIDs(excludedIDs)
	for {
		selection, nextDecision, err := s.selectAccountWithSchedulerForResolvedModel(
			ctx,
			groupID,
			"",
			sessionHash,
			requestedModel,
			channelMappedModel,
			effectiveExcluded,
			OpenAIUpstreamTransportAny,
			"",
			false,
			false,
			true,
			platform,
		)
		decision = nextDecision
		if err != nil || selection == nil || selection.Account == nil {
			return selection, decision, delivery, err
		}

		account := selection.Account
		capabilities, capabilityErr := s.modelProtocolCapability.List(ctx, account.ID)
		if capabilityErr != nil {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			return nil, decision, delivery, fmt.Errorf("%w: %v", ErrModelProtocolCapabilityUnavailable, capabilityErr)
		}
		delivery = EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
			Account:               account,
			PublicModel:           requestedModel,
			ChannelMappedModel:    channelMappedModel,
			GroupPlatform:         account.Platform,
			AllowMessagesDispatch: true,
			InboundProtocol:       protocol,
			NativeRoutingEnabled:  true,
			Capabilities:          capabilities,
		})
		if delivery.Eligible && delivery.Mode == ModelDeliveryModeNative && delivery.UpstreamProtocol == protocol {
			s.bindProtocolDeliverySticky(ctx, groupID, sessionHash, selection)
			return selection, decision, delivery, nil
		}

		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		if effectiveExcluded == nil {
			effectiveExcluded = make(map[int64]struct{})
		}
		if _, exists := effectiveExcluded[account.ID]; exists {
			return nil, decision, delivery, ErrNoAvailableAccounts
		}
		effectiveExcluded[account.ID] = struct{}{}
	}
}

// ForwardNativeAnthropicMessages sends an Anthropic Messages body unchanged
// except for the final account/channel model mapping.
func (s *OpenAIGatewayService) ForwardNativeAnthropicMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil || !account.IsOpenAI() || account.Type != AccountTypeAPIKey {
		return nil, errors.New("native Anthropic Messages requires an OpenAI API key account")
	}
	requestModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if requestModel == "" {
		return nil, errors.New("model is required")
	}
	originalModel = strings.TrimSpace(originalModel)
	if originalModel == "" {
		originalModel = requestModel
	}
	normalizedModel := NormalizeOpenAICompatRequestedModel(requestModel)
	// The protocol-specific delivery mapping (explicit channel mapping first,
	// otherwise the group's legacy Messages dispatch mapping) is already in the
	// body. Native forwarding only applies the selected account's final mapping.
	billingModel := resolveOpenAIForwardModel(account, normalizedModel, "")
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	body = s.ReplaceModelInBody(body, upstreamModel)
	stream := gjson.GetBytes(body, "stream").Bool()

	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, "/v1/messages")
	resp, err := s.sendNativeAnthropicMessagesRequest(ctx, c, account, targetURL, body, stream, apiKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if isNativeProtocolUnavailableResponse(resp.StatusCode, respBody, upstreamMsg) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "native_protocol_unavailable",
				Message:            upstreamMsg,
			})
			failoverErr := newOpenAIUpstreamFailoverError(resp.StatusCode, resp.Header, respBody, upstreamMsg, false)
			failoverErr.Reason = openAINativeProtocolUnavailableReason
			failoverErr.NextAccountAction = NextAccountRetry
			return nil, failoverErr
		}
		if failoverErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); failoverErr != nil {
			return nil, failoverErr
		}
		writeAnthropicPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/json"
		}
		c.Data(resp.StatusCode, contentType, respBody)
		return nil, fmt.Errorf("native Anthropic Messages upstream returned HTTP %d", resp.StatusCode)
	}

	if stream {
		return s.streamNativeAnthropicMessages(c, resp, originalModel, billingModel, upstreamModel, startTime)
	}
	return s.bufferNativeAnthropicMessages(c, resp, originalModel, billingModel, upstreamModel, startTime)
}

func (s *OpenAIGatewayService) sendNativeAnthropicMessagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	targetURL string,
	body []byte,
	stream bool,
	apiKey string,
) (*http.Response, error) {
	upstreamCtx, release := detachUpstreamContext(ctx)
	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	release()
	if err != nil {
		return nil, fmt.Errorf("build native Anthropic Messages request: %w", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if !allowedHeaders[strings.ToLower(strings.TrimSpace(key))] {
				continue
			}
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	req.Header.Del("Authorization")
	req.Header.Del("x-api-key")
	req.Header.Del("Cookie")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if userAgent := strings.TrimSpace(account.GetOpenAIUserAgent()); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	return resp, nil
}

func (s *OpenAIGatewayService) bufferNativeAnthropicMessages(
	c *gin.Context,
	resp *http.Response,
	originalModel, billingModel, upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, anthropicTooLargeError)
	if err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, newOpenAIUpstreamFailoverError(resp.StatusCode, resp.Header, body, "invalid JSON response", false)
	}
	usage := openAIUsageFromClaudeUsage(parseClaudeUsageFromResponseBody(body))
	if originalModel != upstreamModel {
		if rewritten, rewriteErr := sjson.SetBytes(body, "model", originalModel); rewriteErr == nil {
			body = rewritten
		}
	}
	writeAnthropicPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(http.StatusOK, contentType, body)
	return &OpenAIForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		ResponseID:       gjson.GetBytes(body, "id").String(),
		Usage:            usage,
		Model:            originalModel,
		BillingModel:     billingModel,
		UpstreamModel:    upstreamModel,
		UpstreamEndpoint: "/v1/messages",
		Stream:           false,
		ResponseHeaders:  resp.Header.Clone(),
		Duration:         time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) streamNativeAnthropicMessages(
	c *gin.Context,
	resp *http.Response,
	originalModel, billingModel, upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	writeHeaders := s.newStreamHeaderWriter(c, resp.Header)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}
	usage := OpenAIUsage{}
	var firstTokenMs *int
	clientDisconnected := false
	sawTerminal := false
	sawTerminalError := false
	responseID := ""
	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)
	for scanner.Scan() {
		line := scanner.Text()
		if data, isData := extractAnthropicSSEDataLine(line); isData {
			trimmed := strings.TrimSpace(data)
			if firstTokenMs == nil && trimmed != "" && trimmed != "[DONE]" {
				value := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &value
			}
			mergeNativeAnthropicSSEUsage(data, &usage)
			if responseID == "" {
				responseID = gjson.Get(data, "message.id").String()
			}
			if anthropicStreamEventIsTerminal("", trimmed) {
				sawTerminal = true
			}
			if anthropicStreamEventIsError("", trimmed) {
				sawTerminalError = true
			}
			line = rewriteNativeAnthropicSSEModel(line, data, originalModel, upstreamModel)
		} else if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "event:") {
			eventName := strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			if anthropicStreamEventIsTerminal(eventName, "") {
				sawTerminal = true
			}
			if anthropicStreamEventIsError(eventName, "") {
				sawTerminalError = true
			}
		}
		if !clientDisconnected {
			writeHeaders()
			if _, err := io.WriteString(c.Writer, line+"\n"); err != nil {
				clientDisconnected = true
			} else if line == "" {
				flusher.Flush()
			}
		}
	}
	result := &OpenAIForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		ResponseID:       responseID,
		Usage:            usage,
		Model:            originalModel,
		BillingModel:     billingModel,
		UpstreamModel:    upstreamModel,
		UpstreamEndpoint: "/v1/messages",
		Stream:           true,
		ResponseHeaders:  resp.Header.Clone(),
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnected,
	}
	if err := scanner.Err(); err != nil {
		message := fmt.Sprintf("native Anthropic Messages stream read failed: %v", err)
		return result, newOpenAIUpstreamFailoverError(http.StatusBadGateway, resp.Header, nil, message, false)
	}
	if sawTerminalError {
		return result, ErrNativeAnthropicStreamErrorForwarded
	}
	if !sawTerminal {
		return result, newOpenAIUpstreamFailoverError(http.StatusBadGateway, resp.Header, nil, "native Anthropic Messages stream ended without message_stop", false)
	}
	if !clientDisconnected {
		flusher.Flush()
	}
	return result, nil
}

func isNativeProtocolUnavailableResponse(statusCode int, body []byte, message string) bool {
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(message + " " + string(body)))
	for _, marker := range []string{
		"endpoint not supported",
		"endpoint is not supported",
		"unsupported endpoint",
		"does not support this endpoint",
		"unknown endpoint",
		"no route for",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func rewriteNativeAnthropicSSEModel(line, data, originalModel, upstreamModel string) string {
	if originalModel == "" || originalModel == upstreamModel || gjson.Get(data, "type").String() != "message_start" || !gjson.Get(data, "message.model").Exists() {
		return line
	}
	rewritten, err := sjson.Set(data, "message.model", originalModel)
	if err != nil {
		return line
	}
	return line[:len(line)-len(data)] + rewritten
}

func openAIUsageFromClaudeUsage(usage *ClaudeUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	return OpenAIUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
}

func mergeNativeAnthropicSSEUsage(data string, usage *OpenAIUsage) {
	if usage == nil || strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
		return
	}
	parsed := gjson.Parse(data)
	node := parsed.Get("usage")
	if parsed.Get("type").String() == "message_start" {
		node = parsed.Get("message.usage")
	}
	if !node.Exists() {
		return
	}
	if value := node.Get("input_tokens"); value.Exists() {
		usage.InputTokens = int(value.Int())
	}
	if value := node.Get("output_tokens"); value.Exists() {
		usage.OutputTokens = int(value.Int())
	}
	if value := node.Get("cache_creation_input_tokens"); value.Exists() {
		usage.CacheCreationInputTokens = int(value.Int())
	}
	if value := node.Get("cache_read_input_tokens"); value.Exists() {
		usage.CacheReadInputTokens = int(value.Int())
	} else if value := node.Get("cached_tokens"); value.Exists() {
		usage.CacheReadInputTokens = int(value.Int())
	}
}
