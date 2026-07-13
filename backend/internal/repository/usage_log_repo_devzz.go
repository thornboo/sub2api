package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const ownerAnalyticsNearLimitThreshold = 0.8

func strictDateFormat(granularity string) (string, bool) {
	f, ok := dateFormatWhitelist[granularity]
	return f, ok
}

func (r *usageLogRepository) GetAPIKeyUsageTrendForUser(ctx context.Context, userID, apiKeyID int64, startTime, endTime time.Time, granularity, timezoneName string) (results []TrendDataPoint, err error) {
	dateFormat, ok := strictDateFormat(granularity)
	if !ok {
		return nil, fmt.Errorf("invalid granularity: %s", granularity)
	}
	if timezoneName == "" {
		timezoneName = timezone.Name()
		if timezoneName == "Local" {
			timezoneName = "UTC"
		}
	}
	if _, err := time.LoadLocation(timezoneName); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			TO_CHAR(created_at AT TIME ZONE $3, '%s') as date,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(cache_creation_tokens), 0) as cache_creation_tokens,
			COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as total_tokens,
			COALESCE(SUM(total_cost), 0) as cost,
			COALESCE(SUM(actual_cost), 0) as actual_cost
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
			AND user_id = $4
			AND api_key_id = $5
		GROUP BY date
		ORDER BY date ASC
	`, dateFormat)

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, timezoneName, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results, err = scanTrendRows(rows)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ownerAnalyticsLimit(limit int) int {
	if limit <= 0 {
		return service.DefaultOwnerAPIKeyAnalyticsLimit
	}
	if limit > service.MaxOwnerAPIKeyAnalyticsLimit {
		return service.MaxOwnerAPIKeyAnalyticsLimit
	}
	return limit
}

func ownerAnalyticsAPIKeyFilters(conditions []string, args []any, filters service.OwnerAPIKeyAnalyticsFilters, alias string) ([]string, []any, error) {
	if filters.UserID <= 0 {
		return nil, nil, fmt.Errorf("owner analytics requires user id")
	}
	conditions = append(conditions, fmt.Sprintf("%s.user_id = $%d", alias, len(args)+1))
	args = append(args, filters.UserID)
	conditions = append(conditions, fmt.Sprintf("%s.deleted_at IS NULL", alias))

	if filters.APIKeyID != nil {
		conditions = append(conditions, fmt.Sprintf("%s.id = $%d", alias, len(args)+1))
		args = append(args, *filters.APIKeyID)
	}
	if filters.MemberID != nil {
		conditions = append(conditions, fmt.Sprintf("%s.member_id = $%d", alias, len(args)+1))
		args = append(args, *filters.MemberID)
	} else {
		switch strings.TrimSpace(filters.MemberScope) {
		case usagestats.MemberScopeAssigned:
			conditions = append(conditions, fmt.Sprintf("%s.member_id IS NOT NULL", alias))
		case usagestats.MemberScopeUnassigned:
			conditions = append(conditions, fmt.Sprintf("%s.member_id IS NULL", alias))
		}
	}
	if filters.GroupID != nil {
		if *filters.GroupID == 0 {
			conditions = append(conditions, fmt.Sprintf("%s.group_id IS NULL", alias))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s.group_id = $%d", alias, len(args)+1))
			args = append(args, *filters.GroupID)
		}
	}
	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("%s.status = $%d", alias, len(args)+1))
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(%s.name ILIKE $%d OR %s.key ILIKE $%d)", alias, len(args)+1, alias, len(args)+1))
		args = append(args, "%"+filters.Search+"%")
	}
	if len(filters.Tags) > 0 {
		tagsJSON, err := json.Marshal(filters.Tags)
		if err != nil {
			return nil, nil, err
		}
		conditions = append(conditions, fmt.Sprintf("%s.tags @> $%d::jsonb", alias, len(args)+1))
		args = append(args, string(tagsJSON))
	}
	return conditions, args, nil
}

func ownerAnalyticsMemberFilterActive(filters service.OwnerAPIKeyAnalyticsFilters) bool {
	return filters.MemberFilterSet || filters.MemberID != nil || strings.TrimSpace(filters.MemberScope) != ""
}

func ownerAnalyticsUsageConditions(filters service.OwnerAPIKeyAnalyticsFilters, includeTime bool) ([]string, []any, error) {
	conditions := make([]string, 0, 8)
	args := make([]any, 0, 8)
	if filters.UserID <= 0 {
		return nil, nil, fmt.Errorf("owner analytics requires user id")
	}
	conditions = append(conditions, fmt.Sprintf("ul.user_id = $%d", len(args)+1))
	args = append(args, filters.UserID)
	if includeTime {
		conditions = append(conditions, fmt.Sprintf("ul.created_at >= $%d", len(args)+1))
		args = append(args, filters.StartTime)
		conditions = append(conditions, fmt.Sprintf("ul.created_at < $%d", len(args)+1))
		args = append(args, filters.EndTime)
	}
	if ownerAnalyticsMemberFilterActive(filters) {
		if filters.MemberID != nil {
			conditions = append(conditions, fmt.Sprintf("ul.member_id = $%d", len(args)+1))
			args = append(args, *filters.MemberID)
		} else {
			switch strings.TrimSpace(filters.MemberScope) {
			case usagestats.MemberScopeAssigned:
				conditions = append(conditions, "ul.member_id IS NOT NULL")
			case usagestats.MemberScopeUnassigned:
				conditions = append(conditions, "ul.member_id IS NULL")
			}
		}
		if filters.APIKeyID != nil {
			conditions = append(conditions, fmt.Sprintf("ul.api_key_id = $%d", len(args)+1))
			args = append(args, *filters.APIKeyID)
		}
		if filters.GroupID != nil {
			if *filters.GroupID == 0 {
				conditions = append(conditions, "ul.group_id IS NULL")
			} else {
				conditions = append(conditions, fmt.Sprintf("ul.group_id = $%d", len(args)+1))
				args = append(args, *filters.GroupID)
			}
		}
		// Key metadata filters are intentionally current-state filters. Historical
		// member rows otherwise remain visible even after a key is soft-deleted.
		if filters.Status != "" || filters.Search != "" || len(filters.Tags) > 0 {
			conditions = append(conditions, "ak.deleted_at IS NULL")
		}
		if filters.Status != "" {
			conditions = append(conditions, fmt.Sprintf("ak.status = $%d", len(args)+1))
			args = append(args, filters.Status)
		}
		if filters.Search != "" {
			conditions = append(conditions, fmt.Sprintf("(ak.name ILIKE $%d OR ak.key ILIKE $%d)", len(args)+1, len(args)+1))
			args = append(args, "%"+filters.Search+"%")
		}
		if len(filters.Tags) > 0 {
			tagsJSON, err := json.Marshal(filters.Tags)
			if err != nil {
				return nil, nil, err
			}
			conditions = append(conditions, fmt.Sprintf("ak.tags @> $%d::jsonb", len(args)+1))
			args = append(args, string(tagsJSON))
		}
		return conditions, args, nil
	}
	return ownerAnalyticsAPIKeyFilters(conditions, args, filters, "ak")
}

func ownerAnalyticsUsageSelect(prefix string) string {
	if prefix != "" {
		prefix += "."
	}
	return fmt.Sprintf(`
		COUNT(*) as requests,
		COALESCE(SUM(%sinput_tokens), 0) as input_tokens,
		COALESCE(SUM(%soutput_tokens), 0) as output_tokens,
		COALESCE(SUM(%scache_creation_tokens), 0) as cache_creation_tokens,
		COALESCE(SUM(%scache_read_tokens), 0) as cache_read_tokens,
		COALESCE(SUM(%sinput_tokens + %soutput_tokens + %scache_creation_tokens + %scache_read_tokens), 0) as total_tokens,
		COALESCE(SUM(%sactual_cost), 0) as actual_cost
	`, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)
}

func (r *usageLogRepository) GetOwnerAPIKeyAnalyticsSummary(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) (*service.OwnerAPIKeyAnalyticsSummary, error) {
	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT
			` + ownerAnalyticsUsageSelect("ul") + `,
			COUNT(DISTINCT ul.api_key_id) as used_key_count
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		` + buildWhere(conditions)

	summary := &service.OwnerAPIKeyAnalyticsSummary{}
	if err := scanSingleRow(ctx, r.sql, query, args,
		&summary.Requests,
		&summary.InputTokens,
		&summary.OutputTokens,
		&summary.CacheCreationTokens,
		&summary.CacheReadTokens,
		&summary.TotalTokens,
		&summary.ActualCost,
		&summary.UsedKeyCount,
	); err != nil {
		return nil, err
	}

	keyConditions, keyArgs, err := ownerAnalyticsAPIKeyFilters(nil, nil, filters, "ak")
	if err != nil {
		return nil, err
	}
	keyQuery := `
		SELECT
			COUNT(*) FILTER (WHERE ak.status = $` + strconv.Itoa(len(keyArgs)+1) + `) as active_key_count,
			COUNT(*) FILTER (
				WHERE ak.quota > 0
				  AND ak.quota_used >= ak.quota * $` + strconv.Itoa(len(keyArgs)+2) + `
			) as near_quota_key_count,
			COUNT(*) FILTER (
				WHERE (
					ak.rate_limit_5h > 0
					AND ak.window_5h_start IS NOT NULL
					AND ak.window_5h_start >= NOW() - INTERVAL '5 hours'
					AND ak.usage_5h >= ak.rate_limit_5h * $` + strconv.Itoa(len(keyArgs)+2) + `
				) OR (
					ak.rate_limit_1d > 0
					AND ak.window_1d_start IS NOT NULL
					AND ak.window_1d_start >= NOW() - INTERVAL '1 day'
					AND ak.usage_1d >= ak.rate_limit_1d * $` + strconv.Itoa(len(keyArgs)+2) + `
				) OR (
					ak.rate_limit_7d > 0
					AND ak.window_7d_start IS NOT NULL
					AND ak.window_7d_start >= NOW() - INTERVAL '7 days'
					AND ak.usage_7d >= ak.rate_limit_7d * $` + strconv.Itoa(len(keyArgs)+2) + `
				)
			) as near_rate_limit_key_count
		FROM api_keys ak
		` + buildWhere(keyConditions)
	keyArgs = append(keyArgs, service.StatusAPIKeyActive, ownerAnalyticsNearLimitThreshold)
	if err := scanSingleRow(ctx, r.sql, keyQuery, keyArgs,
		&summary.CurrentKeySnapshot.ActiveKeyCount,
		&summary.CurrentKeySnapshot.NearQuotaKeyCount,
		&summary.CurrentKeySnapshot.NearRateLimitKeyCount,
	); err != nil {
		return nil, err
	}
	summary.CurrentKeySnapshot.SnapshotAt = time.Now().UTC()
	return summary, nil
}

