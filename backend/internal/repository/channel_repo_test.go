//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// --- marshalModelMapping ---

func TestMarshalModelMapping(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]map[string]string
		wantJSON string // expected JSON output (exact match)
	}{
		{
			name:     "empty map",
			input:    map[string]map[string]string{},
			wantJSON: "{}",
		},
		{
			name:     "nil map",
			input:    nil,
			wantJSON: "{}",
		},
		{
			name: "populated map",
			input: map[string]map[string]string{
				"openai": {"gpt-4": "gpt-4-turbo"},
			},
		},
		{
			name: "nested values",
			input: map[string]map[string]string{
				"openai":    {"*": "gpt-5.4"},
				"anthropic": {"claude-old": "claude-new"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalModelMapping(tt.input)
			require.NoError(t, err)

			if tt.wantJSON != "" {
				require.Equal(t, []byte(tt.wantJSON), result)
			} else {
				// round-trip: unmarshal and compare with input
				var parsed map[string]map[string]string
				require.NoError(t, json.Unmarshal(result, &parsed))
				require.Equal(t, tt.input, parsed)
			}
		})
	}
}

// --- unmarshalModelMapping ---

func TestUnmarshalModelMapping(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantNil bool
		want    map[string]map[string]string
	}{
		{
			name:    "nil data",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "empty data",
			input:   []byte{},
			wantNil: true,
		},
		{
			name:    "invalid JSON",
			input:   []byte("not-json"),
			wantNil: true,
		},
		{
			name:    "type error - number",
			input:   []byte("42"),
			wantNil: true,
		},
		{
			name:    "type error - array",
			input:   []byte("[1,2,3]"),
			wantNil: true,
		},
		{
			name:  "valid JSON",
			input: []byte(`{"openai":{"gpt-4":"gpt-4-turbo"},"anthropic":{"old":"new"}}`),
			want: map[string]map[string]string{
				"openai":    {"gpt-4": "gpt-4-turbo"},
				"anthropic": {"old": "new"},
			},
		},
		{
			name:  "empty object",
			input: []byte("{}"),
			want:  map[string]map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unmarshalModelMapping(tt.input)
			if tt.wantNil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tt.want, result)
			}
		})
	}
}

func TestModelMappingOrderRoundTrip(t *testing.T) {
	input := map[string][]string{
		"openai": {"gpt-5.2", "gpt-5.10"},
	}

	encoded, err := marshalModelMappingOrder(input)
	require.NoError(t, err)
	require.Equal(t, input, unmarshalModelMappingOrder(encoded))

	empty, err := marshalModelMappingOrder(nil)
	require.NoError(t, err)
	require.Equal(t, []byte("{}"), empty)
	require.Nil(t, unmarshalModelMappingOrder([]byte("invalid")))
}

// --- escapeLike ---

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special chars",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "backslash",
			input: `a\b`,
			want:  `a\\b`,
		},
		{
			name:  "percent",
			input: "50%",
			want:  `50\%`,
		},
		{
			name:  "underscore",
			input: "a_b",
			want:  `a\_b`,
		},
		{
			name:  "all special chars",
			input: `a\b%c_d`,
			want:  `a\\b\%c\_d`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "consecutive special chars",
			input: "%_%",
			want:  `\%\_\%`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, escapeLike(tt.input))
		})
	}
}

// --- isUniqueViolation ---

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique violation code 23505",
			err:  &pq.Error{Code: "23505"},
			want: true,
		},
		{
			name: "different pq error code",
			err:  &pq.Error{Code: "23503"},
			want: false,
		},
		{
			name: "non-pq error",
			err:  errors.New("some generic error"),
			want: false,
		},
		{
			name: "typed nil pq.Error",
			err: func() error {
				var pqErr *pq.Error
				return pqErr
			}(),
			want: false,
		},
		{
			name: "bare nil",
			err:  nil,
			want: false,
		},
		{
			name: "wrapped pq error with 23505",
			err:  fmt.Errorf("wrapped: %w", &pq.Error{Code: "23505"}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isUniqueViolation(tt.err))
		})
	}
}

