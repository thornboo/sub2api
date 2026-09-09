package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// SoleAPI documents remaining as the available balance including rewards, in Credits:
// https://soleapi.com/zh/docs/guides/billing
// The API key authenticates the query; its individual spending limit is not used as a balance.
func fetchSoleAPISupplierBalance(ctx context.Context, cfg *config.Config, balanceConfig UpstreamSupplierBalanceConfig, apiKey string) (float64, error) {
	if balanceConfig.Provider != "soleapi" {
		return 0, errors.New("unsupported supplier balance provider")
	}
	baseURL, err := normalizeNewAPISupplierBaseURL(balanceConfig.BaseURL)
	if err != nil {
		return 0, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || strings.ContainsAny(apiKey, "\r\n") {
		return 0, errors.New("missing or invalid SoleAPI API Key")
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
		return 0, fmt.Errorf("SoleAPI balance query failed: %w", err)
	}
	if payload["error"] != nil || payload["success"] == false {
		message := safeNewAPIUpstreamFailureMessage(payload, newAPISupplierSensitiveHeaderValues(headers))
		if message == "" {
			message = "upstream response indicates failure"
		}
		return 0, fmt.Errorf("SoleAPI balance query failed: %s", message)
	}
	if payload["unit"] != "Credits" {
		return 0, errors.New("SoleAPI balance response has missing or unsupported unit; expected Credits")
	}
	remaining := flexibleFloat(payload["remaining"])
	if remaining == nil || !isFiniteFloat(*remaining) {
		return 0, errors.New("SoleAPI balance response has missing or invalid remaining")
	}
	return *remaining, nil
}
