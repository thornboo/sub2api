package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dailyUsageRepoStub struct {
	service.UsageLogRepository
	trend      []usagestats.TrendDataPoint
	modelStats []usagestats.ModelStat

	called      bool
	startTime   time.Time
	endTime     time.Time
	granularity string
	userID      int64
	apiKeyID    int64

	trendCalled      bool
	trendStartTime   time.Time
	trendEndTime     time.Time
	trendGranularity string
	trendTimezone    string
	trendUserID      int64
	trendAPIKeyID    int64

	modelCalled           bool
	modelStartTime        time.Time
	modelEndTime          time.Time
	modelUserID           int64
	modelAPIKeyID         int64
	dashboardModelCalled  bool
	dashboardModelFilters usagestats.UsageLogFilters
	dashboardModelSource  string

	ownerFilters     service.OwnerAPIKeyAnalyticsFilters
	ownerSummary     *service.OwnerAPIKeyAnalyticsSummary
	ownerLeaderboard *service.OwnerAPIKeyLeaderboardResponse
	ownerModels      []service.OwnerModelAnalyticsItem
	ownerGroups      []service.OwnerGroupAnalyticsItem
	ownerTags        []service.OwnerTagAnalyticsItem
	ownerTrend       []service.OwnerTrendAnalyticsPoint
}

func (s *dailyUsageRepoStub) GetUsageTrendWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	granularity string,
	userID, apiKeyID, accountID, groupID int64,
	model string,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.TrendDataPoint, error) {
	s.called = true
	s.startTime = startTime
	s.endTime = endTime
	s.granularity = granularity
	s.userID = userID
	s.apiKeyID = apiKeyID
	return s.trend, nil
}

func (s *dailyUsageRepoStub) GetAPIKeyUsageTrendForUser(
	ctx context.Context,
	userID, apiKeyID int64,
	startTime, endTime time.Time,
	granularity, timezoneName string,
) ([]usagestats.TrendDataPoint, error) {
	s.trendCalled = true
	s.trendStartTime = startTime
	s.trendEndTime = endTime
	s.trendGranularity = granularity
	s.trendTimezone = timezoneName
	s.trendUserID = userID
	s.trendAPIKeyID = apiKeyID
	return s.trend, nil
}

func (s *dailyUsageRepoStub) GetModelStatsWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	userID, apiKeyID, accountID, groupID int64,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.ModelStat, error) {
	s.modelCalled = true
	s.modelStartTime = startTime
	s.modelEndTime = endTime
	s.modelUserID = userID
	s.modelAPIKeyID = apiKeyID
	return s.modelStats, nil
}

func (s *dailyUsageRepoStub) GetUserModelStats(
	ctx context.Context,
	userID int64,
	startTime, endTime time.Time,
) ([]usagestats.ModelStat, error) {
	return s.modelStats, nil
}

