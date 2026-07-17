//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/opssql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	migrationfs "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOpsFailureClassificationV2RawPreaggAndDrilldownConserveRows(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	bucketStart := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)
	bucketEnd := bucketStart.Add(time.Hour)
	dayStart := bucketStart.Truncate(24 * time.Hour)
	prefix := "ops-v2-it-" + uuid.NewString()[:12]
	platform := "ops-v2-" + uuid.NewString()[:8]

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM ops_error_logs WHERE request_id LIKE $1`, prefix+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM ops_metrics_hourly WHERE bucket_start = $1`, bucketStart)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM ops_metrics_daily WHERE bucket_date = $1 AND platform = $2`, dayStart, platform)
	})

	insert := func(suffix, domain, category, reason string, customerVisible, slaImpact bool, eventScope string, status int) {
		t.Helper()
		_, err := repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
			RequestID:             fmt.Sprintf("%s-%s", prefix, suffix),
			Platform:              platform,
			ErrorPhase:            "request",
			ErrorType:             "api_error",
			Severity:              "P1",
			StatusCode:            status,
			EventScope:            eventScope,
			CustomerVisible:       customerVisible,
			FailureDomain:         domain,
			FailureCategory:       category,
			FailureReason:         reason,
			ResolutionOwner:       service.OpsResolutionOwnerPlatformOps,
			PoolOwnership:         service.OpsPoolOwnershipPlatform,
			SLAImpact:             service.OpsBool(slaImpact),
			ClassificationVersion: service.OpsFailureClassificationVersion,
			ErrorMessage:          suffix,
			CreatedAt:             bucketStart.Add(10 * time.Minute),
		})
		require.NoError(t, err)
	}

	insert("routing", service.OpsFailureDomainPlatform, service.OpsFailureCategoryRouting, service.OpsFailureReasonNoAvailableAccounts, true, true, service.OpsEventScopeRequestTerminal, 503)
	insert("dependency", service.OpsFailureDomainPlatform, service.OpsFailureCategoryDependency, service.OpsFailureReasonDatabaseUnavailable, true, true, service.OpsEventScopeRequestTerminal, 500)
	insert("customer", service.OpsFailureDomainCustomer, service.OpsFailureCategoryQuota, service.OpsFailureReasonAPIKeyQuotaExhausted, true, false, service.OpsEventScopeRequestTerminal, 429)
	insert("recovered", service.OpsFailureDomainUpstream, service.OpsFailureCategoryRateLimit, service.OpsFailureReasonProviderRateLimited, false, false, service.OpsEventScopeUpstreamAttemptRecovered, 200)

	raw, err := repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{
		StartTime: bucketStart,
		EndTime:   bucketEnd,
		Platform:  platform,
		QueryMode: service.OpsQueryModeRaw,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), raw.CustomerVisibleFailureCount)
	require.Equal(t, int64(2), raw.PlatformSLAFailureCount)
	require.Equal(t, int64(1), raw.SLAExcludedFailureCount)

	require.NoError(t, repo.UpsertHourlyMetrics(ctx, bucketStart, bucketEnd))
	preagg, err := repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{
		StartTime: bucketStart,
		EndTime:   bucketEnd,
		Platform:  platform,
		QueryMode: service.OpsQueryModePreagg,
	})
	require.NoError(t, err)
	require.Equal(t, raw.CustomerVisibleFailureCount, preagg.CustomerVisibleFailureCount)
	require.Equal(t, raw.PlatformSLAFailureCount, preagg.PlatformSLAFailureCount)
	require.Equal(t, raw.SLAExcludedFailureCount, preagg.SLAExcludedFailureCount)
	require.Equal(t, raw.ClassificationUnknownCount, preagg.ClassificationUnknownCount)
	require.Equal(t, raw.FailureBreakdown, preagg.FailureBreakdown)

	require.NoError(t, repo.UpsertDailyMetrics(ctx, dayStart, dayStart.Add(24*time.Hour)))
	var dailyClassificationVersion int
	var dailyVisible, dailyPlatformSLA, dailyExcluded int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT classification_version, customer_visible_failure_count, platform_sla_failure_count, sla_excluded_failure_count
		FROM ops_metrics_daily
		WHERE bucket_date = $1 AND platform = $2 AND group_id IS NULL
	`, dayStart, platform).Scan(&dailyClassificationVersion, &dailyVisible, &dailyPlatformSLA, &dailyExcluded))
	require.Equal(t, int(service.OpsFailureClassificationVersion), dailyClassificationVersion)
	require.Equal(t, raw.CustomerVisibleFailureCount, dailyVisible)
	require.Equal(t, raw.PlatformSLAFailureCount, dailyPlatformSLA)
	require.Equal(t, raw.SLAExcludedFailureCount, dailyExcluded)

	// Simulate a stale application replica writing one legacy row after the v2
	// migration. The aggregate bucket must not claim v2 completeness, and the
	// raw fallback must surface the row as unclassified instead of hiding it.
	_, err = repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
		RequestID:         prefix + "-legacy",
		Platform:          platform,
		ErrorPhase:        "internal",
		ErrorType:         "api_error",
		Severity:          "P1",
		StatusCode:        500,
		ErrorMessage:      "legacy replica failure",
		IsBusinessLimited: false,
		CreatedAt:         bucketStart.Add(20 * time.Minute),
	})
	require.NoError(t, err)

	// A rolling-upgrade replica can still write an HTTP 200 stream-terminal
	// failure without v2 columns. It is customer-visible, but SLA attribution
	// must remain unknown because the legacy row lacks a reliable terminal
	// ownership decision.
	_, err = repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
		RequestID:         prefix + "-legacy-stream-terminal",
		Platform:          platform,
		ErrorPhase:        "request",
		ErrorType:         "api_error",
		ErrorOwner:        "provider",
		Severity:          "P1",
		StatusCode:        200,
		Stream:            true,
		ErrorMessage:      "legacy stream terminal failure",
		IsBusinessLimited: false,
		CreatedAt:         bucketStart.Add(25 * time.Minute),
	})
	require.NoError(t, err)

	// The strict recovered marker is the inverse case: it is useful provider
	// health evidence but must not enter customer-visible failure counts/lists.
	_, err = repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
		RequestID:         prefix + "-legacy-recovered",
		Platform:          platform,
		ErrorPhase:        "upstream",
		ErrorType:         "upstream_error",
		ErrorOwner:        "provider",
		Severity:          "P2",
		StatusCode:        200,
		Stream:            true,
		ErrorMessage:      "Recovered upstream error after failover",
		IsBusinessLimited: false,
		CreatedAt:         bucketStart.Add(30 * time.Minute),
	})
	require.NoError(t, err)

	legacyRaw, err := repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{
		StartTime: bucketStart,
		EndTime:   bucketEnd,
		Platform:  platform,
		QueryMode: service.OpsQueryModeRaw,
	})
	require.NoError(t, err)
	require.Equal(t, int64(5), legacyRaw.CustomerVisibleFailureCount)
	require.Equal(t, int64(3), legacyRaw.PlatformSLAFailureCount)
	require.Equal(t, int64(2), legacyRaw.ClassificationUnknownCount)
	var terminalBreakdownTotal int64
	for _, item := range legacyRaw.FailureBreakdown {
		if item.Domain == service.OpsFailureDomainUpstream && item.Category == "recovered_attempt" {
			continue
		}
		terminalBreakdownTotal += item.Count
	}
	require.Equal(t, legacyRaw.CustomerVisibleFailureCount, terminalBreakdownTotal)

	visibleErrors, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{
		StartTime: &bucketStart,
		EndTime:   &bucketEnd,
		Platform:  platform,
		View:      "all",
		Page:      1,
		PageSize:  20,
	})
	require.NoError(t, err)
	require.Equal(t, 5, visibleErrors.Total)
	visibleRequestIDs := make(map[string]bool, len(visibleErrors.Errors))
	for _, item := range visibleErrors.Errors {
		visibleRequestIDs[item.RequestID] = true
	}
	require.True(t, visibleRequestIDs[prefix+"-legacy-stream-terminal"])
	require.False(t, visibleRequestIDs[prefix+"-legacy-recovered"])

	// The previously built bucket still says v2 at this point. Raw evidence must
	// nevertheless prevent a stale old-version aggregate from being trusted.
	_, err = repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{
		StartTime: bucketStart,
		EndTime:   bucketEnd,
		Platform:  platform,
		QueryMode: service.OpsQueryModePreagg,
	})
	require.ErrorIs(t, err, service.ErrOpsPreaggregatedNotPopulated)

	require.NoError(t, repo.UpsertHourlyMetrics(ctx, bucketStart, bucketEnd))
	_, err = repo.GetDashboardOverview(ctx, &service.OpsDashboardFilter{
		StartTime: bucketStart,
		EndTime:   bucketEnd,
		Platform:  platform,
		QueryMode: service.OpsQueryModePreagg,
	})
	require.ErrorIs(t, err, service.ErrOpsPreaggregatedNotPopulated)

	require.NoError(t, repo.UpsertDailyMetrics(ctx, dayStart, dayStart.Add(24*time.Hour)))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT classification_version, customer_visible_failure_count, platform_sla_failure_count, sla_excluded_failure_count
		FROM ops_metrics_daily
		WHERE bucket_date = $1 AND platform = $2 AND group_id IS NULL
	`, dayStart, platform).Scan(&dailyClassificationVersion, &dailyVisible, &dailyPlatformSLA, &dailyExcluded))
	require.Zero(t, dailyClassificationVersion)
	require.Equal(t, legacyRaw.CustomerVisibleFailureCount, dailyVisible)
	require.Equal(t, legacyRaw.PlatformSLAFailureCount, dailyPlatformSLA)
	require.Equal(t, legacyRaw.SLAExcludedFailureCount, dailyExcluded)

	nonRouting, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{
		StartTime:       &bucketStart,
		EndTime:         &bucketEnd,
		Platform:        platform,
		FailureDomain:   service.OpsFailureDomainPlatform,
		FailureCategory: service.OpsFailureBreakdownCategoryNonRouting,
		View:            "all",
		Page:            1,
		PageSize:        20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, nonRouting.Total)
	require.Len(t, nonRouting.Errors, 1)
	require.Equal(t, service.OpsFailureCategoryDependency, nonRouting.Errors[0].FailureCategory)
}

func TestOpsFailureClassificationV2BackfillMatchesWriterSemantics(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	prefix := "ops-v2-backfill-" + uuid.NewString()[:12]
	createdAt := time.Now().UTC().Add(-time.Hour)
	rows := []struct {
		suffix     string
		phase      string
		errType    string
		owner      string
		status     int
		stream     bool
		message    string
		domain     string
		category   string
		reason     string
		resolution string
		pool       string
		slaImpact  bool
	}{
		{
			suffix: "cyber", phase: "request", errType: "cyber_policy", owner: "provider", status: 200, stream: true,
			message: "cyber_policy: blocked", domain: service.OpsFailureDomainCustomer, category: service.OpsFailureCategoryPermission,
			reason: service.OpsFailureReasonEndpointNotAllowed, resolution: service.OpsResolutionOwnerCustomer,
			pool: service.OpsPoolOwnershipUnknown, slaImpact: false,
		},
		{
			suffix: "cyber-session", phase: "request", errType: "cyber_policy_session_blocked", owner: "platform", status: 403,
			message: "cyber_policy_session_blocked", domain: service.OpsFailureDomainCustomer, category: service.OpsFailureCategoryPermission,
			reason: service.OpsFailureReasonEndpointNotAllowed, resolution: service.OpsResolutionOwnerCustomer,
			pool: service.OpsPoolOwnershipUnknown, slaImpact: false,
		},
		{
			suffix: "provider-balance", phase: "upstream", errType: "upstream_error", owner: "provider", status: 403,
			message: "insufficient balance", domain: service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryBalance,
			reason: service.OpsFailureReasonProviderBalanceExhausted, resolution: service.OpsResolutionOwnerPlatformOps,
			pool: service.OpsPoolOwnershipPlatform, slaImpact: true,
		},
		{
			suffix: "provider-404", phase: "upstream", errType: "upstream_error", owner: "provider", status: 404,
			message: "provider rejected request", domain: service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryProtocol,
			reason: service.OpsFailureReasonProvider4xx, resolution: service.OpsResolutionOwnerPlatformOps,
			pool: service.OpsPoolOwnershipPlatform, slaImpact: true,
		},
		{
			suffix: "provider-500", phase: "upstream", errType: "upstream_error", owner: "provider", status: 500,
			message: "provider internal error", domain: service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryInternal,
			reason: service.OpsFailureReasonProvider5xx, resolution: service.OpsResolutionOwnerPlatformOps,
			pool: service.OpsPoolOwnershipPlatform, slaImpact: true,
		},
		{
			suffix: "provider-unknown", phase: "upstream", errType: "upstream_error", owner: "provider", status: 200,
			message: "provider terminal error without status", domain: service.OpsFailureDomainUpstream, category: service.OpsFailureCategoryUnknown,
			reason: service.OpsFailureReasonProviderErrorUnknown, resolution: service.OpsResolutionOwnerPlatformOps,
			pool: service.OpsPoolOwnershipPlatform, slaImpact: true,
		},
	}

	for _, row := range rows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ops_error_logs (
				request_id, error_phase, error_type, error_owner, status_code,
				stream, error_message, is_business_limited, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,FALSE,$8)
		`, prefix+"-"+row.suffix, row.phase, row.errType, row.owner, row.status, row.stream, row.message, createdAt)
		require.NoError(t, err)
	}

	migrationSQL, err := migrationfs.FS.ReadFile("192_ops_failure_classification_v2.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	for _, expected := range rows {
		var (
			domain, category, reason, resolution, pool string
			slaImpact                                  bool
			version                                    int
		)
		err = tx.QueryRowContext(ctx, `
			SELECT failure_domain, failure_category, failure_reason,
			       resolution_owner, pool_ownership, sla_impact, classification_version
			FROM ops_error_logs
			WHERE request_id = $1
		`, prefix+"-"+expected.suffix).Scan(&domain, &category, &reason, &resolution, &pool, &slaImpact, &version)
		require.NoError(t, err)
		require.Equal(t, expected.domain, domain, expected.suffix)
		require.Equal(t, expected.category, category, expected.suffix)
		require.Equal(t, expected.reason, reason, expected.suffix)
		require.Equal(t, expected.resolution, resolution, expected.suffix)
		require.Equal(t, expected.pool, pool, expected.suffix)
		require.Equal(t, expected.slaImpact, slaImpact, expected.suffix)
		require.Equal(t, int(service.OpsFailureClassificationVersion), version, expected.suffix)
	}
}

