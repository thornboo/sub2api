package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	upstreamSupplierBalanceQueryTimeout = 15 * time.Second
	upstreamSupplierBalanceStatusPath   = "/api/status"
	upstreamSupplierBalanceUserSelfPath = "/api/user/self"
)

// fetchNewAPISupplierBalance queries the New API account wallet balance.
//
// The New API wallet value is stored as quota units. The public status endpoint
// supplies quota_per_unit, so the USD credit is quota / quota_per_unit.
func fetchNewAPISupplierBalance(ctx context.Context, cfg *config.Config, balanceConfig UpstreamSupplierBalanceConfig, accessToken string) (float64, error) {
	if strings.TrimSpace(balanceConfig.Provider) != "newapi" {
		return 0, errors.New("unsupported supplier balance provider")
	}
	baseURL, err := normalizeNewAPISupplierBaseURL(balanceConfig.BaseURL)
	if err != nil {
		return 0, err
	}
	token := strings.TrimSpace(accessToken)
	if token == "" {
		return 0, errors.New("missing supplier Access Token")
	}
	if strings.ContainsAny(token, "\r\n") {
		return 0, errors.New("invalid supplier Access Token")
	}

	queryCtx, cancel := context.WithTimeout(ctx, upstreamSupplierBalanceQueryTimeout)
	defer cancel()
	if shouldValidateNewAPISupplierResolvedIP(cfg) {
		queryCtx = WithHTTPUpstreamPublicHostsOnly(queryCtx)
	}

	client := newUpstreamSupplierBalanceHTTPClient()
	defer client.CloseIdleConnections()
	statusURL, err := buildNewAPISupplierBalanceURL(baseURL, upstreamSupplierBalanceStatusPath)
	if err != nil {
		return 0, err
	}
	statusURL, err = cnValidateProbeURL(cfg, statusURL)
	if err != nil {
		return 0, fmt.Errorf("status endpoint rejected: %w", err)
	}
	quotaPerUnit, err := fetchNewAPIQuotaPerUnit(queryCtx, client, statusURL)
	if err != nil {
		return 0, err
	}

	selfURL, err := buildNewAPISupplierBalanceURL(baseURL, upstreamSupplierBalanceUserSelfPath)
	if err != nil {
		return 0, err
	}
	selfURL, err = cnValidateProbeURL(cfg, selfURL)
	if err != nil {
		return 0, fmt.Errorf("user endpoint rejected: %w", err)
	}
	quota, err := fetchNewAPIAccountQuota(queryCtx, client, selfURL, token, balanceConfig.UserID)
	if err != nil {
		return 0, err
	}
	balance := quota / quotaPerUnit
	if !isFiniteFloat(balance) {
		return 0, errors.New("supplier balance response produced an invalid balance")
	}
	return balance, nil
}

func newUpstreamSupplierBalanceHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Timeout: upstreamSupplierBalanceQueryTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				if HTTPUpstreamPublicHostsOnly(ctx) {
					// Dial the validated IP directly so DNS cannot change the target
					// between the private-host check and the actual connection.
					return safeDialContext(ctx, network, address)
				}
				return dialer.DialContext(ctx, network, address)
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func shouldValidateNewAPISupplierResolvedIP(cfg *config.Config) bool {
	if cfg == nil || !cfg.Security.URLAllowlist.Enabled {
		return false
	}
	return !cfg.Security.URLAllowlist.AllowPrivateHosts
}

func normalizeNewAPISupplierBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", errors.New("supplier website is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid supplier website")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("supplier website must be an HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("supplier website must not contain credentials, query or fragment")
	}
	return parsed.String(), nil
}