func TestChannelListOrderBy_AllowsDescendingIDSort(t *testing.T) {
	params := pagination.PaginationParams{
		SortBy:    "id",
		SortOrder: "desc",
	}

	require.Equal(t, "c.id DESC, c.id DESC", channelListOrderBy(params))
}

func TestListModelPricingRoundTripsTimePricing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Now()
	timePricingJSON := []byte(`{"enabled":true,"timezone":"Asia/Shanghai","default_label":"regular","default_multiplier":0.8,"rules":[{"label":"peak","start_time":"09:00","end_time":"12:00","multiplier":2}]}`)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, channel_id, sort_order, platform, models, billing_mode, input_price, output_price, cache_write_price, cache_read_price, image_input_price, image_output_price, per_request_price, time_pricing, created_at, updated_at
		 FROM channel_model_pricing WHERE channel_id = $1 ORDER BY platform, sort_order, id`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "channel_id", "sort_order", "platform", "models", "billing_mode",
			"input_price", "output_price", "cache_write_price", "cache_read_price",
			"image_input_price", "image_output_price", "per_request_price", "time_pricing",
			"created_at", "updated_at",
		}).AddRow(int64(11), int64(7), 0, "anthropic", []byte(`["claude-sonnet-4"]`), service.BillingModeToken,
			1e-6, 2e-6, nil, nil, nil, nil, nil, timePricingJSON, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, pricing_id, min_tokens, max_tokens, tier_label,
		        input_price, output_price, cache_write_price, cache_read_price,
		        per_request_price, sort_order, created_at, updated_at
		 FROM channel_pricing_intervals
		 WHERE pricing_id = ANY($1) ORDER BY pricing_id, sort_order, id`)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pricing_id", "min_tokens", "max_tokens", "tier_label",
			"input_price", "output_price", "cache_write_price", "cache_read_price",
			"per_request_price", "sort_order", "created_at", "updated_at",
		}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT channel_id, model, enabled
		 FROM model_self_check_config
		 WHERE channel_id = ANY($1)`)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"channel_id", "model", "enabled"}))

	repo := &channelRepository{db: db}
	got, err := repo.ListModelPricing(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].TimePricing)
	require.Equal(t, "Asia/Shanghai", got[0].TimePricing.Timezone)
	require.Equal(t, "regular", got[0].TimePricing.DefaultLabel)
	require.NotNil(t, got[0].TimePricing.DefaultMultiplier)
	require.Equal(t, 0.8, *got[0].TimePricing.DefaultMultiplier)
	require.Equal(t, 2.0, got[0].TimePricing.Rules[0].Multiplier)
	require.Equal(t, "peak", got[0].TimePricing.Rules[0].Label)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTimePricingJSONPreservesDisabledSchedule(t *testing.T) {
	defaultMultiplier := 0.8
	original := &service.TimePricing{
		Enabled:           false,
		Timezone:          "Asia/Shanghai",
		DefaultLabel:      "平时",
		DefaultMultiplier: &defaultMultiplier,
		Rules: []service.TimePricingRule{{
			Label:      "高峰",
			StartTime:  "09:00",
			EndTime:    "12:00",
			Multiplier: 2,
		}},
	}

	encoded := marshalTimePricing(original)
	require.NotEqual(t, `{}`, string(encoded))
	restored := unmarshalTimePricing(encoded)
	require.NotNil(t, restored)
	require.False(t, restored.Enabled)
	require.Equal(t, "Asia/Shanghai", restored.Timezone)
	require.Equal(t, "平时", restored.DefaultLabel)
	require.NotNil(t, restored.DefaultMultiplier)
	require.Equal(t, 0.8, *restored.DefaultMultiplier)
	require.Len(t, restored.Rules, 1)
	require.Equal(t, "高峰", restored.Rules[0].Label)
	require.Equal(t, 2.0, restored.Rules[0].Multiplier)
}
