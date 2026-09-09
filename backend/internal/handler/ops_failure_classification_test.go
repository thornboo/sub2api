package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClassifyOpsFailureV2SeparatesAttributionFromSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		prepare      func(*gin.Context)
		errType      string
		message      string
		code         string
		status       int
		eventScope   string
		domain       string
		category     string
		reason       string
		owner        string
		customerSeen bool
		slaImpact    bool
	}{
		{
			name:    "platform managed pool exhaustion counts against SLA",
			prepare: func(c *gin.Context) { markOpsRoutingCapacityLimited(c) },
			errType: "api_error", message: "No available OpenAI accounts", status: http.StatusServiceUnavailable,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainPlatform, category: service.OpsFailureCategoryRouting,
			reason: service.OpsFailureReasonNoAvailableAccounts, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name:    "enterprise member budget is enterprise owned and excluded",
			errType: "api_error", message: `error: code=429 reason="ENTERPRISE_MEMBER_BUDGET_EXCEEDED" message="enterprise member monthly budget is exhausted"`, status: http.StatusTooManyRequests,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainEnterprise, category: service.OpsFailureCategoryBudget,
			reason: service.OpsFailureReasonEnterpriseMemberBudgetExhausted, owner: service.OpsResolutionOwnerEnterpriseAdmin,
			customerSeen: true, slaImpact: false,
		},
		{
			name:    "customer balance is visible but excluded",
			errType: "billing_error", message: "Insufficient account balance", code: opsCodeInsufficientBalance, status: http.StatusForbidden,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainCustomer, category: service.OpsFailureCategoryBalance,
			reason: service.OpsFailureReasonUserBalanceExhausted, owner: service.OpsResolutionOwnerCustomer,
			customerSeen: true, slaImpact: false,
		},
		{
			name:    "final upstream failure retains upstream cause and counts against SLA",
			prepare: func(c *gin.Context) { service.SetOpsUpstreamError(c, http.StatusBadGateway, "provider failed", "") },
			errType: "upstream_error", message: "Upstream request failed", status: http.StatusBadGateway,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryInternal,
			reason: service.OpsFailureReasonProvider5xx, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name: "provider billing evidence wins over a generic 403 status",
			prepare: func(c *gin.Context) {
				service.SetOpsUpstreamError(c, http.StatusForbidden, "insufficient balance", "")
			},
			errType: "upstream_error", message: "provider rejected request: insufficient balance", status: http.StatusBadGateway,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryBalance,
			reason: service.OpsFailureReasonProviderBalanceExhausted, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name: "explicit account auth evidence wins over a billing-like message",
			prepare: func(c *gin.Context) {
				c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
					Stage: string(service.GatewayFailureStageAccountAuth), UpstreamStatusCode: http.StatusForbidden,
				}})
			},
			errType: "upstream_error", message: "insufficient balance", status: http.StatusBadGateway,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryCredential,
			reason: service.OpsFailureReasonProviderAuthFailed, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name: "provider 4xx is not mislabeled as provider 5xx",
			prepare: func(c *gin.Context) {
				service.SetOpsUpstreamError(c, http.StatusNotFound, "endpoint not found", "")
			},
			errType: "upstream_error", message: "upstream request failed", status: http.StatusBadGateway,
			eventScope: service.OpsEventScopeRequestTerminal,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryProtocol,
			reason: service.OpsFailureReasonProvider4xx, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name:       "provider failure without a usable status remains explicitly unknown",
			errType:    "upstream_error",
			message:    "provider returned an unclassified terminal error",
			status:     http.StatusOK,
			eventScope: service.OpsEventScopeStreamTerminal,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryUnknown,
			reason: service.OpsFailureReasonProviderErrorUnknown, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: true, slaImpact: true,
		},
		{
			name: "recovered upstream attempt is health evidence only",
			prepare: func(c *gin.Context) {
				service.SetOpsUpstreamError(c, http.StatusTooManyRequests, "provider rate limited", "")
			},
			errType: "upstream_error", message: "Recovered upstream error", status: http.StatusTooManyRequests,
			eventScope: service.OpsEventScopeUpstreamAttemptRecovered,
			domain:     service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryRateLimit,
			reason: service.OpsFailureReasonProviderRateLimited, owner: service.OpsResolutionOwnerPlatformOps,
			customerSeen: false, slaImpact: false,
		},
		{
			name:    "client cancellation remains diagnostic",
			errType: "api_error", message: "context canceled", status: 499,
			eventScope: service.OpsEventScopeStreamTerminal,
			domain:     service.OpsFailureDomainClient, category: service.OpsFailureCategoryCancellation,
			reason: service.OpsFailureReasonClientCancelled, owner: service.OpsResolutionOwnerClient,
			customerSeen: true, slaImpact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			if tt.prepare != nil {
				tt.prepare(c)
			}
			got := classifyOpsFailureV2(c, tt.errType, tt.message, tt.code, tt.status, tt.eventScope)
			require.Equal(t, tt.domain, got.FailureDomain)
			require.Equal(t, tt.category, got.FailureCategory)
			require.Equal(t, tt.reason, got.FailureReason)
			require.Equal(t, tt.owner, got.ResolutionOwner)
			require.Equal(t, tt.customerSeen, got.CustomerVisible)
			require.NotNil(t, got.SLAImpact)
			require.Equal(t, tt.slaImpact, *got.SLAImpact)
			require.Equal(t, service.OpsFailureClassificationVersion, got.ClassificationVersion)
		})
	}
}

