package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyOpsCurrentFailureState(t *testing.T) {
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	threshold := 5.0
	thresholds := &OpsMetricThresholds{RequestErrorRatePercentMax: &threshold}

	tests := []struct {
		name                     string
		current                  *OpsCurrentFailureWindow
		selectedPlatformFailures int64
		selectedStart            time.Time
		selectedEnd              time.Time
		want                     string
	}{
		{
			name: "active only when the fixed window exceeds the configured threshold",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
				SuccessCount: 94, CustomerVisibleFailureCount: 6, PlatformSLAFailureCount: 6,
			},
			selectedStart: now.Add(-time.Hour), selectedEnd: now,
			want: "active",
		},
		{
			name: "a small number of platform failures below threshold is not active",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
				SuccessCount: 999, CustomerVisibleFailureCount: 1, PlatformSLAFailureCount: 1,
			},
			selectedPlatformFailures: 8,
			selectedStart:            now.Add(-time.Hour),
			selectedEnd:              now,
			want:                     "recovered",
		},
		{
			name: "unclassified evidence makes the state unknown",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
				SuccessCount: 100, CustomerVisibleFailureCount: 1, ClassificationUnknownCount: 1,
			},
			selectedStart: now.Add(-time.Hour), selectedEnd: now,
			want: "unknown",
		},
		{
			name: "confirmed active failures take precedence over unknown evidence",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
				SuccessCount: 90, CustomerVisibleFailureCount: 11,
				PlatformSLAFailureCount: 10, ClassificationUnknownCount: 1,
			},
			selectedStart: now.Add(-time.Hour), selectedEnd: now,
			want: "active",
		},
		{
			name: "a historical selection never claims the current window recovered",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
				SuccessCount: 100,
			},
			selectedPlatformFailures: 8,
			selectedStart:            now.Add(-48 * time.Hour),
			selectedEnd:              now.Add(-24 * time.Hour),
			want:                     "quiet",
		},
		{
			name: "an empty current window is unknown",
			current: &OpsCurrentFailureWindow{
				StartTime: now.Add(-15 * time.Minute), EndTime: now,
			},
			selectedStart: now.Add(-time.Hour), selectedEnd: now,
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOpsCurrentFailureState(
				tt.current,
				tt.selectedPlatformFailures,
				tt.selectedStart,
				tt.selectedEnd,
				thresholds,
			)
			require.Equal(t, tt.want, got)
		})
	}
}
