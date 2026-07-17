package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// classifyOpsFailureV2 is the only writer-side classifier for the structured
// v2 fields. The legacy classifier remains in place solely to populate
// is_business_limited during the compatibility window.
func classifyOpsFailureV2(
	c *gin.Context,
	errType string,
	message string,
	code string,
	status int,
	eventScope string,
) service.OpsFailureClassification {
	classification := service.OpsFailureClassification{
		EventScope:            eventScope,
		CustomerVisible:       eventScope != service.OpsEventScopeUpstreamAttemptRecovered,
		FailureDomain:         service.OpsFailureDomainUnknown,
		FailureCategory:       service.OpsFailureCategoryUnknown,
		FailureReason:         service.OpsFailureReasonLegacyUnknown,
		ResolutionOwner:       service.OpsResolutionOwnerUnknown,
		PoolOwnership:         service.OpsPoolOwnershipUnknown,
		SLAImpact:             nil,
		ClassificationVersion: service.OpsFailureClassificationVersion,
	}

	msg := strings.ToLower(strings.TrimSpace(message))
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))

	if eventScope == service.OpsEventScopeUpstreamAttemptRecovered {
		classification.CustomerVisible = false
		classification.SLAImpact = service.OpsBool(false)
		classifyRecoveredUpstreamFailure(&classification, c, msg, status)
		return classification
	}

	if service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c) {
		return opsPlatformFailure(classification, service.OpsFailureCategoryInternal, service.OpsFailureReasonBudgetOutcomeAmbiguous)
	}

	if isOpsRoutingCapacityLimited(c) {
		classification.PoolOwnership = service.OpsPoolOwnershipPlatform
		return opsPlatformFailure(classification, service.OpsFailureCategoryRouting, service.OpsFailureReasonNoAvailableAccounts)
	}

	if reason, ok := classifyMarkedClientPolicyFailure(c); ok {
		classification.FailureCategory = service.OpsFailureCategoryPermission
		classification.FailureReason = reason
		classification.SLAImpact = service.OpsBool(false)
		if apiKey := getOpsAPIKey(c); apiKey != nil && apiKey.MemberID != nil {
			classification.FailureDomain = service.OpsFailureDomainEnterprise
			classification.ResolutionOwner = service.OpsResolutionOwnerEnterpriseAdmin
		} else {
			classification.FailureDomain = service.OpsFailureDomainCustomer
			classification.ResolutionOwner = service.OpsResolutionOwnerCustomer
		}
		return classification
	}

	if isEnterpriseMemberLimit(normalizedCode, msg) {
		classification.FailureDomain = service.OpsFailureDomainEnterprise
		classification.ResolutionOwner = service.OpsResolutionOwnerEnterpriseAdmin
		classification.SLAImpact = service.OpsBool(false)
		if strings.Contains(normalizedCode, "BUDGET") || strings.Contains(msg, "monthly budget") {
			classification.FailureCategory = service.OpsFailureCategoryBudget
			classification.FailureReason = service.OpsFailureReasonEnterpriseMemberBudgetExhausted
		} else {
			classification.FailureCategory = service.OpsFailureCategoryRateLimit
			classification.FailureReason = service.OpsFailureReasonEnterpriseMemberRateExceeded
		}
		return classification
	}

	if !hasOpsUpstreamErrorContext(c) && errType != "upstream_error" && errType != "overloaded_error" && classificationForCustomerCode(&classification, normalizedCode, msg) {
		return classification
	}

	if status == 499 || strings.Contains(msg, "context canceled") || strings.Contains(msg, "client cancelled") {
		classification.FailureDomain = service.OpsFailureDomainClient
		classification.FailureCategory = service.OpsFailureCategoryCancellation
		classification.FailureReason = service.OpsFailureReasonClientCancelled
		classification.ResolutionOwner = service.OpsResolutionOwnerClient
		classification.SLAImpact = service.OpsBool(false)
		return classification
	}
	if strings.Contains(msg, "client disconnected") || strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer") {
		classification.FailureDomain = service.OpsFailureDomainClient
		classification.FailureCategory = service.OpsFailureCategoryNetwork
		classification.FailureReason = service.OpsFailureReasonClientDisconnected
		classification.ResolutionOwner = service.OpsResolutionOwnerClient
		classification.SLAImpact = service.OpsBool(false)
		return classification
	}

	if hasOpsUpstreamErrorContext(c) || errType == "upstream_error" || errType == "overloaded_error" {
		return classifyTerminalUpstreamFailure(classification, c, msg, status)
	}

	if errType == "invalid_request_error" || strings.Contains(msg, "invalid request") || strings.Contains(msg, "unable to parse request") {
		return opsClientRequestFailure(classification, service.OpsFailureCategoryProtocol, service.OpsFailureReasonInvalidRequest)
	}
	if status == http.StatusNotFound || strings.Contains(msg, "model") && strings.Contains(msg, "not found") {
		return opsClientRequestFailure(classification, service.OpsFailureCategoryCapability, service.OpsFailureReasonModelNotFound)
	}
	if strings.Contains(msg, "not supported") || strings.Contains(msg, "unsupported") {
		return opsClientRequestFailure(classification, service.OpsFailureCategoryProtocol, service.OpsFailureReasonUnsupportedProtocol)
	}

	phase := classifyOpsPhase(errType, message, code)
	if phase == "routing" || isOpsNoAvailableAccountMessage(msg) {
		classification.PoolOwnership = service.OpsPoolOwnershipPlatform
		return opsPlatformFailure(classification, service.OpsFailureCategoryRouting, service.OpsFailureReasonNoAvailableAccounts)
	}
	if strings.Contains(msg, "database") || strings.Contains(msg, "postgres") {
		return opsPlatformFailure(classification, service.OpsFailureCategoryDependency, service.OpsFailureReasonDatabaseUnavailable)
	}
	if strings.Contains(msg, "redis") {
		return opsPlatformFailure(classification, service.OpsFailureCategoryDependency, service.OpsFailureReasonRedisUnavailable)
	}
	if status >= http.StatusInternalServerError || phase == "internal" {
		return opsPlatformFailure(classification, service.OpsFailureCategoryInternal, service.OpsFailureReasonInternalError)
	}
	if status >= http.StatusBadRequest {
		return opsClientRequestFailure(classification, service.OpsFailureCategoryProtocol, service.OpsFailureReasonInvalidRequest)
	}

	return classification
}

