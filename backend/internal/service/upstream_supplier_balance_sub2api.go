package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// sub2api's /v1/usage can return key quotas, subscription limits or the account wallet.
// Only the wallet response contains balance; remaining alone is not an account balance.
// https://github.com/Wei-Shaw/sub2api/blob/main/backend/internal/handler/gateway_handler.go
func fetchSub2APISupplierBalance(ctx context.Context, cfg *config.Config, balanceConfig UpstreamSupplierBalanceConfig, apiKey string) (float64, error) {
	if balanceConfig.Provider != "sub2api" {
		return 0, errors.New("unsupported supplier balance provider")
	}
	baseURL, err := normalizeNewAPISupplierBaseURL(balanceConfig.BaseURL)
	if err != nil {
		return 0, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || strings.ContainsAny(apiKey, "\r\n") {
		return 0, errors.New("missing or invalid sub2api API Key")
	}
	endpoint, err := buildNewAPISupplierBalanceURL(baseURL, "/v1/usage")
	if err != nil {
		return 0, err
	}
	endpoint, err = cnValidateProbeURL(cfg, endpoint)
	if err != nil {
		return 0, fmt.Errorf("balance endpoint rejected: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, upstreamSupplierBalanceQueryTimeout)
	defer cancel()
	if shouldValidateNewAPISupplierResolvedIP(cfg) {
		queryCtx = WithHTTPUpstreamPublicHostsOnly(queryCtx)
	}
	client := newUpstreamSupplierBalanceHTTPClient()
	defer client.CloseIdleConnections()
	headers := http.Header{}
	headers.Set("Authorization", formatBearerAuthValue(apiKey))
	payload, err := requestSupplierBalanceJSON(queryCtx, client, endpoint, headers)
	if err != nil {
		return 0, fmt.Errorf("sub2api balance query failed: %w", err)
	}
	if payload["error"] != nil || payload["success"] == false || payload["isValid"] == false {
		message := safeNewAPIUpstreamFailureMessage(payload, newAPISupplierSensitiveHeaderValues(headers))
		if message == "" {
			message = "upstream response indicates failure or an invalid API Key"
		}
		return 0, fmt.Errorf("sub2api balance query failed: %s", message)
	}
	if payload["mode"] == "quota_limited" {
		return 0, errors.New("sub2api returned API Key limits, not the account wallet; use a balance-billed API Key without quota or periodic spending limits")
	}
	if payload["mode"] != "unrestricted" {
		return 0, errors.New("sub2api balance response has missing or unsupported mode; expected unrestricted account wallet data")
	}
	if payload["subscription"] != nil || payload["balance"] == nil {
		return 0, errors.New("sub2api did not return an account wallet balance; subscription allowances cannot be used; use an API Key in a balance-billed group")
	}
	if payload["unit"] != "USD" {
		return 0, errors.New("sub2api balance response has missing or unsupported unit; expected USD")
	}
	balance := flexibleFloat(payload["balance"])
	if balance == nil || !isFiniteFloat(*balance) {
		return 0, errors.New("sub2api balance response has invalid account wallet balance")
	}
	return *balance, nil
}
