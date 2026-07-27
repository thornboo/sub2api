package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
