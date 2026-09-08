package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestUserModelStatusDTOOmitsUpstreamFields(t *testing.T) {
	latency := 321
	availability := 99.5
	item := userModelStatusListItem{
		GroupID:         10,
		GroupName:       "Pro",
		Model:           "gpt-4o",
		DisplayName:     "gpt-4o",
		Status:          "operational",
		ReasonCode:      "ok",
		MessageCode:     "normal",
		LatestLatencyMs: &latency,
		Availability24h: &availability,
	}

	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	body := string(payload)
	for _, forbidden := range []string{
		"account_id",
		"channel_id",
		"provider",
		"platform",
		"upstream",
		"endpoint",
		"raw_error",
		"error_code",
		"cost",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("user model status payload leaked forbidden field %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"reason_code":"ok"`) {
		t.Fatalf("user model status payload missing safe reason_code: %s", body)
	}
}

func TestUserModelStatusHandlersRequireAuthenticatedSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ChannelMonitorUserHandler{}
	tests := []struct {
		name string
		path string
		call func(*gin.Context)
	}{
		{
			name: "list",
			path: "/api/v1/model-status",
			call: handler.ListModelStatus,
		},
		{
			name: "detail",
			path: "/api/v1/model-status/detail?group_id=20&model=private-model",
			call: handler.GetModelStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)

			tt.call(ctx)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
		})
	}
}

func TestUserModelStatusListProjectsPausedSelfCheckWithoutDroppingHistory(t *testing.T) {
	handler := newPausedModelStatusTestHandler(false)
	recorder, ctx := newAuthenticatedModelStatusContext(http.MethodGet, "/api/v1/model-status")

	handler.ListModelStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Items []struct {
				Status          string `json:"status"`
				ReasonCode      string `json:"reason_code"`
				MessageCode     string `json:"message_code"`
				LatestLatencyMs *int   `json:"latest_latency_ms"`
				LastCheckedAt   string `json:"last_checked_at"`
				Timeline        []struct {
					Status     string `json:"status"`
					ReasonCode string `json:"reason_code"`
				} `json:"timeline"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data.Items) != 1 {
		t.Fatalf("items = %#v, want one", payload.Data.Items)
	}
	item := payload.Data.Items[0]
	if item.Status != service.UserModelStatusUnknown || item.ReasonCode != userModelStatusReasonProbePaused || item.MessageCode != "no_data" {
		t.Fatalf("paused status/reason/message = %q/%q/%q", item.Status, item.ReasonCode, item.MessageCode)
	}
	if item.LatestLatencyMs != nil {
		t.Fatalf("latest latency = %v, want nil while paused", item.LatestLatencyMs)
	}
	if item.LastCheckedAt == "" || len(item.Timeline) != 1 || item.Timeline[0].ReasonCode != "ok" {
		t.Fatalf("history not preserved while paused: %#v", item)
	}
}

func TestUserModelStatusDetailUsesLiveSelfCheckStatusAfterReenabled(t *testing.T) {
	handler := newPausedModelStatusTestHandler(true)
	recorder, ctx := newAuthenticatedModelStatusContext(http.MethodGet, "/api/v1/model-status/detail?group_id=10&model=gpt-4o")

	handler.GetModelStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Status          string `json:"status"`
			ReasonCode      string `json:"reason_code"`
			MessageCode     string `json:"message_code"`
			LatestLatencyMs *int   `json:"latest_latency_ms"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Status != "operational" || payload.Data.ReasonCode != "ok" || payload.Data.MessageCode != "normal" {
		t.Fatalf("live status/reason/message = %q/%q/%q", payload.Data.Status, payload.Data.ReasonCode, payload.Data.MessageCode)
	}
	if payload.Data.LatestLatencyMs == nil || *payload.Data.LatestLatencyMs != 321 {
		t.Fatalf("latest latency = %v, want 321 after re-enabled", payload.Data.LatestLatencyMs)
	}
}

func newAuthenticatedModelStatusContext(method, path string) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, nil)
	ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	return recorder, ctx
}