func (s *dailyUsageRepoStub) GetModelStatsWithUsageFiltersBySource(
	_ context.Context,
	_, _ time.Time,
	filters usagestats.UsageLogFilters,
	source string,
) ([]usagestats.ModelStat, error) {
	s.dashboardModelCalled = true
	s.dashboardModelFilters = filters
	s.dashboardModelSource = source
	return s.modelStats, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyAnalyticsSummary(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) (*service.OwnerAPIKeyAnalyticsSummary, error) {
	s.ownerFilters = filters
	if s.ownerSummary != nil {
		return s.ownerSummary, nil
	}
	return &service.OwnerAPIKeyAnalyticsSummary{}, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyAnalyticsLeaderboard(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) (*service.OwnerAPIKeyLeaderboardResponse, error) {
	s.ownerFilters = filters
	if s.ownerLeaderboard != nil {
		return s.ownerLeaderboard, nil
	}
	return &service.OwnerAPIKeyLeaderboardResponse{}, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyModelAnalytics(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) ([]service.OwnerModelAnalyticsItem, error) {
	s.ownerFilters = filters
	return s.ownerModels, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyGroupAnalytics(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) ([]service.OwnerGroupAnalyticsItem, error) {
	s.ownerFilters = filters
	return s.ownerGroups, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyTagAnalytics(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) ([]service.OwnerTagAnalyticsItem, error) {
	s.ownerFilters = filters
	return s.ownerTags, nil
}

func (s *dailyUsageRepoStub) GetOwnerAPIKeyUsageTrend(
	ctx context.Context,
	filters service.OwnerAPIKeyAnalyticsFilters,
) ([]service.OwnerTrendAnalyticsPoint, error) {
	s.ownerFilters = filters
	return s.ownerTrend, nil
}

type dailyUsageAPIKeyRepoStub struct {
	service.APIKeyRepository
	keys map[int64]*service.APIKey
}

func (s *dailyUsageAPIKeyRepoStub) GetByID(ctx context.Context, id int64) (*service.APIKey, error) {
	key, ok := s.keys[id]
	if !ok {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *key
	return &clone, nil
}

func newDailyUsageTestRouter(usageRepo *dailyUsageRepoStub, apiKeyRepo *dailyUsageAPIKeyRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(usageRepo, nil, nil, nil)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, apiKeySvc, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/usage/analytics/summary", handler.GetOwnerAPIKeyAnalyticsSummary)
	router.GET("/usage/analytics/leaderboard", handler.GetOwnerAPIKeyAnalyticsLeaderboard)
	router.GET("/usage/analytics/models", handler.GetOwnerAPIKeyModelAnalytics)
	router.GET("/usage/analytics/groups", handler.GetOwnerAPIKeyGroupAnalytics)
	router.GET("/usage/analytics/tags", handler.GetOwnerAPIKeyTagAnalytics)
	router.GET("/usage/analytics/trend", handler.GetOwnerAPIKeyUsageTrend)
	router.GET("/user/api-keys/:id/usage/daily", handler.GetMyAPIKeyDailyUsage)
	router.GET("/user/api-keys/:id/usage/trend", handler.GetMyAPIKeyUsageTrend)
	router.GET("/user/api-keys/:id/usage/models", handler.GetMyAPIKeyModelStats)
	router.GET("/usage/dashboard/models", handler.DashboardModels)
	return router
}

type dailyUsageHandlerResponse struct {
	Code int `json:"code"`
	Data struct {
		Items []usagestats.APIKeyDailyUsagePoint `json:"items"`
		Days  int                                `json:"days"`
	} `json:"data"`
}

type apiKeyUsageTrendHandlerResponse struct {
	Code int `json:"code"`
	Data struct {
		Items       []usagestats.TrendDataPoint `json:"items"`
		Granularity string                      `json:"granularity"`
		StartDate   string                      `json:"start_date"`
		EndDate     string                      `json:"end_date"`
		Timezone    string                      `json:"timezone"`
	} `json:"data"`
}

func TestGetMyAPIKeyDailyUsageRejectsCrossUserAccess(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 99, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/daily?days=30", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, usageRepo.called)
}

func TestGetMyAPIKeyDailyUsageRejectsInvalidDays(t *testing.T) {
	for _, path := range []string{
		"/user/api-keys/7/usage/daily?days=0",
		"/user/api-keys/7/usage/daily?days=91",
	} {
		t.Run(path, func(t *testing.T) {
			usageRepo := &dailyUsageRepoStub{}
			apiKeyRepo := &dailyUsageAPIKeyRepoStub{
				keys: map[int64]*service.APIKey{
					7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
				},
			}
			router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.False(t, usageRepo.called)
		})
	}
}

func TestGetMyAPIKeyDailyUsageReturnsEmptyData(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{trend: []usagestats.TrendDataPoint{}}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/daily", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dailyUsageHandlerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, 30, got.Data.Days)
	require.Empty(t, got.Data.Items)
}

func TestGetMyAPIKeyDailyUsageAggregatesByDayForOwnedKey(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		trend: []usagestats.TrendDataPoint{
			{
				Date:                "2026-05-19",
				Requests:            3,
				InputTokens:         10,
				OutputTokens:        20,
				CacheCreationTokens: 4,
				CacheReadTokens:     6,
				TotalTokens:         40,
				Cost:                0.5,
				ActualCost:          0.4,
			},
		},
	}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/daily?days=7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.called)
	require.Equal(t, "day", usageRepo.granularity)
	require.Equal(t, int64(42), usageRepo.userID)
	require.Equal(t, int64(7), usageRepo.apiKeyID)
	require.True(t, usageRepo.startTime.Before(usageRepo.endTime))

	var got dailyUsageHandlerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, 7, got.Data.Days)
	require.Len(t, got.Data.Items, 1)
	require.Equal(t, usagestats.APIKeyDailyUsagePoint{
		Date:             "2026-05-19",
		Requests:         3,
		InputTokens:      10,
		OutputTokens:     20,
		CacheReadTokens:  6,
		CacheWriteTokens: 4,
		TotalTokens:      40,
		Cost:             0.5,
		ActualCost:       0.4,
	}, got.Data.Items[0])
}

func TestGetMyAPIKeyUsageTrendRejectsCrossUserAccess(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 99, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/trend?granularity=hour", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, usageRepo.trendCalled)
}

func TestGetMyAPIKeyUsageTrendRejectsInvalidGranularity(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/trend?granularity=minute", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, usageRepo.trendCalled)
}

func TestGetMyAPIKeyUsageTrendRejectsExcessiveHourRange(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/trend?granularity=hour&start_date=2026-01-01&end_date=2026-02-01", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, usageRepo.trendCalled)
}

func TestGetMyAPIKeyUsageTrendUsesTimezoneAwareDedicatedRepoPath(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		trend: []usagestats.TrendDataPoint{
			{
				Date:                "2026-06-14 23:00",
				Requests:            2,
				InputTokens:         11,
				OutputTokens:        13,
				CacheCreationTokens: 3,
				CacheReadTokens:     5,
				TotalTokens:         32,
				Cost:                0.21,
				ActualCost:          0.18,
			},
		},
	}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/trend?granularity=hour&start_date=2026-06-14&end_date=2026-06-14&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.trendCalled)
	require.Equal(t, int64(42), usageRepo.trendUserID)
	require.Equal(t, int64(7), usageRepo.trendAPIKeyID)
	require.Equal(t, "hour", usageRepo.trendGranularity)
	require.Equal(t, "Asia/Shanghai", usageRepo.trendTimezone)
	require.Equal(t, "2026-06-14", usageRepo.trendStartTime.In(time.FixedZone("CST", 8*60*60)).Format(apiKeyUsageTrendDateLayout))
	require.Equal(t, "2026-06-15", usageRepo.trendEndTime.In(time.FixedZone("CST", 8*60*60)).Format(apiKeyUsageTrendDateLayout))

	var got apiKeyUsageTrendHandlerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "hour", got.Data.Granularity)
	require.Equal(t, "2026-06-14", got.Data.StartDate)
	require.Equal(t, "2026-06-14", got.Data.EndDate)
	require.Equal(t, "Asia/Shanghai", got.Data.Timezone)
	require.Len(t, got.Data.Items, 1)
	require.Equal(t, usageRepo.trend[0], got.Data.Items[0])
}