func TestClassifyOpsFailureV2EnterpriseMemberLimitSignals(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		message  string
		code     string
		category string
		reason   string
	}{
		{
			name:     "shared budget message only",
			message:  service.ErrEnterpriseMemberBudgetExceeded.Message,
			category: service.OpsFailureCategoryBudget,
			reason:   service.OpsFailureReasonEnterpriseMemberBudgetExhausted,
		},
		{
			name:     "shared 5h rate message only",
			message:  service.ErrEnterpriseMemberRateLimit5hExceeded.Message,
			category: service.OpsFailureCategoryRateLimit,
			reason:   service.OpsFailureReasonEnterpriseMemberRateExceeded,
		},
		{
			name:     "shared daily rate message only",
			message:  service.ErrEnterpriseMemberRateLimit1dExceeded.Message,
			category: service.OpsFailureCategoryRateLimit,
			reason:   service.OpsFailureReasonEnterpriseMemberRateExceeded,
		},
		{
			name:     "shared 7 day rate message only",
			message:  service.ErrEnterpriseMemberRateLimit7dExceeded.Message,
			category: service.OpsFailureCategoryRateLimit,
			reason:   service.OpsFailureReasonEnterpriseMemberRateExceeded,
		},
		{
			name:     "legacy English budget message only",
			message:  "enterprise member monthly budget is exhausted",
			category: service.OpsFailureCategoryBudget,
			reason:   service.OpsFailureReasonEnterpriseMemberBudgetExhausted,
		},
		{
			name:     "legacy English short-window message only",
			message:  "enterprise member 5-hour spending limit is exhausted",
			category: service.OpsFailureCategoryRateLimit,
			reason:   service.OpsFailureReasonEnterpriseMemberRateExceeded,
		},
		{
			name:     "budget code only",
			code:     "ENTERPRISE_MEMBER_BUDGET_EXCEEDED",
			category: service.OpsFailureCategoryBudget,
			reason:   service.OpsFailureReasonEnterpriseMemberBudgetExhausted,
		},
		{
			name:     "rate code only",
			code:     "ENTERPRISE_MEMBER_RATE_1D_EXCEEDED",
			category: service.OpsFailureCategoryRateLimit,
			reason:   service.OpsFailureReasonEnterpriseMemberRateExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)

			got := classifyOpsFailureV2(
				c,
				"api_error",
				tt.message,
				tt.code,
				http.StatusTooManyRequests,
				service.OpsEventScopeRequestTerminal,
			)

			require.Equal(t, service.OpsFailureDomainEnterprise, got.FailureDomain)
			require.Equal(t, tt.category, got.FailureCategory)
			require.Equal(t, tt.reason, got.FailureReason)
			require.Equal(t, service.OpsResolutionOwnerEnterpriseAdmin, got.ResolutionOwner)
			require.True(t, got.CustomerVisible)
			require.NotNil(t, got.SLAImpact)
			require.False(t, *got.SLAImpact)
		})
	}
}