func (r *usageLogRepository) ownerAPIKeyAnalyticsTotal(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) (totalKeys int64, totalActualCost float64, err error) {
	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return 0, 0, err
	}
	query := `
		SELECT
			COUNT(DISTINCT ul.api_key_id) as total_keys,
			COALESCE(SUM(ul.actual_cost), 0) as total_actual_cost
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		` + buildWhere(conditions)
	if err := scanSingleRow(ctx, r.sql, query, args, &totalKeys, &totalActualCost); err != nil {
		return 0, 0, err
	}
	return totalKeys, totalActualCost, nil
}

func (r *usageLogRepository) GetOwnerAPIKeyAnalyticsLeaderboard(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) (*service.OwnerAPIKeyLeaderboardResponse, error) {
	limit := ownerAnalyticsLimit(filters.Limit)
	totalKeys, totalActualCost, err := r.ownerAPIKeyAnalyticsTotal(ctx, filters)
	if err != nil {
		return nil, err
	}

	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT
			ak.id,
			ak.name,
			COALESCE(ak.tags, '[]'::jsonb)::text,
			ak.group_id,
			COALESCE(g.name, ''),
			ak.status,
			ak.last_used_at,
			` + ownerAnalyticsUsageSelect("ul") + `
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		LEFT JOIN groups g ON g.id = ak.group_id
		` + buildWhere(conditions) + `
		GROUP BY ak.id, ak.name, ak.tags, ak.group_id, g.name, ak.status, ak.last_used_at
		ORDER BY actual_cost DESC, requests DESC, ak.id ASC
		LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	items := make([]service.OwnerAPIKeyLeaderboardItem, 0, limit)
	ids := make([]int64, 0, limit)
	var displayedActualCost float64
	for rows.Next() {
		var item service.OwnerAPIKeyLeaderboardItem
		var tagsJSON string
		var groupID sql.NullInt64
		var lastUsed sql.NullTime
		if err := rows.Scan(
			&item.APIKeyID,
			&item.KeyName,
			&tagsJSON,
			&groupID,
			&item.GroupName,
			&item.Status,
			&lastUsed,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if groupID.Valid {
			item.GroupID = &groupID.Int64
		}
		if lastUsed.Valid {
			item.LastUsedAt = &lastUsed.Time
		}
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if totalActualCost > 0 {
			item.SharePercent = item.ActualCost / totalActualCost * 100
		}
		displayedActualCost += item.ActualCost
		items = append(items, item)
		ids = append(ids, item.APIKeyID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		previous, err := r.ownerAPIKeyPreviousActualCost(ctx, filters, ids)
		if err != nil {
			return nil, err
		}
		for i := range items {
			prev := previous[items[i].APIKeyID]
			items[i].PreviousActualCost = prev
			if prev > 0 {
				items[i].ChangePercent = (items[i].ActualCost - prev) / prev * 100
			}
		}
	}

	return &service.OwnerAPIKeyLeaderboardResponse{
		Items:               items,
		Total:               totalKeys,
		TotalActualCost:     totalActualCost,
		DisplayedActualCost: displayedActualCost,
	}, nil
}

func (r *usageLogRepository) ownerAPIKeyPreviousActualCost(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters, apiKeyIDs []int64) (map[int64]float64, error) {
	out := make(map[int64]float64, len(apiKeyIDs))
	previousFilters := filters
	previousFilters.EndTime = filters.StartTime
	previousFilters.StartTime = filters.StartTime.Add(-filters.EndTime.Sub(filters.StartTime))
	conditions, args, err := ownerAnalyticsUsageConditions(previousFilters, true)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, fmt.Sprintf("ul.api_key_id = ANY($%d)", len(args)+1))
	args = append(args, pq.Array(apiKeyIDs))
	query := `
		SELECT ul.api_key_id, COALESCE(SUM(ul.actual_cost), 0)
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		` + buildWhere(conditions) + `
		GROUP BY ul.api_key_id
	`
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var apiKeyID int64
		var actualCost float64
		if err := rows.Scan(&apiKeyID, &actualCost); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[apiKeyID] = actualCost
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *usageLogRepository) GetOwnerAPIKeyModelAnalytics(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) ([]service.OwnerModelAnalyticsItem, error) {
	limit := ownerAnalyticsLimit(filters.Limit)
	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	modelExpr := "COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), 'unknown')"
	query := `
		SELECT
			` + modelExpr + ` as model,
			` + ownerAnalyticsUsageSelect("ul") + `
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		` + buildWhere(conditions) + `
		GROUP BY ` + modelExpr + `
		ORDER BY actual_cost DESC, requests DESC, model ASC
		LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := make([]service.OwnerModelAnalyticsItem, 0, limit)
	for rows.Next() {
		var item service.OwnerModelAnalyticsItem
		if err := rows.Scan(
			&item.Model,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *usageLogRepository) GetOwnerAPIKeyGroupAnalytics(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) ([]service.OwnerGroupAnalyticsItem, error) {
	limit := ownerAnalyticsLimit(filters.Limit)
	_, totalActualCost, err := r.ownerAPIKeyAnalyticsTotal(ctx, filters)
	if err != nil {
		return nil, err
	}
	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT
			ul.group_id,
			COALESCE(g.name, ''),
			COUNT(DISTINCT ul.api_key_id) as key_count,
			` + ownerAnalyticsUsageSelect("ul") + `
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		LEFT JOIN groups g ON g.id = ul.group_id
		` + buildWhere(conditions) + `
		GROUP BY ul.group_id, g.name
		ORDER BY actual_cost DESC, requests DESC
		LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := make([]service.OwnerGroupAnalyticsItem, 0, limit)
	for rows.Next() {
		var item service.OwnerGroupAnalyticsItem
		var groupID sql.NullInt64
		if err := rows.Scan(
			&groupID,
			&item.GroupName,
			&item.KeyCount,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if groupID.Valid {
			item.GroupID = &groupID.Int64
		}
		if totalActualCost > 0 {
			item.SharePercent = item.ActualCost / totalActualCost * 100
		}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *usageLogRepository) GetOwnerAPIKeyTagAnalytics(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) ([]service.OwnerTagAnalyticsItem, error) {
	limit := ownerAnalyticsLimit(filters.Limit)
	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT
			tag.value as tag,
			COUNT(DISTINCT ak.id) as key_count,
			` + ownerAnalyticsUsageSelect("ul") + `
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		CROSS JOIN LATERAL (
			SELECT DISTINCT btrim(raw_tag.value) AS value
			FROM jsonb_array_elements_text(COALESCE(ak.tags, '[]'::jsonb)) AS raw_tag(value)
			WHERE btrim(raw_tag.value) <> ''
		) AS tag
		` + buildWhere(conditions) + `
		GROUP BY tag.value
		ORDER BY actual_cost DESC, requests DESC, tag ASC
		LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := make([]service.OwnerTagAnalyticsItem, 0, limit)
	for rows.Next() {
		var item service.OwnerTagAnalyticsItem
		if err := rows.Scan(
			&item.Tag,
			&item.KeyCount,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *usageLogRepository) GetOwnerAPIKeyUsageTrend(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) ([]service.OwnerTrendAnalyticsPoint, error) {
	dateFormat, ok := strictDateFormat(filters.Granularity)
	if !ok {
		return nil, fmt.Errorf("invalid granularity: %s", filters.Granularity)
	}
	timezoneName := filters.TimezoneName
	if timezoneName == "" {
		timezoneName = timezone.Name()
		if timezoneName == "Local" {
			timezoneName = "UTC"
		}
	}
	if _, err := time.LoadLocation(timezoneName); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	conditions, args, err := ownerAnalyticsUsageConditions(filters, true)
	if err != nil {
		return nil, err
	}
	args = append(args, timezoneName)
	query := `
		SELECT
			TO_CHAR(ul.created_at AT TIME ZONE $` + strconv.Itoa(len(args)) + `, '` + dateFormat + `') as date,
			` + ownerAnalyticsUsageSelect("ul") + `
		FROM usage_logs ul
		JOIN api_keys ak ON ul.api_key_id = ak.id
		` + buildWhere(conditions) + `
		GROUP BY date
		ORDER BY date ASC
	`
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := make([]service.OwnerTrendAnalyticsPoint, 0)
	for rows.Next() {
		var item service.OwnerTrendAnalyticsPoint
		if err := rows.Scan(
			&item.Date,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nullUsageScheduleMetaJSON(v *service.UsageScheduleMeta) any {
	if v == nil {
		return nil
	}
	payload, err := json.Marshal(v)
	if err != nil || string(payload) == "{}" {
		return nil
	}
	return string(payload)
}

func usageScheduleMetaFromNullJSON(v sql.NullString) *service.UsageScheduleMeta {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	var out service.UsageScheduleMeta
	if err := json.Unmarshal([]byte(v.String), &out); err != nil {
		return nil
	}
	if out.Provider == "" &&
		out.Layer == "" &&
		!out.StickyPreviousHit &&
		!out.StickySessionHit &&
		out.CandidateCount == 0 &&
		out.TopK == 0 &&
		out.LatencyMs == 0 &&
		out.LoadSkew == 0 &&
		out.SelectedAccountID == 0 &&
		out.SelectedAccountType == "" {
		return nil
	}
	return &out
}