func classifyMarkedClientPolicyFailure(c *gin.Context) (string, bool) {
	if c == nil || !service.HasOpsClientBusinessLimited(c) {
		return "", false
	}
	raw, _ := c.Get(service.OpsClientBusinessLimitedReasonKey)
	reason, _ := raw.(string)
	switch strings.TrimSpace(reason) {
	case service.OpsClientBusinessLimitedReasonIPRestriction:
		return service.OpsFailureReasonIPRestricted, true
	case service.OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned:
		return service.OpsFailureReasonGroupUnassigned, true
	case service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable:
		return service.OpsFailureReasonGroupUnavailable, true
	case service.OpsClientBusinessLimitedReasonLocalFeatureGate,
		service.OpsClientBusinessLimitedReasonLocalPolicyDenied:
		return service.OpsFailureReasonEndpointNotAllowed, true
	default:
		return service.OpsFailureReasonEndpointNotAllowed, true
	}
}

func isEnterpriseMemberLimit(code, msg string) bool {
	return strings.Contains(code, "ENTERPRISE_MEMBER_BUDGET_EXCEEDED") ||
		strings.Contains(code, "ENTERPRISE_MEMBER_RATE_") ||
		strings.Contains(msg, "enterprise_member_budget_exceeded") ||
		strings.Contains(msg, "enterprise_member_rate_") ||
		strings.Contains(msg, "enterprise member monthly budget is exhausted") ||
		strings.Contains(msg, "enterprise member 5-hour spending limit is exhausted") ||
		strings.Contains(msg, "enterprise member daily spending limit is exhausted") ||
		strings.Contains(msg, "enterprise member 7-day spending limit is exhausted")
}