func TestClassifyOpsFailureV2AsyncEnterpriseMemberBudgetUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		message string
		code    string
	}{
		{"code only", "", "ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE"},
		{"normalized code", "", " enterprise_member_async_budget_unavailable "},
		{"Chinese message only", service.ErrEnterpriseMemberAsyncBudgetUnavailable.Message, ""},
		{"wrapped error", service.ErrEnterpriseMemberAsyncBudgetUnavailable.Error(), ""},
		{"legacy English message", "available enterprise member budget is insufficient for this asynchronous task after accounting for active task holds and this task's estimated cost", ""},
		{"legacy detailed message", "Asynchronous task budget is unavailable for the monthly limit: limit US$100.000000, settled usage US$80.000000, active task holds US$15.000000, requested task hold US$10.000000.", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			got := classifyOpsFailureV2(c, "api_error", tc.message, tc.code, http.StatusTooManyRequests, service.OpsEventScopeRequestTerminal)
			require.Equal(t, service.OpsFailureDomainEnterprise, got.FailureDomain)
			require.Equal(t, service.OpsFailureCategoryBudget, got.FailureCategory)
			require.Equal(t, "enterprise_member_async_budget_unavailable", got.FailureReason)
			require.Equal(t, service.OpsResolutionOwnerEnterpriseAdmin, got.ResolutionOwner)
			require.True(t, got.CustomerVisible)
			require.NotNil(t, got.SLAImpact)
			require.False(t, *got.SLAImpact)
		})
	}
}

func TestClassifyOpsFailureV2AsyncBudgetGoogleResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, window := range []string{"月度", "5 小时", "1 天", "7 天"} {
		t.Run(window, func(t *testing.T) {
			// Google responses carry a numeric error.code. The logger must also
			// recognize the detailed client message when no domain code is parsed.
			parsed := parseOpsErrorResponse([]byte(`{"error":{"code":429,"reason":"ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE","message":"您的` + window + `消费额度不足以提交本次异步任务：限额 US$100.000000，已消费 US$80.000000，进行中任务占用 US$15.000000，本次任务需预占 US$10.000000。"}}`))
			c, _ := gin.CreateTestContext(nil)
			got := classifyOpsFailureV2(c, parsed.ErrorType, parsed.Message, parsed.Code, http.StatusTooManyRequests, service.OpsEventScopeRequestTerminal)
			require.Equal(t, service.OpsFailureDomainEnterprise, got.FailureDomain)
			require.Equal(t, service.OpsFailureCategoryBudget, got.FailureCategory)
			require.Equal(t, "enterprise_member_async_budget_unavailable", got.FailureReason)
			require.Equal(t, service.OpsResolutionOwnerEnterpriseAdmin, got.ResolutionOwner)
			require.NotNil(t, got.SLAImpact)
			require.False(t, *got.SLAImpact)
		})
	}
}

