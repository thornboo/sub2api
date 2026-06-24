package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	UpstreamBalanceProviderSub2API          = "sub2api"
	UpstreamBalanceProviderNewAPICompatible = "new_api_compatible"
	UpstreamBalanceDefaultProvider          = UpstreamBalanceProviderSub2API
	UpstreamBalanceDefaultEndpoint          = "/v1/usage"
	UpstreamBalanceSub2APIProfileEndpoint   = "/api/v1/user/profile"
	UpstreamBalanceNewAPIDefaultEndpoint    = "/api/usage/token/"
	UpstreamBalanceSnapshotExtraKey         = "upstream_balance_snapshot"
	UpstreamBalanceAuthModeAccountAPIKey    = "account_api_key"
	UpstreamBalanceAuthModeBearerToken      = "bearer_token"
	UpstreamBalanceAuthModeCustomHeader     = "custom_header"

	upstreamBalanceQueryEnabledExtraKey = "upstream_balance_query_enabled"
	upstreamBalanceProviderExtraKey     = "upstream_balance_provider"
	upstreamBalanceEndpointExtraKey     = "upstream_balance_endpoint"
	upstreamBalanceAuthModeExtraKey     = "upstream_balance_auth_mode"
	upstreamBalanceAuthHeaderExtraKey   = "upstream_balance_auth_header"
	upstreamBalanceAuthTokenCredKey     = "upstream_balance_auth_token"
	upstreamBalanceNewAPIQuotaPerUSD    = 500000
	upstreamBalanceResponseLimit        = 1 << 20
)

type UpstreamBalanceConfig struct {
	Enabled            bool   `json:"enabled"`
	Provider           string `json:"provider"`
	Endpoint           string `json:"endpoint"`
	AuthMode           string `json:"auth_mode"`
	AuthHeader         string `json:"auth_header,omitempty"`
	TokenCredentialKey string `json:"-"`
}

type UpstreamBalanceSnapshot struct {
	Provider     string     `json:"provider"`
	Status       string     `json:"status"`
	Endpoint     string     `json:"endpoint"`
	RawUnit      string     `json:"raw_unit,omitempty"`
	RawAvailable *float64   `json:"raw_available,omitempty"`
	RawUsed      *float64   `json:"raw_used,omitempty"`
	RawGranted   *float64   `json:"raw_granted,omitempty"`
	AvailableUSD *float64   `json:"available_usd,omitempty"`
	Unlimited    bool       `json:"unlimited"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	FetchedAt    time.Time  `json:"fetched_at"`
	StatusCode   int        `json:"status_code,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type newAPIUsageTokenResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func ResolveUpstreamBalanceConfig(extra map[string]any) UpstreamBalanceConfig {
	cfg := UpstreamBalanceConfig{
		Provider:           UpstreamBalanceDefaultProvider,
		TokenCredentialKey: upstreamBalanceAuthTokenCredKey,
	}
	if extra == nil {
		cfg.Endpoint = defaultUpstreamBalanceEndpoint(cfg.Provider)
		cfg.AuthMode = defaultUpstreamBalanceAuthMode(cfg.Provider)
		return cfg
	}
	if v, ok := extra[upstreamBalanceQueryEnabledExtraKey].(bool); ok {
		cfg.Enabled = v
	}
	if provider, ok := extra[upstreamBalanceProviderExtraKey].(string); ok && strings.TrimSpace(provider) != "" {
		switch strings.TrimSpace(provider) {
		case UpstreamBalanceProviderSub2API, UpstreamBalanceProviderNewAPICompatible:
			cfg.Provider = strings.TrimSpace(provider)
		}
	}
	cfg.Endpoint = defaultUpstreamBalanceEndpoint(cfg.Provider)
	cfg.AuthMode = defaultUpstreamBalanceAuthMode(cfg.Provider)
	if endpoint, ok := extra[upstreamBalanceEndpointExtraKey].(string); ok && strings.TrimSpace(endpoint) != "" {
		cfg.Endpoint = strings.TrimSpace(endpoint)
	}
	if authMode, ok := extra[upstreamBalanceAuthModeExtraKey].(string); ok {
		switch strings.TrimSpace(authMode) {
		case UpstreamBalanceAuthModeAccountAPIKey, UpstreamBalanceAuthModeBearerToken, UpstreamBalanceAuthModeCustomHeader:
			cfg.AuthMode = strings.TrimSpace(authMode)
		}
	}
	if authHeader, ok := extra[upstreamBalanceAuthHeaderExtraKey].(string); ok && strings.TrimSpace(authHeader) != "" {
		cfg.AuthHeader = strings.TrimSpace(authHeader)
	}
	if cfg.Provider == UpstreamBalanceProviderSub2API &&
		cfg.AuthMode == UpstreamBalanceAuthModeAccountAPIKey &&
		normalizeEndpointPathForCompare(cfg.Endpoint) == UpstreamBalanceSub2APIProfileEndpoint {
		cfg.Endpoint = UpstreamBalanceDefaultEndpoint
	}
	return cfg
}

