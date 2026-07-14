package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageLogRepository) ListOwnerUsageMembers(ctx context.Context, ownerID int64) (results []service.OwnerUsageMember, err error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			em.id,
			em.member_code,
			em.name,
			em.status,
			(em.deleted_at IS NOT NULL) AS archived,
			COUNT(ak.id) AS key_count,
			em.monthly_limit_usd,
			em.deleted_at
		FROM enterprise_members em
		LEFT JOIN api_keys ak
		  ON ak.member_id = em.id
		 AND ak.user_id = em.enterprise_user_id
		 AND ak.deleted_at IS NULL
		WHERE em.enterprise_user_id = $1
		  AND em.removed_at IS NULL
		GROUP BY em.id
		ORDER BY (em.deleted_at IS NOT NULL) ASC, LOWER(em.name) ASC, em.id ASC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]service.OwnerUsageMember, 0)
	for rows.Next() {
		var item service.OwnerUsageMember
		var deletedAt sql.NullTime
		if err = rows.Scan(
			&item.ID,
			&item.MemberCode,
			&item.Name,
			&item.Status,
			&item.Archived,
			&item.KeyCount,
			&item.MonthlyLimitUSD,
			&deletedAt,
		); err != nil {
			return nil, err
		}
		if deletedAt.Valid {
			value := deletedAt.Time
			item.DeletedAt = &value
		}
		results = append(results, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *usageLogRepository) ValidateOwnerUsageMember(ctx context.Context, ownerID, memberID int64) error {
	var exists bool
	if err := scanSingleRow(ctx, r.sql, `
		SELECT EXISTS (
			SELECT 1
			FROM enterprise_members
			WHERE id = $1 AND enterprise_user_id = $2
		)
	`, []any{memberID, ownerID}, &exists); err != nil {
		return err
	}
	if !exists {
		return service.ErrEnterpriseMemberNotFound
	}
	return nil
}

func ownerMemberUsageConditions(filters service.OwnerAPIKeyAnalyticsFilters, startTime, endTime time.Time) ([]string, []any, error) {
	if filters.UserID <= 0 {
		return nil, nil, fmt.Errorf("owner member analytics requires user id")
	}
	conditions := []string{
		"ul.user_id = $1",
		"ul.created_at >= $2",
		"ul.created_at < $3",
	}
	args := []any{filters.UserID, startTime, endTime}
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
	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM api_keys filter_key
				WHERE filter_key.id = ul.api_key_id
				  AND filter_key.user_id = ul.user_id
				  AND filter_key.deleted_at IS NULL
				  AND filter_key.status = $%d
			)
		`, len(args)+1))
		args = append(args, filters.Status)
	}
	if len(filters.Tags) > 0 {
		tagsJSON, err := json.Marshal(filters.Tags)
		if err != nil {
			return nil, nil, err
		}
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM api_keys filter_key
				WHERE filter_key.id = ul.api_key_id
				  AND filter_key.user_id = ul.user_id
				  AND filter_key.deleted_at IS NULL
				  AND filter_key.tags @> $%d::jsonb
			)
		`, len(args)+1))
		args = append(args, string(tagsJSON))
	}
	return conditions, args, nil
}

func currentEnterpriseBudgetPeriodStart(now time.Time) string {
	location, err := time.LoadLocation(enterpriseBudgetTimezone())
	if err != nil {
		location = time.FixedZone(enterpriseBudgetTimezone(), 8*60*60)
	}
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location).Format("2006-01-02")
}

