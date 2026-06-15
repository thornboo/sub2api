package admin

import "strings"

func normalizeDashboardTrendGranularity(raw string) string {
	granularity := strings.TrimSpace(raw)
	switch granularity {
	case "hour", "day", "month":
		return granularity
	default:
		return "day"
	}
}