func TestOpsFailureClassificationV2IndexesMatchPlanner(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`)
	require.NoError(t, err)

	start := time.Now().UTC().Add(-24 * time.Hour)
	tests := []struct {
		name      string
		query     string
		indexName string
	}{
		{
			name:      "customer-visible compatibility",
			query:     `SELECT id FROM ops_error_logs WHERE created_at >= $1 AND ` + opssql.CustomerVisible(""),
			indexName: "idx_ops_error_logs_customer_visible_time_v2",
		},
		{
			name:      "legacy fallback probe",
			query:     `SELECT id FROM ops_error_logs WHERE created_at >= $1 AND ` + opssql.LegacyClassification(""),
			indexName: "idx_ops_error_logs_legacy_classification_time_v2",
		},
		{
			name:      "SLA impact expression",
			query:     `SELECT id FROM ops_error_logs WHERE created_at >= $1 AND (` + opssql.SLAImpact("") + `) IS TRUE`,
			indexName: "idx_ops_error_logs_sla_impact_time_v2",
		},
		{
			name:      "failure reason dimension",
			query:     `SELECT id FROM ops_error_logs WHERE failure_reason = 'provider_5xx' AND created_at >= $1`,
			indexName: "idx_ops_error_logs_failure_reason_time_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainOpsQuery(t, ctx, tx, tt.query, start)
			require.Contains(t, plan, tt.indexName)
		})
	}
}

func explainOpsQuery(t *testing.T, ctx context.Context, tx interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query string, args ...any) string {
	t.Helper()
	rows, err := tx.QueryContext(ctx, "EXPLAIN (COSTS OFF) "+query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}