func (r *usageLogRepository) GetOwnerMemberAnalyticsLeaderboard(ctx context.Context, filters service.OwnerAPIKeyAnalyticsFilters) (*service.OwnerMemberLeaderboardResponse, error) {
	limit := ownerAnalyticsLimit(filters.Limit)
	if filters.MemberID == nil && strings.TrimSpace(filters.MemberScope) == usagestats.MemberScopeUnassigned {
		return &service.OwnerMemberLeaderboardResponse{Items: make([]service.OwnerMemberLeaderboardItem, 0)}, nil
	}

	memberUsageFilters := filters
	if memberUsageFilters.MemberID == nil {
		memberUsageFilters.MemberScope = usagestats.MemberScopeAssigned
	}
	currentConditions, args, err := ownerMemberUsageConditions(memberUsageFilters, filters.StartTime, filters.EndTime)
	if err != nil {
		return nil, err
	}
	previousStart := filters.StartTime.Add(-filters.EndTime.Sub(filters.StartTime))
	previousConditions, previousArgs, err := ownerMemberUsageConditions(memberUsageFilters, previousStart, filters.StartTime)
	if err != nil {
		return nil, err
	}
	previousConditions = rebindSQLConditions(previousConditions, len(args))
	args = append(args, previousArgs...)

	keyCountsOwnerPlaceholder := len(args) + 1
	args = append(args, filters.UserID)
	memberScopeOwnerPlaceholder := len(args) + 1
	args = append(args, filters.UserID)

	memberScopeConditions := []string{fmt.Sprintf("em.enterprise_user_id = $%d", memberScopeOwnerPlaceholder)}
	if filters.MemberID != nil {
		memberScopeConditions = append(memberScopeConditions, fmt.Sprintf("em.id = $%d", len(args)+1))
		args = append(args, *filters.MemberID)
	} else if strings.TrimSpace(filters.MemberScope) == usagestats.MemberScopeUnassigned {
		memberScopeConditions = append(memberScopeConditions, "FALSE")
	}

	budgetPeriodPlaceholder := len(args) + 1
	args = append(args, currentEnterpriseBudgetPeriodStart(time.Now()))

	searchWhere := ""
	if filters.Search != "" {
		searchWhere = fmt.Sprintf(`
			WHERE (
				COALESCE(em.name, current_usage.member_name_snapshot, '') ILIKE $%d
				OR COALESCE(em.member_code, current_usage.member_code_snapshot, '') ILIKE $%d
			  )
		`, len(args)+1, len(args)+1)
		args = append(args, "%"+filters.Search+"%")
	}

	limitPlaceholder := len(args) + 1
	args = append(args, limit)
	query := `
		WITH current_usage AS (
			SELECT
				ul.member_id,
				COALESCE(
					(ARRAY_AGG(NULLIF(ul.member_code_snapshot, '') ORDER BY ul.created_at DESC)
						FILTER (WHERE NULLIF(ul.member_code_snapshot, '') IS NOT NULL))[1],
					''
				) AS member_code_snapshot,
				COALESCE(
					(ARRAY_AGG(NULLIF(ul.member_name_snapshot, '') ORDER BY ul.created_at DESC)
						FILTER (WHERE NULLIF(ul.member_name_snapshot, '') IS NOT NULL))[1],
					''
				) AS member_name_snapshot,
				COUNT(*) AS requests,
				COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
				COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
				COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
				COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
				COALESCE(SUM(ul.actual_cost), 0) AS actual_cost,
				MAX(ul.created_at) AS last_used_at
			FROM usage_logs ul
			` + buildWhere(currentConditions) + `
			GROUP BY ul.member_id
		),
		previous_usage AS (
			SELECT ul.member_id, COALESCE(SUM(ul.actual_cost), 0) AS actual_cost
			FROM usage_logs ul
			` + buildWhere(previousConditions) + `
			GROUP BY ul.member_id
		),
		key_counts AS (
			SELECT member_id, COUNT(*) AS key_count
			FROM api_keys
			WHERE user_id = $` + strconv.Itoa(keyCountsOwnerPlaceholder) + `
			  AND deleted_at IS NULL
			GROUP BY member_id
		),
		member_scope AS (
			SELECT em.id AS member_id
			FROM enterprise_members em
			WHERE ` + strings.Join(memberScopeConditions, " AND ") + `
		),
		ranked AS (
			SELECT
				scope.member_id,
				COALESCE(em.member_code, current_usage.member_code_snapshot, '') AS member_code,
				COALESCE(em.name, current_usage.member_name_snapshot, '') AS member_name,
				COALESCE(em.status, 'archived') AS status,
				COALESCE(em.deleted_at IS NOT NULL, false) AS archived,
				COALESCE(key_counts.key_count, 0) AS key_count,
				COALESCE(em.monthly_limit_usd, 0) AS monthly_limit_usd,
				COALESCE(period.used_usd, 0) AS current_used_usd,
				COALESCE(period.reserved_usd, 0) AS current_reserved_usd,
				COALESCE(current_usage.requests, 0) AS requests,
				COALESCE(current_usage.input_tokens, 0) AS input_tokens,
				COALESCE(current_usage.output_tokens, 0) AS output_tokens,
				COALESCE(current_usage.cache_creation_tokens, 0) AS cache_creation_tokens,
				COALESCE(current_usage.cache_read_tokens, 0) AS cache_read_tokens,
				COALESCE(current_usage.total_tokens, 0) AS total_tokens,
				COALESCE(current_usage.actual_cost, 0) AS actual_cost,
				COALESCE(previous_usage.actual_cost, 0) AS previous_actual_cost,
				current_usage.last_used_at
			FROM member_scope scope
			LEFT JOIN enterprise_members em
			  ON em.id = scope.member_id
			 AND em.enterprise_user_id = $` + strconv.Itoa(memberScopeOwnerPlaceholder) + `
			LEFT JOIN current_usage ON current_usage.member_id IS NOT DISTINCT FROM scope.member_id
			LEFT JOIN previous_usage ON previous_usage.member_id IS NOT DISTINCT FROM scope.member_id
			LEFT JOIN key_counts ON key_counts.member_id IS NOT DISTINCT FROM scope.member_id
			LEFT JOIN enterprise_member_budget_periods period
			  ON period.member_id = scope.member_id
			 AND period.period_start = $` + strconv.Itoa(budgetPeriodPlaceholder) + `::date
			` + searchWhere + `
		)
		SELECT
			member_id,
			member_code,
			member_name,
			status,
			archived,
			key_count,
			monthly_limit_usd,
			current_used_usd,
			current_reserved_usd,
			requests,
			input_tokens,
			output_tokens,
			cache_creation_tokens,
			cache_read_tokens,
			total_tokens,
			actual_cost,
			previous_actual_cost,
			last_used_at,
			COUNT(*) OVER () AS total_items,
			COUNT(*) OVER () AS member_count,
			COUNT(*) FILTER (
				WHERE NOT archived
				  AND monthly_limit_usd > 0
				  AND current_used_usd + current_reserved_usd >= monthly_limit_usd * 0.8
			) OVER () AS budget_risk_member_count,
			COALESCE(SUM(current_reserved_usd) OVER (), 0) AS total_reserved_usd,
			COALESCE(SUM(actual_cost) OVER (), 0) AS total_actual_cost
		FROM ranked
		ORDER BY actual_cost DESC, requests DESC, member_name ASC, member_id ASC NULLS LAST
		LIMIT $` + strconv.Itoa(limitPlaceholder)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &service.OwnerMemberLeaderboardResponse{Items: make([]service.OwnerMemberLeaderboardItem, 0, limit)}
	for rows.Next() {
		var item service.OwnerMemberLeaderboardItem
		var memberID sql.NullInt64
		var lastUsedAt sql.NullTime
		var total int64
		var memberCount int64
		var budgetRiskMemberCount int64
		var totalReservedUSD float64
		var totalActualCost float64
		if err := rows.Scan(
			&memberID,
			&item.MemberCode,
			&item.MemberName,
			&item.Status,
			&item.Archived,
			&item.KeyCount,
			&item.MonthlyLimitUSD,
			&item.CurrentUsedUSD,
			&item.CurrentReservedUSD,
			&item.Requests,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.ActualCost,
			&item.PreviousActualCost,
			&lastUsedAt,
			&total,
			&memberCount,
			&budgetRiskMemberCount,
			&totalReservedUSD,
			&totalActualCost,
		); err != nil {
			return nil, err
		}
		if memberID.Valid {
			value := memberID.Int64
			item.MemberID = &value
		}
		if lastUsedAt.Valid {
			value := lastUsedAt.Time
			item.LastUsedAt = &value
		}
		if totalActualCost > 0 {
			item.SharePercent = item.ActualCost / totalActualCost * 100
		}
		if item.PreviousActualCost > 0 {
			item.ChangePercent = (item.ActualCost - item.PreviousActualCost) / item.PreviousActualCost * 100
		}
		result.Total = total
		result.MemberCount = memberCount
		result.BudgetRiskMemberCount = budgetRiskMemberCount
		result.TotalReservedUSD = totalReservedUSD
		result.TotalActualCost = totalActualCost
		result.DisplayedActualCost += item.ActualCost
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func rebindSQLConditions(conditions []string, offset int) []string {
	if offset <= 0 {
		return conditions
	}
	out := make([]string, len(conditions))
	for i, condition := range conditions {
		out[i] = rebindPostgresPlaceholders(condition, offset)
	}
	return out
}

var postgresPlaceholderPattern = regexp.MustCompile(`\$(\d+)`)

func rebindPostgresPlaceholders(value string, offset int) string {
	if offset <= 0 {
		return value
	}
	return postgresPlaceholderPattern.ReplaceAllStringFunc(value, func(placeholder string) string {
		index, err := strconv.Atoi(placeholder[1:])
		if err != nil {
			return placeholder
		}
		return "$" + strconv.Itoa(index+offset)
	})
}