func newPausedModelStatusTestHandler(selfCheckEnabled bool) *ChannelMonitorUserHandler {
	checkedAt := time.Now().UTC().Add(-time.Minute)
	latency := 321
	repo := &userModelStatusHandlerRepoStub{
		targets: []service.ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: service.PlatformOpenAI,
			Model:         "gpt-4o",
		}},
		accounts: []service.ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: service.PlatformOpenAI}},
		latest: []service.ModelSelfCheckHistory{{
			Model: "gpt-4o", AccountID: 1, Platform: service.PlatformOpenAI, Status: "operational",
			LatencyMs: &latency, CheckedAt: checkedAt,
		}},
		snapshots: []service.ModelSelfCheckStatusSnapshot{{
			GroupID: 10, Model: "gpt-4o", Status: "operational", ReasonCode: "ok",
			LatencyMs: &latency, CheckedAt: checkedAt,
		}},
	}
	modelStatus := service.NewModelSelfCheckService(repo)
	modelStatus.SetUserGroupProvider(userModelStatusGroupProviderStub{})
	modelStatus.SetProbeDependencies(userModelStatusAccountRepoStub{}, nil)
	settings := service.NewSettingService(&userModelStatusSettingRepoStub{values: map[string]string{
		service.SettingKeyChannelMonitorEnabled:        "true",
		service.SettingKeyChannelMonitorMode:           service.ChannelMonitorModeV1,
		service.SettingKeyModelSelfCheckEnabled:        strconv.FormatBool(selfCheckEnabled),
		service.SettingKeyModelSelfCheckMaxConcurrency: "1",
	}}, &config.Config{})
	return NewChannelMonitorUserHandler(modelStatus, settings)
}

type userModelStatusSettingRepoStub struct {
	values map[string]string
}

func (s *userModelStatusSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *userModelStatusSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *userModelStatusSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *userModelStatusSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *userModelStatusSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *userModelStatusSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *userModelStatusSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type userModelStatusGroupProviderStub struct{}

func (userModelStatusGroupProviderStub) GetAvailableGroups(context.Context, int64) ([]service.Group, error) {
	return []service.Group{{ID: 10}}, nil
}

type userModelStatusAccountRepoStub struct{}

func (userModelStatusAccountRepoStub) GetByID(context.Context, int64) (*service.Account, error) {
	return nil, service.ErrAccountNotFound
}

func (userModelStatusAccountRepoStub) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return []*service.Account{{
		ID:          1,
		Name:        "account",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		GroupIDs:    []int64{10},
	}}, nil
}

type userModelStatusHandlerRepoStub struct {
	targets   []service.ModelSelfCheckTarget
	accounts  []service.ModelSelfCheckTargetAccount
	latest    []service.ModelSelfCheckHistory
	snapshots []service.ModelSelfCheckStatusSnapshot
}

func (s *userModelStatusHandlerRepoStub) ListStatusTargets(context.Context) ([]service.ModelSelfCheckTarget, error) {
	return append([]service.ModelSelfCheckTarget(nil), s.targets...), nil
}

func (s *userModelStatusHandlerRepoStub) ListTargetAccounts(context.Context, []int64) ([]service.ModelSelfCheckTargetAccount, error) {
	return append([]service.ModelSelfCheckTargetAccount(nil), s.accounts...), nil
}

func (s *userModelStatusHandlerRepoStub) ListLatestByModels(context.Context, []string) ([]service.ModelSelfCheckHistory, error) {
	return append([]service.ModelSelfCheckHistory(nil), s.latest...), nil
}

func (s *userModelStatusHandlerRepoStub) ListHistoriesSince(context.Context, []string, time.Time) ([]service.ModelSelfCheckHistory, error) {
	return nil, nil
}

func (s *userModelStatusHandlerRepoStub) ListRecentHistories(context.Context, string, []int64, int) ([]service.ModelSelfCheckHistory, error) {
	return nil, nil
}

func (s *userModelStatusHandlerRepoStub) ListRecentHistoriesBefore(context.Context, string, []int64, time.Time, int) ([]service.ModelSelfCheckHistory, error) {
	return nil, nil
}

func (s *userModelStatusHandlerRepoStub) ListRecentStatusSnapshots(context.Context, int64, string, int) ([]service.ModelSelfCheckStatusSnapshot, error) {
	return append([]service.ModelSelfCheckStatusSnapshot(nil), s.snapshots...), nil
}

func (s *userModelStatusHandlerRepoStub) ListStatusSnapshotsSince(context.Context, int64, string, time.Time) ([]service.ModelSelfCheckStatusSnapshot, error) {
	return append([]service.ModelSelfCheckStatusSnapshot(nil), s.snapshots...), nil
}

func (s *userModelStatusHandlerRepoStub) ListStatusSnapshotMetrics(context.Context, []service.ModelSelfCheckTarget, time.Time) ([]service.ModelSelfCheckStatusMetrics, error) {
	return []service.ModelSelfCheckStatusMetrics{{GroupID: 10, Model: "gpt-4o", HasSnapshots: true}}, nil
}

func (s *userModelStatusHandlerRepoStub) ListTokenUsageSince(context.Context, time.Time) ([]service.ModelSelfCheckTokenUsage, error) {
	return nil, nil
}

func (s *userModelStatusHandlerRepoStub) CreateHistory(context.Context, *service.ModelSelfCheckHistory) error {
	return nil
}

func (s *userModelStatusHandlerRepoStub) CreateStatusSnapshot(context.Context, *service.ModelSelfCheckStatusSnapshot) error {
	return nil
}

func (s *userModelStatusHandlerRepoStub) DeleteStatusSnapshotsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