func defaultUpstreamBalanceEndpoint(provider string) string {
	switch provider {
	case UpstreamBalanceProviderNewAPICompatible:
		return UpstreamBalanceNewAPIDefaultEndpoint
	default:
		return UpstreamBalanceDefaultEndpoint
	}
}

func defaultUpstreamBalanceAuthMode(provider string) string {
	switch provider {
	case UpstreamBalanceProviderNewAPICompatible:
		return UpstreamBalanceAuthModeAccountAPIKey
	default:
		return UpstreamBalanceAuthModeAccountAPIKey
	}
}

func (s *AccountTestService) FetchUpstreamBalance(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
	if s == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "UPSTREAM_BALANCE_SERVICE_UNAVAILABLE", "upstream balance service is unavailable")
	}
	if account == nil || account.ID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT", "invalid account")
	}
	if account.Type != AccountTypeAPIKey {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_UNSUPPORTED_ACCOUNT_TYPE", "upstream balance query only supports API key accounts")
	}
	cfg := ResolveUpstreamBalanceConfig(account.Extra)
	if !cfg.Enabled {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_DISABLED", "upstream balance query is not enabled for this account")
	}
	switch cfg.Provider {
	case UpstreamBalanceProviderSub2API:
		return s.fetchSub2APIBalance(ctx, account, cfg)
	case UpstreamBalanceProviderNewAPICompatible:
		return s.fetchNewAPICompatibleBalance(ctx, account, cfg)
	default:
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_UNSUPPORTED_PROVIDER", "unsupported upstream balance provider")
	}
}

func (s *AccountTestService) fetchSub2APIBalance(ctx context.Context, account *Account, cfg UpstreamBalanceConfig) (*UpstreamBalanceSnapshot, error) {
	snapshot, body := s.fetchUpstreamBalanceJSON(ctx, account, cfg, "usd")
	if snapshot.Status != "ok" {
		return snapshot, nil
	}
	if err := fillSub2APIBalanceSnapshot(snapshot, body); err != nil {
		snapshot.Status = "error"
		snapshot.Error = sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}
	return snapshot, nil
}

func (s *AccountTestService) fetchNewAPICompatibleBalance(ctx context.Context, account *Account, cfg UpstreamBalanceConfig) (*UpstreamBalanceSnapshot, error) {
	snapshot, body := s.fetchUpstreamBalanceJSON(ctx, account, cfg, "quota")
	if snapshot.Status != "ok" {
		return snapshot, nil
	}
	if err := fillNewAPIBalanceSnapshot(snapshot, body); err != nil {
		snapshot.Status = "error"
		snapshot.Error = sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}
	return snapshot, nil
}

