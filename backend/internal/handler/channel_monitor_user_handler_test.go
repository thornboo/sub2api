package handler

import (
	"encoding/json"
	"strings"
	"testing"
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