func classificationForCustomerCode(classification *service.OpsFailureClassification, code, msg string) bool {
	if classification == nil {
		return false
	}
	classification.FailureDomain = service.OpsFailureDomainCustomer
	classification.ResolutionOwner = service.OpsResolutionOwnerCustomer
	classification.SLAImpact = service.OpsBool(false)

	switch code {
	case opsCodeInsufficientBalance:
		classification.FailureCategory = service.OpsFailureCategoryBalance
		classification.FailureReason = service.OpsFailureReasonUserBalanceExhausted
		return true
	case opsCodeInvalidAPIKey, opsCodeAPIKeyRequired, opsCodeUserNotFound, opsCodeUserInactive:
		classification.FailureCategory = service.OpsFailureCategoryAuthentication
		classification.FailureReason = service.OpsFailureReasonAPIKeyInvalid
		return true
	case opsCodeAPIKeyDisabled:
		classification.FailureCategory = service.OpsFailureCategoryAuthentication
		classification.FailureReason = service.OpsFailureReasonAPIKeyDisabled
		return true
	case opsCodeAPIKeyExpired:
		classification.FailureCategory = service.OpsFailureCategoryAuthentication
		classification.FailureReason = service.OpsFailureReasonAPIKeyExpired
		return true
	case opsCodeAPIKeyQuotaExhausted:
		classification.FailureCategory = service.OpsFailureCategoryQuota
		classification.FailureReason = service.OpsFailureReasonAPIKeyQuotaExhausted
		return true
	case opsCodeGroupDeleted, opsCodeGroupDisabled:
		classification.FailureCategory = service.OpsFailureCategoryPermission
		classification.FailureReason = service.OpsFailureReasonGroupUnavailable
		return true
	case opsCodeUsageLimitExceeded:
		classification.FailureCategory = service.OpsFailureCategoryQuota
		classification.FailureReason = service.OpsFailureReasonUserQuotaExhausted
		return true
	case opsCodeSubscriptionNotFound, opsCodeSubscriptionInvalid:
		classification.FailureCategory = service.OpsFailureCategoryPermission
		classification.FailureReason = service.OpsFailureReasonGroupUnavailable
		return true
	}

	switch {
	case strings.Contains(msg, "insufficient balance"), strings.Contains(msg, "insufficient account balance"):
		classification.FailureCategory = service.OpsFailureCategoryBalance
		classification.FailureReason = service.OpsFailureReasonUserBalanceExhausted
		return true
	case strings.Contains(msg, "api key") && strings.Contains(msg, "额度已用完"):
		classification.FailureCategory = service.OpsFailureCategoryQuota
		classification.FailureReason = service.OpsFailureReasonAPIKeyQuotaExhausted
		return true
	case strings.Contains(msg, "usage quota exhausted"), strings.Contains(msg, "daily usage limit"), strings.Contains(msg, "weekly usage limit"), strings.Contains(msg, "monthly usage limit"):
		classification.FailureCategory = service.OpsFailureCategoryQuota
		classification.FailureReason = service.OpsFailureReasonUserQuotaExhausted
		return true
	case strings.Contains(msg, "requests-per-minute"), strings.Contains(msg, "rpm limit"), strings.Contains(msg, "rate limit exceeded"):
		classification.FailureCategory = service.OpsFailureCategoryRateLimit
		classification.FailureReason = service.OpsFailureReasonRateLimitExceeded
		return true
	case strings.Contains(msg, "concurrency limit"), strings.Contains(msg, "too many pending requests"):
		classification.FailureCategory = service.OpsFailureCategoryConcurrency
		classification.FailureReason = service.OpsFailureReasonConcurrencyExceeded
		return true
	case strings.Contains(msg, "not in whitelist"):
		classification.FailureCategory = service.OpsFailureCategoryPermission
		classification.FailureReason = service.OpsFailureReasonModelNotAuthorized
		return true
	case strings.Contains(msg, "group") && (strings.Contains(msg, "disabled") || strings.Contains(msg, "deleted") || strings.Contains(msg, "not assigned")):
		classification.FailureCategory = service.OpsFailureCategoryPermission
		classification.FailureReason = service.OpsFailureReasonGroupUnavailable
		return true
	}

	classification.FailureDomain = service.OpsFailureDomainUnknown
	classification.ResolutionOwner = service.OpsResolutionOwnerUnknown
	classification.SLAImpact = nil
	return false
}

func classifyRecoveredUpstreamFailure(classification *service.OpsFailureClassification, c *gin.Context, msg string, status int) {
	if classification == nil {
		return
	}
	result := classifyTerminalUpstreamFailure(*classification, c, msg, status)
	result.CustomerVisible = false
	result.SLAImpact = service.OpsBool(false)
	*classification = result
}