func TestGetMyAPIKeyModelStatsRejectsCrossUserAccess(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 99, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/models", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, usageRepo.modelCalled)
}

func TestGetMyAPIKeyModelStatsReturnsUserSafeFields(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		modelStats: []usagestats.ModelStat{
			{
				Model:               "gpt-5.1",
				Requests:            2,
				InputTokens:         1000,
				OutputTokens:        2000,
				CacheCreationTokens: 300,
				CacheReadTokens:     400,
				TotalTokens:         3700,
				Cost:                9.99,
				ActualCost:          1.23,
				AccountCost:         0.44,
			},
		},
	}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42, Status: service.StatusAPIKeyActive},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/user/api-keys/7/usage/models?start_date=2026-06-01&end_date=2026-06-14&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.modelCalled)
	require.Equal(t, int64(42), usageRepo.modelUserID)
	require.Equal(t, int64(7), usageRepo.modelAPIKeyID)

	var got struct {
		Code int `json:"code"`
		Data struct {
			Models    []map[string]any `json:"models"`
			StartDate string           `json:"start_date"`
			EndDate   string           `json:"end_date"`
			Timezone  string           `json:"timezone"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "2026-06-01", got.Data.StartDate)
	require.Equal(t, "2026-06-14", got.Data.EndDate)
	require.Equal(t, "Asia/Shanghai", got.Data.Timezone)
	require.Len(t, got.Data.Models, 1)
	require.Equal(t, "gpt-5.1", got.Data.Models[0]["model"])
	require.Equal(t, float64(1.23), got.Data.Models[0]["actual_cost"])
	require.NotContains(t, got.Data.Models[0], "cost")
	require.NotContains(t, got.Data.Models[0], "account_cost")
}

func TestDashboardModelsReturnsUserSafeFields(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		modelStats: []usagestats.ModelStat{
			{
				Model:       "claude-sonnet-4.5",
				Requests:    3,
				TotalTokens: 1200,
				Cost:        8.88,
				ActualCost:  0.72,
				AccountCost: 0.31,
			},
		},
	}
	apiKeyRepo := &dailyUsageAPIKeyRepoStub{}
	router := newDailyUsageTestRouter(usageRepo, apiKeyRepo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?start_date=2026-06-01&end_date=2026-06-14", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, usageRepo.dashboardModelCalled)
	require.Equal(t, int64(42), usageRepo.dashboardModelFilters.UserID)
	require.Equal(t, usagestats.ModelSourceRequested, usageRepo.dashboardModelSource)

	var got struct {
		Data struct {
			Models []map[string]any `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data.Models, 1)
	require.Equal(t, "claude-sonnet-4.5", got.Data.Models[0]["model"])
	require.NotContains(t, got.Data.Models[0], "cost")
	require.NotContains(t, got.Data.Models[0], "account_cost")
}