func TestClassifyOpsFailureV2AsyncBudgetDoesNotHidePlatformFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name          string
		message       string
		code          string
		markAmbiguous bool
		wantReason    string
	}{
		{"internal failure", "Member budget authorization is temporarily unavailable", "", false, service.OpsFailureReasonInternalError},
		{"ambiguous task outcome", service.ErrEnterpriseMemberAsyncBudgetUnavailable.Message, "ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE", true, service.OpsFailureReasonBudgetOutcomeAmbiguous},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			if tc.markAmbiguous {
				service.MarkEnterpriseMemberBudgetOutcomeAmbiguous(c)
			}
			got := classifyOpsFailureV2(c, "api_error", tc.message, tc.code, http.StatusInternalServerError, service.OpsEventScopeRequestTerminal)
			require.Equal(t, service.OpsFailureDomainPlatform, got.FailureDomain)
			require.Equal(t, tc.wantReason, got.FailureReason)
			require.NotNil(t, got.SLAImpact)
			require.True(t, *got.SLAImpact)
		})
	}
}

func TestOpsFailureProductionFixtureIsConserved(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type bucket struct {
		name       string
		count      int64
		prepare    func(*gin.Context)
		errType    string
		message    string
		code       string
		status     int
		wantDomain string
		wantReason string
		wantSLA    bool
	}
	buckets := []bucket{
		{
			name: "main account balance", count: 4_707,
			errType: "billing_error", message: "Insufficient account balance", code: opsCodeInsufficientBalance, status: http.StatusForbidden,
			wantDomain: service.OpsFailureDomainCustomer, wantReason: service.OpsFailureReasonUserBalanceExhausted,
		},
		{
			name: "enterprise member budget", count: 72,
			errType: "api_error", message: `reason="ENTERPRISE_MEMBER_BUDGET_EXCEEDED" message="enterprise member monthly budget is exhausted"`, status: http.StatusTooManyRequests,
			wantDomain: service.OpsFailureDomainEnterprise, wantReason: service.OpsFailureReasonEnterpriseMemberBudgetExhausted,
		},
		{
			name: "other account permission and request restrictions", count: 211,
			errType: "invalid_request_error", message: "invalid request", status: http.StatusBadRequest,
			wantDomain: service.OpsFailureDomainClient, wantReason: service.OpsFailureReasonInvalidRequest,
		},
		{
			name: "platform routing capacity", count: 4_813,
			prepare: func(c *gin.Context) { markOpsRoutingCapacityLimited(c) },
			errType: "api_error", message: "No available OpenAI accounts", status: http.StatusServiceUnavailable,
			wantDomain: service.OpsFailureDomainPlatform, wantReason: service.OpsFailureReasonNoAvailableAccounts, wantSLA: true,
		},
		{
			name: "terminal upstream failure", count: 65,
			prepare: func(c *gin.Context) { service.SetOpsUpstreamError(c, http.StatusBadGateway, "provider failed", "") },
			errType: "upstream_error", message: "Upstream request failed", status: http.StatusBadGateway,
			wantDomain: service.OpsFailureDomainUpstream, wantReason: service.OpsFailureReasonProvider5xx, wantSLA: true,
		},
		{
			name: "client interruption", count: 39,
			errType: "api_error", message: "context canceled", status: 499,
			wantDomain: service.OpsFailureDomainClient, wantReason: service.OpsFailureReasonClientCancelled,
		},
	}

	var total, platformSLA, excluded int64
	for _, item := range buckets {
		c, _ := gin.CreateTestContext(nil)
		if item.prepare != nil {
			item.prepare(c)
		}
		classification := classifyOpsFailureV2(
			c,
			item.errType,
			item.message,
			item.code,
			item.status,
			service.OpsEventScopeRequestTerminal,
		)
		require.True(t, classification.CustomerVisible, item.name)
		require.Equal(t, item.wantDomain, classification.FailureDomain, item.name)
		require.Equal(t, item.wantReason, classification.FailureReason, item.name)
		require.NotNil(t, classification.SLAImpact, item.name)
		require.Equal(t, item.wantSLA, *classification.SLAImpact, item.name)

		total += item.count
		if item.wantSLA {
			platformSLA += item.count
		} else {
			excluded += item.count
		}
	}
	require.Equal(t, int64(9_907), total)
	require.Equal(t, int64(4_878), platformSLA)
	require.Equal(t, int64(5_029), excluded)
	require.Equal(t, total, platformSLA+excluded)
}
