package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *OpsService) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if filter == nil {
		return nil, infraerrors.BadRequest("OPS_FILTER_REQUIRED", "filter is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_REQUIRED", "start_time/end_time are required")
	}
	if filter.StartTime.After(filter.EndTime) {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_INVALID", "start_time must be <= end_time")
	}

	// Resolve query mode (requested via query param, or DB default).
	filter.QueryMode = s.resolveOpsQueryMode(ctx, filter.QueryMode)

	overview, err := s.opsRepo.GetDashboardOverview(ctx, filter)
	if err != nil && shouldFallbackOpsPreagg(filter, err) {
		rawFilter := cloneOpsFilterWithMode(filter, OpsQueryModeRaw)
		overview, err = s.opsRepo.GetDashboardOverview(ctx, rawFilter)
	}
	if err != nil {
		if errors.Is(err, ErrOpsPreaggregatedNotPopulated) {
			return nil, infraerrors.Conflict("OPS_PREAGG_NOT_READY", "Pre-aggregated ops metrics are not populated yet")
		}
		return nil, err
	}

	// Best-effort system health + jobs; dashboard metrics should still render if these are missing.
	if metrics, err := s.opsRepo.GetLatestSystemMetrics(ctx, 1); err == nil {
		// Attach config-derived limits so the UI can show "current / max" for connection pools.
		// These are best-effort and should never block the dashboard rendering.
		if s != nil && s.cfg != nil {
			if s.cfg.Database.MaxOpenConns > 0 {
				metrics.DBMaxOpenConns = intPtr(s.cfg.Database.MaxOpenConns)
			}
			if s.cfg.Redis.PoolSize > 0 {
				metrics.RedisPoolSize = intPtr(s.cfg.Redis.PoolSize)
			}
		}
		overview.SystemMetrics = metrics
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[Ops] GetLatestSystemMetrics failed: %v", err)
	}

	if heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx); err == nil {
		overview.JobHeartbeats = heartbeats
	} else {
		log.Printf("[Ops] ListJobHeartbeats failed: %v", err)
	}

	// The repository owns the fixed-window counts, while the service owns the
	// administrator-configured threshold. Keep the state decision here so a
	// single platform failure does not incorrectly imply an ongoing incident.
	thresholds, thresholdErr := s.GetMetricThresholds(ctx)
	if thresholdErr != nil {
		log.Printf("[Ops] GetMetricThresholds for current failure state failed: %v", thresholdErr)
		thresholds = defaultOpsMetricThresholds()
	}
	if overview.CurrentWindow != nil {
		overview.CurrentWindow.State = classifyOpsCurrentFailureState(
			overview.CurrentWindow,
			overview.PlatformSLAFailureCount,
			filter.StartTime,
			filter.EndTime,
			thresholds,
		)
	}

	overview.HealthScore = computeDashboardHealthScore(time.Now().UTC(), overview)

	return overview, nil
}

func classifyOpsCurrentFailureState(
	current *OpsCurrentFailureWindow,
	selectedPlatformFailures int64,
	selectedStart time.Time,
	selectedEnd time.Time,
	thresholds *OpsMetricThresholds,
) string {
	if current == nil {
		return "unknown"
	}
	if current.SuccessCount+current.CustomerVisibleFailureCount == 0 {
		return "unknown"
	}

	thresholdPercent := 5.0
	if thresholds != nil && thresholds.RequestErrorRatePercentMax != nil {
		thresholdPercent = *thresholds.RequestErrorRatePercentMax
	}
	requestCountSLA := current.SuccessCount + current.PlatformSLAFailureCount
	platformFailureRatePercent := 0.0
	if requestCountSLA > 0 {
		platformFailureRatePercent = float64(current.PlatformSLAFailureCount) / float64(requestCountSLA) * 100
	}
	if current.PlatformSLAFailureCount > 0 && platformFailureRatePercent >= thresholdPercent {
		return "active"
	}
	// Unknown evidence must prevent a quiet/recovered conclusion, but it must
	// never hide a platform incident that is already proven active above.
	if current.ClassificationUnknownCount > 0 {
		return "unknown"
	}

	selectedOverlapsCurrent := selectedEnd.After(current.StartTime) && selectedStart.Before(current.EndTime)
	if selectedPlatformFailures > 0 && selectedOverlapsCurrent {
		return "recovered"
	}
	return "quiet"
}

func (s *OpsService) resolveOpsQueryMode(ctx context.Context, requested OpsQueryMode) OpsQueryMode {
	if requested.IsValid() {
		// Allow "auto" to be disabled via config until preagg is proven stable in production.
		// Forced `preagg` via query param still works.
		if requested == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
			return OpsQueryModeRaw
		}
		return requested
	}

	mode := OpsQueryModeAuto
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsQueryModeDefault); err == nil {
			mode = ParseOpsQueryMode(raw)
		}
	}

	if mode == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
		return OpsQueryModeRaw
	}
	return mode
}
