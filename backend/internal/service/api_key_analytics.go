package service

import "time"

const (
	DefaultOwnerAPIKeyAnalyticsLimit = 20
	MaxOwnerAPIKeyAnalyticsLimit     = 100
)

type OwnerAPIKeyAnalyticsFilters struct {
	UserID          int64
	APIKeyID        *int64
	MemberID        *int64
	MemberScope     string
	MemberFilterSet bool
	StartTime       time.Time
	EndTime         time.Time
	TimezoneName    string
	Granularity     string
	GroupID         *int64
	Tags            []string
	Status          string
	Search          string
	Limit           int
}

type OwnerUsageMember struct {
	ID              int64      `json:"id"`
	MemberCode      string     `json:"member_code"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	Archived        bool       `json:"archived"`
	KeyCount        int64      `json:"key_count"`
	MonthlyLimitUSD float64    `json:"monthly_limit_usd"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type OwnerAPIKeyUsageTotals struct {
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
}

type OwnerAPIKeyAnalyticsSnapshot struct {
	ActiveKeyCount        int64     `json:"active_key_count"`
	NearQuotaKeyCount     int64     `json:"near_quota_key_count"`
	NearRateLimitKeyCount int64     `json:"near_rate_limit_key_count"`
	SnapshotAt            time.Time `json:"snapshot_at"`
}

type OwnerAPIKeyAnalyticsSummary struct {
	OwnerAPIKeyUsageTotals
	UsedKeyCount       int64                        `json:"used_key_count"`
	CurrentKeySnapshot OwnerAPIKeyAnalyticsSnapshot `json:"current_key_snapshot"`
}

type OwnerAPIKeyLeaderboardItem struct {
	APIKeyID  int64    `json:"api_key_id"`
	KeyName   string   `json:"key_name"`
	Tags      []string `json:"tags"`
	GroupID   *int64   `json:"group_id,omitempty"`
	GroupName string   `json:"group_name"`
	Status    string   `json:"status"`
	OwnerAPIKeyUsageTotals
	SharePercent       float64    `json:"share_percent"`
	PreviousActualCost float64    `json:"previous_actual_cost"`
	ChangePercent      float64    `json:"change_percent"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
}

type OwnerAPIKeyLeaderboardResponse struct {
	Items               []OwnerAPIKeyLeaderboardItem `json:"items"`
	Total               int64                        `json:"total"`
	TotalActualCost     float64                      `json:"total_actual_cost"`
	DisplayedActualCost float64                      `json:"displayed_actual_cost"`
}

type OwnerMemberLeaderboardItem struct {
	MemberID           *int64  `json:"member_id"`
	MemberCode         string  `json:"member_code"`
	MemberName         string  `json:"member_name"`
	Status             string  `json:"status"`
	Archived           bool    `json:"archived"`
	KeyCount           int64   `json:"key_count"`
	MonthlyLimitUSD    float64 `json:"monthly_limit_usd"`
	CurrentUsedUSD     float64 `json:"current_used_usd"`
	CurrentReservedUSD float64 `json:"current_reserved_usd"`
	OwnerAPIKeyUsageTotals
	SharePercent       float64    `json:"share_percent"`
	PreviousActualCost float64    `json:"previous_actual_cost"`
	ChangePercent      float64    `json:"change_percent"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
}

type OwnerMemberLeaderboardResponse struct {
	Items                 []OwnerMemberLeaderboardItem `json:"items"`
	Total                 int64                        `json:"total"`
	MemberCount           int64                        `json:"member_count"`
	BudgetRiskMemberCount int64                        `json:"budget_risk_member_count"`
	TotalReservedUSD      float64                      `json:"total_reserved_usd"`
	TotalActualCost       float64                      `json:"total_actual_cost"`
	DisplayedActualCost   float64                      `json:"displayed_actual_cost"`
}

type OwnerModelAnalyticsItem struct {
	Model string `json:"model"`
	OwnerAPIKeyUsageTotals
}

type OwnerGroupAnalyticsItem struct {
	GroupID      *int64  `json:"group_id,omitempty"`
	GroupName    string  `json:"group_name"`
	KeyCount     int64   `json:"key_count"`
	SharePercent float64 `json:"share_percent"`
	OwnerAPIKeyUsageTotals
}

type OwnerTagAnalyticsItem struct {
	Tag      string `json:"tag"`
	KeyCount int64  `json:"key_count"`
	OwnerAPIKeyUsageTotals
}

type OwnerTrendAnalyticsPoint struct {
	Date string `json:"date"`
	OwnerAPIKeyUsageTotals
}
