package service

// Ops failure classification v2 separates customer visibility, attribution,
// remediation ownership, and platform SLA impact. These values are persisted
// on ops_error_logs and are the source of truth for all v2 aggregates.
const OpsFailureClassificationVersion int16 = 2

const (
	OpsEventScopeRequestTerminal          = "request_terminal"
	OpsEventScopeStreamTerminal           = "stream_terminal"
	OpsEventScopeUpstreamAttemptRecovered = "upstream_attempt_recovered"
)

const (
	OpsFailureDomainCustomer   = "customer"
	OpsFailureDomainEnterprise = "enterprise"
	OpsFailureDomainClient     = "client"
	OpsFailureDomainPlatform   = "platform"
	OpsFailureDomainUpstream   = "upstream"
	OpsFailureDomainUnknown    = "unknown"
)

const (
	OpsFailureCategoryAuthentication = "authentication"
	OpsFailureCategoryBalance        = "balance"
	OpsFailureCategoryBudget         = "budget"
	OpsFailureCategoryQuota          = "quota"
	OpsFailureCategoryRateLimit      = "rate_limit"
	OpsFailureCategoryConcurrency    = "concurrency"
	OpsFailureCategoryPermission     = "permission"
	OpsFailureCategoryCapability     = "capability"
	OpsFailureCategoryProtocol       = "protocol"
	OpsFailureCategoryRouting        = "routing_capacity"
	OpsFailureCategoryCredential     = "credential"
	OpsFailureCategoryOverload       = "overload"
	OpsFailureCategoryTimeout        = "timeout"
	OpsFailureCategoryNetwork        = "network"
	OpsFailureCategoryDependency     = "dependency"
	OpsFailureCategoryInternal       = "internal"
	OpsFailureCategoryCancellation   = "cancellation"
	OpsFailureCategoryUnknown        = "unknown"
)

// OpsFailureBreakdownCategoryNonRouting is a read-side aggregate/filter key.
// It is never persisted as a failure_category; it expands to every platform
// category except routing_capacity so the summary and drill-down conserve rows.
const OpsFailureBreakdownCategoryNonRouting = "non_routing"

const (
	OpsResolutionOwnerCustomer        = "customer"
	OpsResolutionOwnerEnterpriseAdmin = "enterprise_admin"
	OpsResolutionOwnerPlatformOps     = "platform_ops"
	OpsResolutionOwnerClient          = "client"
	OpsResolutionOwnerUnknown         = "unknown"
)

const (
	OpsPoolOwnershipPlatform   = "platform"
	OpsPoolOwnershipEnterprise = "enterprise"
	OpsPoolOwnershipUnknown    = "unknown"
)

// Stable reason codes used by the initial v2 classifier. New values must be
// added deliberately because API filters and dashboard labels consume them.
const (
	OpsFailureReasonUserBalanceExhausted            = "user_balance_exhausted"
	OpsFailureReasonEnterpriseMemberBudgetExhausted = "enterprise_member_budget_exhausted"
	OpsFailureReasonEnterpriseMemberRateExceeded    = "enterprise_member_rate_limit_exceeded"
	OpsFailureReasonAPIKeyInvalid                   = "api_key_invalid"
	OpsFailureReasonAPIKeyDisabled                  = "api_key_disabled"
	OpsFailureReasonAPIKeyExpired                   = "api_key_expired"
	OpsFailureReasonAPIKeyQuotaExhausted            = "api_key_quota_exhausted"
	OpsFailureReasonUserQuotaExhausted              = "user_quota_exhausted"
	OpsFailureReasonRateLimitExceeded               = "rate_limit_exceeded"
	OpsFailureReasonConcurrencyExceeded             = "concurrency_exceeded"
	OpsFailureReasonGroupUnavailable                = "group_unavailable"
	OpsFailureReasonGroupUnassigned                 = "group_unassigned"
	OpsFailureReasonModelNotAuthorized              = "model_not_authorized"
	OpsFailureReasonEndpointNotAllowed              = "endpoint_not_allowed"
	OpsFailureReasonIPRestricted                    = "api_key_ip_restriction"
	OpsFailureReasonInvalidRequest                  = "invalid_request"
	OpsFailureReasonUnsupportedProtocol             = "unsupported_protocol"
	OpsFailureReasonModelNotFound                   = "model_not_found"
	OpsFailureReasonNoAvailableAccounts             = "no_available_accounts"
	OpsFailureReasonProviderAuthFailed              = "provider_auth_failed"
	OpsFailureReasonProviderBalanceExhausted        = "provider_balance_exhausted"
	OpsFailureReasonProviderRateLimited             = "provider_rate_limited"
	OpsFailureReasonProviderOverloaded              = "provider_overloaded"
	OpsFailureReasonProviderTimeout                 = "provider_timeout"
	OpsFailureReasonProviderNetworkError            = "provider_network_error"
	OpsFailureReasonProvider4xx                     = "provider_4xx"
	OpsFailureReasonProvider5xx                     = "provider_5xx"
	OpsFailureReasonProviderErrorUnknown            = "provider_error_unknown"
	OpsFailureReasonClientCancelled                 = "client_cancelled"
	OpsFailureReasonClientDisconnected              = "client_disconnected"
	OpsFailureReasonDatabaseUnavailable             = "database_unavailable"
	OpsFailureReasonRedisUnavailable                = "redis_unavailable"
	OpsFailureReasonBudgetOutcomeAmbiguous          = "budget_outcome_ambiguous"
	OpsFailureReasonInternalError                   = "internal_error"
	OpsFailureReasonLegacyUnknown                   = "legacy_unknown"
)

type OpsFailureClassification struct {
	EventScope            string
	CustomerVisible       bool
	FailureDomain         string
	FailureCategory       string
	FailureReason         string
	ResolutionOwner       string
	PoolOwnership         string
	SLAImpact             *bool
	ClassificationVersion int16
}

func OpsBool(value bool) *bool {
	return &value
}