func classifyTerminalUpstreamFailure(classification service.OpsFailureClassification, c *gin.Context, msg string, status int) service.OpsFailureClassification {
	classification.FailureDomain = service.OpsFailureDomainUpstream
	classification.ResolutionOwner = service.OpsResolutionOwnerPlatformOps
	classification.PoolOwnership = service.OpsPoolOwnershipPlatform
	classification.SLAImpact = service.OpsBool(true)

	upstreamStatus := status
	if c != nil {
		if value, ok := c.Get(service.OpsUpstreamStatusCodeKey); ok {
			switch typed := value.(type) {
			case int:
				if typed > 0 {
					upstreamStatus = typed
				}
			case int64:
				if typed > 0 {
					upstreamStatus = int(typed)
				}
			}
		}
	}

	if hasOpsAccountAuthFailure(c) {
		classification.FailureCategory = service.OpsFailureCategoryCredential
		classification.FailureReason = service.OpsFailureReasonProviderAuthFailed
		return classification
	}
	// Providers sometimes report billing/quota exhaustion with 401/403. Stable
	// response evidence is more specific than the status code, while an explicit
	// account-auth stage above remains authoritative.
	if strings.Contains(msg, "balance") || strings.Contains(msg, "quota") {
		classification.FailureCategory = service.OpsFailureCategoryBalance
		classification.FailureReason = service.OpsFailureReasonProviderBalanceExhausted
		return classification
	}
	if upstreamStatus == http.StatusUnauthorized || upstreamStatus == http.StatusForbidden {
		classification.FailureCategory = service.OpsFailureCategoryCredential
		classification.FailureReason = service.OpsFailureReasonProviderAuthFailed
		return classification
	}
	switch upstreamStatus {
	case http.StatusTooManyRequests:
		classification.FailureCategory = service.OpsFailureCategoryRateLimit
		classification.FailureReason = service.OpsFailureReasonProviderRateLimited
		return classification
	case 529:
		classification.FailureCategory = service.OpsFailureCategoryOverload
		classification.FailureReason = service.OpsFailureReasonProviderOverloaded
		return classification
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		classification.FailureCategory = service.OpsFailureCategoryTimeout
		classification.FailureReason = service.OpsFailureReasonProviderTimeout
		return classification
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		classification.FailureCategory = service.OpsFailureCategoryTimeout
		classification.FailureReason = service.OpsFailureReasonProviderTimeout
		return classification
	}
	if strings.Contains(msg, "network") || strings.Contains(msg, "connection reset") || strings.Contains(msg, "connection refused") {
		classification.FailureCategory = service.OpsFailureCategoryNetwork
		classification.FailureReason = service.OpsFailureReasonProviderNetworkError
		return classification
	}
	if upstreamStatus >= http.StatusInternalServerError {
		classification.FailureCategory = service.OpsFailureCategoryInternal
		classification.FailureReason = service.OpsFailureReasonProvider5xx
		return classification
	}
	if upstreamStatus >= http.StatusBadRequest {
		classification.FailureCategory = service.OpsFailureCategoryProtocol
		classification.FailureReason = service.OpsFailureReasonProvider4xx
		return classification
	}
	classification.FailureCategory = service.OpsFailureCategoryUnknown
	classification.FailureReason = service.OpsFailureReasonProviderErrorUnknown
	return classification
}

func opsPlatformFailure(classification service.OpsFailureClassification, category, reason string) service.OpsFailureClassification {
	classification.FailureDomain = service.OpsFailureDomainPlatform
	classification.FailureCategory = category
	classification.FailureReason = reason
	classification.ResolutionOwner = service.OpsResolutionOwnerPlatformOps
	classification.SLAImpact = service.OpsBool(true)
	return classification
}

func opsClientRequestFailure(classification service.OpsFailureClassification, category, reason string) service.OpsFailureClassification {
	classification.FailureDomain = service.OpsFailureDomainClient
	classification.FailureCategory = category
	classification.FailureReason = reason
	classification.ResolutionOwner = service.OpsResolutionOwnerClient
	classification.SLAImpact = service.OpsBool(false)
	return classification
}

func applyOpsFailureClassification(entry *service.OpsInsertErrorLogInput, classification service.OpsFailureClassification) {
	if entry == nil {
		return
	}
	entry.EventScope = classification.EventScope
	entry.CustomerVisible = classification.CustomerVisible
	entry.FailureDomain = classification.FailureDomain
	entry.FailureCategory = classification.FailureCategory
	entry.FailureReason = classification.FailureReason
	entry.ResolutionOwner = classification.ResolutionOwner
	entry.PoolOwnership = classification.PoolOwnership
	entry.SLAImpact = classification.SLAImpact
	entry.ClassificationVersion = classification.ClassificationVersion
}