func (s *AccountTestService) fetchUpstreamBalanceJSON(ctx context.Context, account *Account, cfg UpstreamBalanceConfig, rawUnit string) (*UpstreamBalanceSnapshot, []byte) {
	now := time.Now().UTC()
	snapshot := &UpstreamBalanceSnapshot{
		Provider:  cfg.Provider,
		Status:    "error",
		RawUnit:   rawUnit,
		FetchedAt: now,
	}

	if s.httpUpstream == nil {
		snapshot.Error = "upstream HTTP client is unavailable"
		return snapshot, nil
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		snapshot.Error = "missing base_url"
		return snapshot, nil
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		snapshot.Error = "invalid base_url: " + sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}
	endpointURL, err := buildUpstreamBalanceURL(normalizedBaseURL, cfg.Endpoint)
	if err != nil {
		snapshot.Error = sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}
	snapshot.Endpoint = endpointURL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		snapshot.Error = "failed to create request"
		return snapshot, nil
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Accept", "application/json")
	if err := applyUpstreamBalanceAuth(req, account, cfg); err != nil {
		snapshot.Error = sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		tlsProfile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		snapshot.Error = sanitizeUpstreamErrorMessage(err.Error())
		return snapshot, nil
	}
	defer func() { _ = resp.Body.Close() }()

	snapshot.StatusCode = resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceResponseLimit))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := strings.TrimSpace(extractUpstreamErrorMessage(body))
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		snapshot.Error = fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, truncateString(sanitizeUpstreamErrorMessage(msg), 512))
		return snapshot, nil
	}
	snapshot.Status = "ok"
	snapshot.Error = ""
	return snapshot, body
}

func applyUpstreamBalanceAuth(req *http.Request, account *Account, cfg UpstreamBalanceConfig) error {
	tokenCredKey := cfg.TokenCredentialKey
	if tokenCredKey == "" {
		tokenCredKey = upstreamBalanceAuthTokenCredKey
	}
	switch cfg.AuthMode {
	case "", UpstreamBalanceAuthModeAccountAPIKey:
		apiKey := strings.TrimSpace(account.GetCredential("api_key"))
		if apiKey == "" {
			return errors.New("missing api_key")
		}
		req.Header.Set("Authorization", formatBearerAuthValue(apiKey))
		return nil
	case UpstreamBalanceAuthModeBearerToken:
		token := strings.TrimSpace(account.GetCredential(tokenCredKey))
		if token == "" {
			return fmt.Errorf("missing %s", tokenCredKey)
		}
		req.Header.Set("Authorization", formatBearerAuthValue(token))
		return nil
	case UpstreamBalanceAuthModeCustomHeader:
		token := strings.TrimSpace(account.GetCredential(tokenCredKey))
		if token == "" {
			return fmt.Errorf("missing %s", tokenCredKey)
		}
		header := strings.TrimSpace(cfg.AuthHeader)
		if header == "" {
			return errors.New("missing upstream balance auth header")
		}
		if !isSafeHTTPHeaderName(header) {
			return errors.New("invalid upstream balance auth header")
		}
		req.Header.Set(header, token)
		return nil
	default:
		return errors.New("unsupported upstream balance auth mode")
	}
}

func formatBearerAuthValue(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func isSafeHTTPHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r <= 32 || r >= 127 {
			return false
		}
		switch r {
		case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}':
			return false
		}
	}
	return true
}

func buildUpstreamBalanceURL(baseURL, endpoint string) (string, error) {
	parsedBase, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return "", fmt.Errorf("invalid base_url")
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = UpstreamBalanceDefaultEndpoint
	}
	if strings.HasPrefix(strings.ToLower(endpoint), "http://") || strings.HasPrefix(strings.ToLower(endpoint), "https://") {
		return "", fmt.Errorf("balance endpoint must be a path, not an absolute URL")
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Path == "" {
		return "", fmt.Errorf("invalid balance endpoint")
	}

	parsedBase.Path = parsedEndpoint.Path
	parsedBase.RawQuery = parsedEndpoint.RawQuery
	parsedBase.Fragment = ""
	return parsedBase.String(), nil
}

func fillSub2APIBalanceSnapshot(snapshot *UpstreamBalanceSnapshot, body []byte) error {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("failed to parse balance response: %w", err)
	}
	if msg := responseEnvelopeError(payload); msg != "" {
		return errors.New(msg)
	}
	data := responseData(payload)
	if len(data) == 0 {
		return fmt.Errorf("balance response missing data")
	}

	balance := firstFlexibleFloat(data, "balance", "remaining", "available_balance", "remaining_balance", "quota_remaining")
	if balance == nil {
		balance = firstNestedFlexibleFloat(data, []string{"quota", "remaining"}, []string{"subscription", "remaining"})
	}
	if balance == nil {
		return fmt.Errorf("balance response missing balance")
	}
	snapshot.RawAvailable = balance
	snapshot.AvailableUSD = balance
	if used := firstFlexibleFloat(data, "balance_used", "used_balance", "quota_used", "used"); used != nil {
		snapshot.RawUsed = used
	}
	if granted := firstFlexibleFloat(data, "total_recharged", "total_balance", "limit"); granted != nil {
		snapshot.RawGranted = granted
	} else if granted := firstNestedFlexibleFloat(data, []string{"quota", "limit"}); granted != nil {
		snapshot.RawGranted = granted
	}
	return nil
}