func TestOwnerAPIKeyAnalyticsSummaryBindsAuthenticatedUserAndReturnsSnapshot(t *testing.T) {
	groupID := int64(9)
	usageRepo := &dailyUsageRepoStub{
		ownerSummary: &service.OwnerAPIKeyAnalyticsSummary{
			OwnerAPIKeyUsageTotals: service.OwnerAPIKeyUsageTotals{
				Requests:            12,
				InputTokens:         1000,
				OutputTokens:        2000,
				CacheCreationTokens: 300,
				CacheReadTokens:     400,
				TotalTokens:         3700,
				ActualCost:          1.23,
			},
			UsedKeyCount: 3,
			CurrentKeySnapshot: service.OwnerAPIKeyAnalyticsSnapshot{
				ActiveKeyCount:        5,
				NearQuotaKeyCount:     1,
				NearRateLimitKeyCount: 2,
				SnapshotAt:            time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/summary?granularity=day&start_date=2026-06-01&end_date=2026-06-14&timezone=Asia/Shanghai&api_key_id=77&group_id=9&tags=Team-A,frontend,team-a&status=active&search=alice&limit=7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), usageRepo.ownerFilters.UserID)
	require.NotNil(t, usageRepo.ownerFilters.APIKeyID)
	require.Equal(t, int64(77), *usageRepo.ownerFilters.APIKeyID)
	require.NotNil(t, usageRepo.ownerFilters.GroupID)
	require.Equal(t, groupID, *usageRepo.ownerFilters.GroupID)
	require.Equal(t, []string{"team-a", "frontend"}, usageRepo.ownerFilters.Tags)
	require.Equal(t, service.StatusAPIKeyActive, usageRepo.ownerFilters.Status)
	require.Equal(t, "alice", usageRepo.ownerFilters.Search)
	require.Equal(t, 7, usageRepo.ownerFilters.Limit)
	require.Equal(t, "Asia/Shanghai", usageRepo.ownerFilters.TimezoneName)
	require.Equal(t, "day", usageRepo.ownerFilters.Granularity)
	require.Equal(t, "2026-06-01", usageRepo.ownerFilters.StartTime.In(time.FixedZone("CST", 8*60*60)).Format(apiKeyUsageTrendDateLayout))
	require.Equal(t, "2026-06-15", usageRepo.ownerFilters.EndTime.In(time.FixedZone("CST", 8*60*60)).Format(apiKeyUsageTrendDateLayout))

	var got struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "2026-06-01", got.Data["start_date"])
	require.Equal(t, "2026-06-14", got.Data["end_date"])
	require.Equal(t, "Asia/Shanghai", got.Data["timezone"])
	require.Contains(t, rec.Body.String(), "current_key_snapshot")
	require.NotContains(t, rec.Body.String(), "account_cost")
	require.NotContains(t, rec.Body.String(), "upstream_model")
	require.NotContains(t, rec.Body.String(), "upstream_account")
}