func buildNewAPISupplierBalanceURL(baseURL, endpointPath string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid supplier website")
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	endpointPath = "/" + strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	parsed.Path = basePath + endpointPath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func fetchNewAPIQuotaPerUnit(ctx context.Context, client *http.Client, endpoint string) (float64, error) {
	payload, err := requestNewAPISupplierJSON(ctx, client, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("status query failed: %w", err)
	}
	data := newAPISupplierResponseData(payload)
	if data == nil {
		return 0, errors.New("status response missing data")
	}
	quotaPerUnit := flexibleFloat(data["quota_per_unit"])
	if quotaPerUnit == nil {
		return 0, errors.New("status response missing quota_per_unit")
	}
	if *quotaPerUnit <= 0 || !isFiniteFloat(*quotaPerUnit) {
		return 0, errors.New("status response has invalid quota_per_unit")
	}
	return *quotaPerUnit, nil
}

func fetchNewAPIAccountQuota(ctx context.Context, client *http.Client, endpoint, accessToken string, userID int64) (float64, error) {
	headers := http.Header{}
	headers.Set("Authorization", formatBearerAuthValue(accessToken))
	if userID > 0 {
		headers.Set("New-Api-User", strconv.FormatInt(userID, 10))
	}
	payload, err := requestNewAPISupplierJSON(ctx, client, endpoint, headers)
	if err != nil {
		return 0, fmt.Errorf("account query failed: %w", err)
	}
	data := newAPISupplierResponseData(payload)
	if data == nil {
		return 0, errors.New("account response missing data")
	}
	quota := flexibleFloat(data["quota"])
	if quota == nil {
		return 0, errors.New("account response missing quota")
	}
	if !isFiniteFloat(*quota) {
		return 0, errors.New("account response has invalid quota")
	}
	return *quota, nil
}

func requestNewAPISupplierJSON(ctx context.Context, client *http.Client, endpoint string, headers http.Header) (map[string]any, error) {
	payload, err := requestSupplierBalanceJSON(ctx, client, endpoint, headers)
	if err != nil {
		return nil, err
	}
	if success, ok := payload["success"].(bool); !ok || !success {
		if message := safeNewAPIUpstreamFailureMessage(payload, newAPISupplierSensitiveHeaderValues(headers)); message != "" {
			return nil, fmt.Errorf("upstream response indicates failure: %s", message)
		}
		return nil, errors.New("upstream response indicates failure")
	}
	return payload, nil
}

// The transport, size limit and error redaction apply to all supplier providers;
// success envelopes and balance units are interpreted by each provider.
func requestSupplierBalanceJSON(ctx context.Context, client *http.Client, endpoint string, headers http.Header) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("failed to build upstream request")
	}
	if err := validateNewAPISupplierRequestHost(req); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, classifyNewAPISupplierRequestError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readUpstreamResponseBodyLimited(resp.Body, upstreamBalanceResponseLimit)
	if err != nil {
		detail := classifyNewAPISupplierRequestError(err)
		if errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			detail = errors.New("upstream response is too large")
		}
		return nil, fmt.Errorf("upstream HTTP %d response: %w", resp.StatusCode, detail)
	}
	sensitiveValues := newAPISupplierSensitiveHeaderValues(headers)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if message := safeNewAPIUpstreamFailureMessageFromBody(body, sensitiveValues); message != "" {
			return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, message)
		}
		return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errors.New("failed to parse upstream response")
	}
	return payload, nil
}

func classifyNewAPISupplierRequestError(err error) error {
	if err == nil {
		return errors.New("upstream request failed")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("upstream request timed out")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("upstream request timed out")
	}
	return errors.New("upstream request failed")
}

func safeNewAPIUpstreamFailureMessageFromBody(body []byte, sensitiveValues []string) string {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return ""
	}
	return safeNewAPIUpstreamFailureMessage(payload, sensitiveValues)
}

func safeNewAPIUpstreamFailureMessage(payload map[string]any, sensitiveValues []string) string {
	for _, key := range []string{"message", "error", "msg"} {
		raw, ok := payload[key].(string)
		if details, nested := payload[key].(map[string]any); nested {
			raw, ok = details["message"].(string)
		}
		if !ok {
			continue
		}
		message := sanitizeNewAPIUpstreamFailureMessage(raw, sensitiveValues)
		if message != "" {
			return message
		}
	}
	return ""
}

func sanitizeNewAPIUpstreamFailureMessage(raw string, sensitiveValues []string) string {
	message := strings.TrimSpace(raw)
	if message == "" || strings.ContainsAny(message, "<>") {
		return ""
	}
	for _, sensitiveValue := range sensitiveValues {
		sensitiveValue = strings.TrimSpace(sensitiveValue)
		if sensitiveValue == "" {
			continue
		}
		message = strings.ReplaceAll(message, sensitiveValue, "[redacted]")
	}
	message = strings.Join(strings.Fields(message), " ")
	const maxMessageLength = 160
	runes := []rune(message)
	if len(runes) > maxMessageLength {
		message = string(runes[:maxMessageLength]) + "..."
	}
	return message
}

func newAPISupplierSensitiveHeaderValues(headers http.Header) []string {
	authValue := strings.TrimSpace(headers.Get("Authorization"))
	if authValue == "" {
		return nil
	}
	values := []string{authValue}
	if token := strings.TrimSpace(strings.TrimPrefix(authValue, "Bearer ")); token != "" && token != authValue {
		values = append(values, token)
	}
	return values
}

func validateNewAPISupplierRequestHost(req *http.Request) error {
	if req == nil || req.URL == nil {
		return errors.New("request url is nil")
	}
	if !HTTPUpstreamPublicHostsOnly(req.Context()) {
		return nil
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if host == "" {
		return errors.New("request host is empty")
	}
	if err := urlvalidator.ValidateResolvedIP(host); err != nil {
		return errors.New("upstream host rejected by URL security policy")
	}
	return nil
}

func newAPISupplierResponseData(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil
	}
	return data
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
