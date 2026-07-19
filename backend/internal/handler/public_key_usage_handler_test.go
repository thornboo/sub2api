package handler

import (
	"encoding/json"
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
	log := &service.UsageLog{
		ID: 1, UserID: 2, APIKeyID: 3, AccountID: 4, RequestID: "request-1",
		Model: "gpt-test", ActualCost: 1.25, AccountStatsCost: floatPointer(99), CreatedAt: time.Now(),
		APIKey:  &service.APIKey{ID: 3, UserID: 2, Key: "raw-key-canary", Name: "key"},
		Account: &service.Account{ID: 4, Name: "upstream-account-canary"},
	}
	models := mapPublicKeyUsageModels([]usagestats.ModelStat{{Model: "gpt-test", ActualCost: 1.25, AccountCost: 99}})
	errorRecord := mapPublicKeyUsageError(&service.UserErrorRequest{
		ID: 9, RequestID: "safe-request-id", Message: "safe error", Model: "gpt-test", CreatedAt: time.Now(),
	})
	payload, err := json.Marshal(struct {
		Record      publicKeyUsageRecord      `json:"record"`
		ErrorRecord publicKeyUsageRecord      `json:"error_record"`
		Models      []publicKeyUsageModelStat `json:"models"`
	}{Record: mapPublicKeyUsageLog(log), ErrorRecord: errorRecord, Models: models})
	if err != nil {
		t.Fatalf("marshal public DTO: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"raw-key-canary", "upstream-account-canary", "account_cost", "account_id", "api_key_id", "user_id", "upstream_endpoint", "error_body"} {
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

func floatPointer(value float64) *float64 { return &value }