func normalizeEndpointPathForCompare(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.Path != "" {
		endpoint = parsed.Path
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	if endpoint != "/" {
		endpoint = strings.TrimRight(endpoint, "/")
	}
	return endpoint
}

func fillNewAPIBalanceSnapshot(snapshot *UpstreamBalanceSnapshot, body []byte) error {
	var payload newAPIUsageTokenResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("failed to parse balance response: %w", err)
	}
	if payload.Message != "" && !payload.Success && len(payload.Data) == 0 {
		return errors.New(payload.Message)
	}
	data := payload.Data
	if len(data) == 0 {
		return fmt.Errorf("balance response missing data")
	}

	snapshot.RawGranted = flexibleFloat(data["total_granted"])
	snapshot.RawUsed = flexibleFloat(data["total_used"])
	snapshot.RawAvailable = flexibleFloat(data["total_available"])
	if unlimited, ok := data["unlimited_quota"].(bool); ok {
		snapshot.Unlimited = unlimited
	}
	snapshot.ExpiresAt = flexibleTime(data["expires_at"])
	if snapshot.RawAvailable != nil && !snapshot.Unlimited {
		availableUSD := *snapshot.RawAvailable / upstreamBalanceNewAPIQuotaPerUSD
		snapshot.AvailableUSD = &availableUSD
	}
	return nil
}

func responseData(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if data, ok := payload["data"].(map[string]any); ok {
		return data
	}
	return payload
}

func responseEnvelopeError(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if code := flexibleFloat(payload["code"]); code != nil && *code != 0 {
		if msg := strings.TrimSpace(upstreamBalanceString(payload["message"])); msg != "" {
			return msg
		}
		return fmt.Sprintf("upstream returned code %s", formatFloatForError(*code))
	}
	if success, ok := payload["success"].(bool); ok && !success {
		if msg := strings.TrimSpace(upstreamBalanceString(payload["message"])); msg != "" {
			return msg
		}
		if msg := strings.TrimSpace(extractNestedErrorMessage(payload)); msg != "" {
			return msg
		}
		return "upstream returned success=false"
	}
	return ""
}

func firstFlexibleFloat(data map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value := flexibleFloat(data[key]); value != nil {
			return value
		}
	}
	return nil
}

func firstNestedFlexibleFloat(data map[string]any, paths ...[]string) *float64 {
	for _, path := range paths {
		current := any(data)
		for _, key := range path {
			next, ok := current.(map[string]any)
			if !ok {
				current = nil
				break
			}
			current = next[key]
		}
		if value := flexibleFloat(current); value != nil {
			return value
		}
	}
	return nil
}

func upstreamBalanceString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func extractNestedErrorMessage(payload map[string]any) string {
	if raw, ok := payload["error"].(map[string]any); ok {
		if msg := upstreamBalanceString(raw["message"]); msg != "" {
			return msg
		}
	}
	return ""
}

func formatFloatForError(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func flexibleFloat(value any) *float64 {
	switch v := value.(type) {
	case nil:
		return nil
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return &parsed
		}
	case float64:
		return &v
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return &parsed
		}
	}
	return nil
}

func flexibleTime(value any) *time.Time {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return &parsed
		}
		if unix, err := strconv.ParseInt(raw, 10, 64); err == nil && unix > 0 {
			parsed := unixTimestampToTime(unix)
			return &parsed
		}
	case json.Number:
		if unix, err := v.Int64(); err == nil && unix > 0 {
			parsed := unixTimestampToTime(unix)
			return &parsed
		}
	case float64:
		if v > 0 {
			parsed := unixTimestampToTime(int64(v))
			return &parsed
		}
	}
	return nil
}

func unixTimestampToTime(value int64) time.Time {
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}
