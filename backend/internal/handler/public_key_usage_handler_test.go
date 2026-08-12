package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestPublicKeyUsageBearerCredential(t *testing.T) {
	tests := []struct {
		value string
		want  string
		ok    bool
	}{
		{value: "Bearer sk-test", want: "sk-test", ok: true},
		{value: "bearer sk-test", want: "sk-test", ok: true},
		{value: "sk-test", ok: false},
		{value: "Bearer", ok: false},
		{value: "Bearer one two", ok: false},
	}
	for _, test := range tests {
		got, ok := parseBearerCredential(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("parseBearerCredential(%q) = %q, %v; want %q, %v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func TestPublicKeyUsageDateRangeIsInclusiveAndBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/?start_date=2026-07-01&end_date=2026-07-30&timezone=Asia%2FShanghai", nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	start, end, startDate, endDate, _, ok := parsePublicKeyUsageDateRange(ctx)
	if !ok {
		t.Fatalf("expected valid range, status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if startDate != "2026-07-01" || endDate != "2026-07-30" {
		t.Fatalf("range labels = %s..%s", startDate, endDate)
	}
	if end.Sub(start).Hours() != 30*24 {
		t.Fatalf("inclusive range duration = %v, want 30 days", end.Sub(start))
	}

	request = httptest.NewRequest(http.MethodGet, "/?start_date=2026-01-01&end_date=2026-04-01", nil)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = request
	_, _, _, _, _, ok = parsePublicKeyUsageDateRange(ctx)
	if ok || recorder.Code != http.StatusBadRequest {
		t.Fatalf("91-day range should fail with 400, got ok=%v status=%d", ok, recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/?timezone=Not%2FA-Timezone", nil)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = request
	_, _, _, _, _, ok = parsePublicKeyUsageDateRange(ctx)
	if ok || recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid timezone should fail with 400, got ok=%v status=%d", ok, recorder.Code)
	}
}

func TestPublicKeyUsageCookieAndNoStoreHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/key/usage-session", nil)
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")

	setPublicKeyUsageNoStore(ctx)
	setPublicKeyUsageCookie(ctx, "opaque-token", 3600)
	result := recorder.Result()
	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != publicKeyUsageSessionPath {
		t.Fatalf("unexpected cookie attributes: %+v", cookie)
	}
	if result.Header.Get("Cache-Control") != "no-store" || result.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("missing no-store headers: %+v", result.Header)
	}
}

func TestPublicKeyUsageCSVFormulaProtectionAndIPMasking(t *testing.T) {
	for _, value := range []string{"=CMD()", "+1", "-1", "@SUM(A1)", "\t=CMD()", "\r=CMD()"} {
		if got := protectCSVFormula(value); !strings.HasPrefix(got, "'") {
			t.Fatalf("formula value %q was not protected: %q", value, got)
		}
	}
	if got := maskPublicKeyUsageIP("203.0.113.42"); got != "203.0.113.*" {
		t.Fatalf("masked IPv4 = %q", got)
	}
	if got := maskPublicKeyUsageIP("invalid"); got != "" {
		t.Fatalf("invalid IP should be removed, got %q", got)
	}
}

func TestPublicKeyUsageExportUsesEffectivePageSize(t *testing.T) {
	if !shouldContinuePublicKeyUsageExport(500, 500, 500) {
		t.Fatal("a full repository-capped error page must continue exporting")
	}
	if shouldContinuePublicKeyUsageExport(499, 499, 500) {
		t.Fatal("a partial page must stop exporting")
	}
	if shouldContinuePublicKeyUsageExport(publicKeyUsageExportLimit, 500, 500) {
		t.Fatal("the export limit must stop pagination")
	}
}

func TestPublicKeyUsageDTOsOmitSecretsAndAccountCost(t *testing.T) {
	upstreamEndpoint := "/v1/internal/upstream"
	age := int64(345)
	log := &service.UsageLog{
		ID: 1, UserID: 2, APIKeyID: 3, AccountID: 4, RequestID: "request-1",
		Model: "gpt-test", ActualCost: 1.25, AccountStatsCost: floatPointer(99), CreatedAt: time.Now(),
		UpstreamEndpoint:       &upstreamEndpoint,
		RoutePlanSource:        "last_known_good",
		RoutePlanSnapshotAgeMs: &age,
		ScheduleMeta: &service.UsageScheduleMeta{
			CandidateCount:    3,
			SelectedAccountID: 4,
			ShadowDiffType:    "legacy_success_new_pruned",
		},
		APIKey:  &service.APIKey{ID: 3, UserID: 2, Key: "raw-key-canary", Name: "key"},
		Account: &service.Account{ID: 4, Name: "upstream-account-canary"},
	}
	models := mapPublicKeyUsageModels([]usagestats.ModelStat{{Model: "gpt-test", ActualCost: 1.25, AccountCost: 99}})
	errorRecord := mapPublicKeyUsageError(&service.UserErrorRequest{
		ID: 9, RequestID: "safe-request-id", Message: "safe error", Model: "gpt-test", CreatedAt: time.Now(),
	})
	csvText := strings.Join(publicKeyUsageCSVRow(mapPublicKeyUsageLog(log)), ",")
	payload, err := json.Marshal(struct {
		Record      publicKeyUsageRecord      `json:"record"`
		ErrorRecord publicKeyUsageRecord      `json:"error_record"`
		Models      []publicKeyUsageModelStat `json:"models"`
		CSV         string                    `json:"csv"`
	}{Record: mapPublicKeyUsageLog(log), ErrorRecord: errorRecord, Models: models, CSV: csvText})
	if err != nil {
		t.Fatalf("marshal public DTO: %v", err)
	}
	text := string(payload) + csvText
	for _, forbidden := range []string{
		"raw-key-canary",
		"upstream-account-canary",
		"account_cost",
		"account_id",
		"api_key_id",
		"user_id",
		"upstream_endpoint",
		"route_plan_source",
		"route_plan_snapshot_age_ms",
		"schedule_meta",
		"candidate_count",
		"selected_account_id",
		"legacy_success_new_pruned",
		"last_known_good",
		"/v1/internal/upstream",
		"error_body",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public DTO contains forbidden value %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"actual_cost":1.25`) {
		t.Fatalf("public DTO lost actual billed amount: %s", text)
	}
	if !strings.Contains(text, `"request_id":"safe-request-id"`) {
		t.Fatalf("public error DTO lost safe request id: %s", text)
	}
}

func TestPublicKeyUsageMemberBudgetUsesSettledSpendOnly(t *testing.T) {
	mapped := mapPublicKeyUsageMemberBudget(&service.EnterpriseMemberBudgetSummary{
		LimitUSD:     100,
		UsedUSD:      39.64,
		ReservedUSD:  53.38,
		RemainingUSD: 60.36,
	})

	if mapped == nil {
		t.Fatal("expected member budget mapping")
		return
	}
	if mapped.Monthly.Used != 39.64 {
		t.Fatalf("monthly used = %v, want settled spend only", mapped.Monthly.Used)
	}
	if mapped.Monthly.Remaining != 60.36 {
		t.Fatalf("monthly remaining = %v, want settled-spend remainder", mapped.Monthly.Remaining)
	}
	if mapped.ReservedUSD != 53.38 {
		t.Fatalf("compatibility reserved_usd = %v, want unchanged diagnostic field", mapped.ReservedUSD)
	}
}

func TestPublicKeyUsageModelsForGroupReturnsEmptyWhenGroupHasNoPersistentlyEnabledAccounts(t *testing.T) {
	groupID := int64(41)
	otherGroupID := int64(42)
	repo := &publicKeyUsageModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			otherGroupID: {
				{
					ID:       7,
					Platform: service.PlatformOpenAI,
					Credentials: map[string]any{
						"model_mapping": map[string]any{"gpt-other": "gpt-other"},
					},
				},
			},
		},
	}
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(repo)}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive,
	})

	if len(models) != 0 {
		t.Fatalf("models for a group without persistently enabled accounts = %v, want empty list", models)
	}
}

func TestPublicKeyUsageModelsForGroupKeepsDefaultModelsWhenEnabledAccountHasNoMapping(t *testing.T) {
	groupID := int64(43)
	repo := &publicKeyUsageModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			groupID: {{ID: 8, Platform: service.PlatformOpenAI}},
		},
	}
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(repo)}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive,
	})

	if len(models) == 0 {
		t.Fatal("models for an enabled default-model account should not be empty")
	}
	if !containsStringFold(models, "gpt-5.6-sol") {
		t.Fatalf("models for an enabled default-model account = %v, want OpenAI defaults", models)
	}
}

func TestPublicKeyUsageModelsForGroupCustomListReturnsEmptyWhenGroupHasNoPersistentlyEnabledAccounts(t *testing.T) {
	groupID := int64(44)
	repo := &publicKeyUsageModelsAccountRepoStub{byGroup: map[int64][]service.Account{}}
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(repo)}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID:       groupID,
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"gpt-5.6-sol"},
		},
	})

	if len(models) != 0 {
		t.Fatalf("custom models for a group without persistently enabled accounts = %v, want empty list", models)
	}
}

func TestPublicKeyUsageModelsForGroupDoesNotFallbackForDifferentPlatformAccount(t *testing.T) {
	groupID := int64(44)
	repo := &publicKeyUsageModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			groupID: {{ID: 9, Platform: service.PlatformGemini}},
		},
	}
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(repo)}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive,
	})

	if len(models) != 0 {
		t.Fatalf("models for an OpenAI group without persistently enabled OpenAI accounts = %v, want empty list", models)
	}
}

func TestPublicKeyUsageModelsForGroupKeepsConfiguredModelsDuringTransientCooldown(t *testing.T) {
	groupID := int64(45)
	cooldownUntil := time.Now().Add(time.Hour)
	repo := &publicKeyUsageModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			groupID: {{
				ID:                     10,
				Platform:               service.PlatformOpenAI,
				RateLimitResetAt:       &cooldownUntil,
				OverloadUntil:          &cooldownUntil,
				TempUnschedulableUntil: &cooldownUntil,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"gpt-configured": "gpt-upstream"},
				},
			}},
		},
	}
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(repo)}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive,
	})

	if !containsStringFold(models, "gpt-configured") {
		t.Fatalf("models for a persistently enabled account in transient cooldown = %v, want configured model", models)
	}
}

func TestPublicKeyUsageModelsForGroupDoesNotAdvertiseDefaultsOnRepositoryError(t *testing.T) {
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(&publicKeyUsageModelsAccountRepoStub{
		err: errors.New("database unavailable"),
	})}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: 46, Platform: service.PlatformAnthropic, Status: service.StatusActive,
	})

	if len(models) != 0 {
		t.Fatalf("models after repository failure = %v, want empty list rather than unverified defaults", models)
	}
}

func TestPublicKeyUsageModelsForCompositeGroupDoesNotFallbackWhenNoAccountsOrRoutesExist(t *testing.T) {
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(&publicKeyUsageModelsAccountRepoStub{})}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID:       47,
		Platform: service.PlatformComposite,
		Status:   service.StatusActive,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"gpt-5.6-sol"},
		},
	})

	if len(models) != 0 {
		t.Fatalf("models for an empty composite group = %v, want empty list", models)
	}
}

func TestPublicKeyUsageModelsForCompositeGroupKeepsConfiguredPlatformDuringTransientCooldown(t *testing.T) {
	groupID := int64(48)
	cooldownUntil := time.Now().Add(time.Hour)
	handler := &GatewayHandler{gatewayService: newPublicKeyUsageGatewayService(&publicKeyUsageModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			groupID: {{
				ID:                     11,
				Platform:               service.PlatformOpenAI,
				RateLimitResetAt:       &cooldownUntil,
				OverloadUntil:          &cooldownUntil,
				TempUnschedulableUntil: &cooldownUntil,
			}},
		},
	})}

	models := handler.publicKeyUsageModelsForGroup(context.Background(), &service.Group{
		ID: groupID, Platform: service.PlatformComposite, Status: service.StatusActive,
	})

	if !containsStringFold(models, "gpt-5.6-sol") {
		t.Fatalf("models for a composite group with a cooling OpenAI account = %v, want OpenAI defaults", models)
	}
	if containsStringFold(models, "claude-sonnet-4-6") {
		t.Fatalf("models for a composite group with only OpenAI accounts = %v, must not include Anthropic defaults", models)
	}
}

type publicKeyUsageModelsAccountRepoStub struct {
	service.AccountRepository

	byGroup map[int64][]service.Account
	err     error
}

func (s *publicKeyUsageModelsAccountRepoStub) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]service.Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	accounts := append([]service.Account(nil), s.byGroup[groupID]...)
	return accounts, nil
}

func (s *publicKeyUsageModelsAccountRepoStub) ListModelAvailabilityCandidates(
	_ context.Context,
	groupID *int64,
	platforms []string,
	_ bool,
) ([]service.Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	if groupID == nil {
		return nil, nil
	}
	wantedPlatforms := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		wantedPlatforms[platform] = struct{}{}
	}
	accounts := make([]service.Account, 0)
	for _, account := range s.byGroup[*groupID] {
		if _, ok := wantedPlatforms[account.Platform]; ok {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func newPublicKeyUsageGatewayService(repo service.AccountRepository) *service.GatewayService {
	return service.NewGatewayService(
		repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func floatPointer(value float64) *float64 { return &value }