func TestOwnerAPIKeyAnalyticsLeaderboardReturnsDisplayedActualCost(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		ownerLeaderboard: &service.OwnerAPIKeyLeaderboardResponse{
			Items: []service.OwnerAPIKeyLeaderboardItem{
				{
					APIKeyID: 1,
					KeyName:  "Alice",
					OwnerAPIKeyUsageTotals: service.OwnerAPIKeyUsageTotals{
						Requests:   10,
						ActualCost: 3.5,
					},
					SharePercent: 35,
				},
			},
			Total:               3,
			TotalActualCost:     10,
			DisplayedActualCost: 3.5,
		},
	}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/leaderboard?granularity=day&start_date=2026-06-01&end_date=2026-06-14", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, float64(3), got.Data["total"])
	require.Equal(t, 10.0, got.Data["total_actual_cost"])
	require.Equal(t, 3.5, got.Data["displayed_actual_cost"])
	require.Contains(t, rec.Body.String(), "\"share_percent\":35")
	require.NotContains(t, rec.Body.String(), "account_cost")
	require.NotContains(t, rec.Body.String(), "upstream_model")
}

func TestOwnerAPIKeyAnalyticsRejectsInvalidGranularity(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/trend?granularity=minute", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, usageRepo.ownerFilters.UserID)
}

func TestOwnerAPIKeyAnalyticsRejectsInvalidAPIKeyID(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/summary?api_key_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, usageRepo.ownerFilters.UserID)
}

func TestOwnerAPIKeyAnalyticsRejectsExcessiveDayRange(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/leaderboard?granularity=day&start_date=2025-01-01&end_date=2026-01-01", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, usageRepo.ownerFilters.UserID)
}

func TestOwnerAPIKeyModelAnalyticsReturnsUserSafeFields(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		ownerModels: []service.OwnerModelAnalyticsItem{
			{
				Model: "gpt-5.1",
				OwnerAPIKeyUsageTotals: service.OwnerAPIKeyUsageTotals{
					Requests:    8,
					TotalTokens: 9000,
					ActualCost:  2.34,
				},
			},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/models?start_date=2026-06-01&end_date=2026-06-14&timezone=Asia/Shanghai&status=inactive", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), usageRepo.ownerFilters.UserID)
	require.Equal(t, "Asia/Shanghai", usageRepo.ownerFilters.TimezoneName)
	require.Equal(t, service.StatusAPIKeyDisabled, usageRepo.ownerFilters.Status)

	var got struct {
		Data struct {
			Models []map[string]any `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data.Models, 1)
	require.Equal(t, "gpt-5.1", got.Data.Models[0]["model"])
	require.Equal(t, float64(2.34), got.Data.Models[0]["actual_cost"])
	require.NotContains(t, got.Data.Models[0], "cost")
	require.NotContains(t, got.Data.Models[0], "account_cost")
	require.NotContains(t, got.Data.Models[0], "upstream_model")
}

func TestOwnerAPIKeyTagAnalyticsDoesNotExposeSharePercent(t *testing.T) {
	usageRepo := &dailyUsageRepoStub{
		ownerTags: []service.OwnerTagAnalyticsItem{
			{
				Tag:      "frontend",
				KeyCount: 2,
				OwnerAPIKeyUsageTotals: service.OwnerAPIKeyUsageTotals{
					Requests:   5,
					ActualCost: 0.78,
				},
			},
		},
	}
	router := newDailyUsageTestRouter(usageRepo, &dailyUsageAPIKeyRepoStub{}, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/analytics/tags?granularity=week&start_date=2026-06-01&end_date=2026-06-14", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), usageRepo.ownerFilters.UserID)
	require.Contains(t, rec.Body.String(), "frontend")
	require.NotContains(t, rec.Body.String(), "share_percent")
	require.NotContains(t, rec.Body.String(), "account_cost")
	require.NotContains(t, rec.Body.String(), "upstream_model")
}
