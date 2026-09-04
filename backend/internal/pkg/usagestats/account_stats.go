package usagestats

// AccountStats 账号使用统计
//
// cost: 账号口径费用（使用 total_cost * account_rate_multiplier）
// standard_cost: 标准费用（使用 total_cost，不含倍率）
// user_cost: 用户/API Key 口径费用（使用 actual_cost，受分组倍率影响）
type AccountStats struct {
	Requests                  int64    `json:"requests"`
	Tokens                    int64    `json:"tokens"`
	Cost                      *float64 `json:"cost"`
	UpstreamExpectedCostCount int64    `json:"upstream_expected_cost_count"`
	MissingUpstreamCostCount  int64    `json:"missing_upstream_cost_count"`
	StandardCost              float64  `json:"standard_cost"`
	UserCost                  float64  `json:"user_cost"`
}
