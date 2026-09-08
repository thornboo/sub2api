//go:build integration

package repository

import (
	"context"
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelRuntimeAggregationSQLEmptyTempTablesReturnNoBuckets(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	createModelRuntimeTempTables(t, ctx, tx)

	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	rows, err := tx.QueryContext(ctx, modelRuntimeAggregationSQL, start, start.Add(24*time.Hour))
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func TestModelRuntimeAggregationSQLRequestSemantics(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	createModelRuntimeTempTables(t, ctx, tx)

	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	model := "requested-model"
	groupID := int64(11)

	usage := func(id int64, apiKeyID int64, requestID string, at time.Time, mutate func(*modelRuntimeUsageFixture)) {
		t.Helper()
		row := modelRuntimeUsageFixture{
			ID:           id,
			APIKeyID:     apiKeyID,
			RequestID:    requestID,
			GroupID:      groupID,
			CreatedAt:    at,
			Requested:    model,
			Model:        "upstream-model",
			ActualCost:   1,
			InputTokens:  1,
			OutputTokens: 20,
			RequestType:  2,
			Stream:       true,
			FirstTokenMS: 100,
			DurationMS:   2100,
		}
		if mutate != nil {
			mutate(&row)
		}
		insertModelRuntimeUsage(t, ctx, tx, row)
	}
	terminalError := func(id int64, apiKeyID int64, requestID string, clientRequestID string, at time.Time, slaImpact bool) {
		t.Helper()
		insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
			ID:              id,
			APIKeyID:        apiKeyID,
			RequestID:       requestID,
			ClientRequestID: clientRequestID,
			GroupID:         groupID,
			CreatedAt:       at,
			Requested:       model,
			Model:           "upstream-model",
			StatusCode:      500,
			CustomerVisible: true,
			SLAImpact:       slaImpact,
		})
	}

	usage(1, 101, "client:dedupe", start.Add(2*time.Hour), nil)
	usage(2, 101, "client:dedupe", start.Add(2*time.Hour+10*time.Minute), nil)
	terminalError(1, 101, "", "same-client", start.Add(3*time.Hour), true)
	usage(3, 101, "client:same-client", start.Add(3*time.Hour+time.Minute), nil)
	usage(4, 202, "client:same-client", start.Add(3*time.Hour+2*time.Minute), nil)
	terminalError(2, 101, "partial-local", "", start.Add(4*time.Hour), true)
	usage(5, 101, "local:partial-local", start.Add(4*time.Hour+time.Minute), nil)
	insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
		ID: 3, APIKeyID: 101, RequestID: "recovered", GroupID: groupID, CreatedAt: start.Add(5 * time.Hour),
		Requested: model, Model: "upstream-model", StatusCode: 500, CustomerVisible: true, SLAImpact: true,
		EventScope: "upstream_attempt_recovered",
	})
	usage(6, 101, "local:recovered", start.Add(5*time.Hour+time.Minute), nil)
	terminalError(4, 101, "", "client-cancel", start.Add(6*time.Hour), false)
	usage(7, 101, "client:client-cancel", start.Add(6*time.Hour+time.Minute), nil)
	usage(8, 101, "client:free", start.Add(7*time.Hour), func(row *modelRuntimeUsageFixture) {
		row.ActualCost = 0
		row.OutputTokens = 0
	})
	usage(9, 101, "client:placeholder", start.Add(8*time.Hour), func(row *modelRuntimeUsageFixture) {
		row.ActualCost = 0
		row.InputTokens = 0
		row.OutputTokens = 0
	})
	usage(10, 101, "web_search:bill", start.Add(9*time.Hour), nil)
	usage(11, 101, "client:start-boundary", start, nil)
	usage(12, 101, "client:end-boundary", end, nil)
	usage(13, 101, "client:nonstream", start.Add(10*time.Hour), func(row *modelRuntimeUsageFixture) {
		row.Stream = false
		row.RequestType = 1
		row.OutputTokens = 100
	})
	usage(14, 101, "client:invalid-ttft", start.Add(11*time.Hour), func(row *modelRuntimeUsageFixture) {
		row.FirstTokenMS = 0
		row.DurationMS = 1000
		row.OutputTokens = 100
	})
	usage(15, 101, "client:valid-throughput", start.Add(12*time.Hour), func(row *modelRuntimeUsageFixture) {
		row.FirstTokenMS = 500
		row.DurationMS = 1500
		row.OutputTokens = 50
	})

	buckets := queryModelRuntimeBuckets(t, ctx, tx, start, end)
	modelBuckets := buckets[service.ModelRuntimeKey{GroupID: groupID, Model: model}]
	require.NotEmpty(t, modelBuckets)

	total := sumModelRuntimeBuckets(modelBuckets)
	require.Equal(t, int64(8), total.Successes)
	require.Equal(t, int64(2), total.Failures)
	require.Equal(t, float64(1000), total.FirstTokenSumMS)
	require.Equal(t, int64(6), total.FirstTokenCount)
	require.Equal(t, float64(15100), total.DurationSumMS)
	require.Equal(t, int64(8), total.DurationCount)
	require.Equal(t, float64(130), total.OutputTokens)
	require.Equal(t, float64(9000), total.GenerationTimeMS)

	require.Equal(t, int64(1), modelBuckets[0].Successes, "start boundary must be inclusive")
	require.NotContains(t, modelBuckets, 24, "end boundary must be exclusive")

	for _, key := range sortedModelRuntimeKeys(buckets) {
		require.Equal(t, model, key.Model, "requested_model must win over raw model")
	}
}

func TestModelRuntimeAggregationSQLPreservesFailuresWithReusedRequestHeader(t *testing.T) {
	for _, partialUsage := range []bool{false, true} {
		name := "terminal_failures"
		if partialUsage {
			name = "terminal_failures_with_partial_usage"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			tx := testTx(t)
			createModelRuntimeTempTables(t, ctx, tx)
			start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
			key := service.ModelRuntimeKey{GroupID: 11, Model: "requested-model"}
			for i, clientID := range []string{"generated-first-request", "generated-second-request"} {
				insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
					ID: int64(i + 1), APIKeyID: 101, RequestID: "reused-x-request-id", ClientRequestID: clientID,
					GroupID: key.GroupID, CreatedAt: start.Add(time.Duration(i+1) * time.Hour),
					Requested: key.Model, Model: "upstream-model", StatusCode: 502,
					CustomerVisible: true, SLAImpact: true, EventScope: service.OpsEventScopeRequestTerminal,
				})
				if partialUsage {
					insertModelRuntimeUsage(t, ctx, tx, modelRuntimeUsageFixture{
						ID: int64(i + 1), APIKeyID: 101, RequestID: "client:" + clientID, GroupID: key.GroupID,
						CreatedAt: start.Add(time.Duration(i+1)*time.Hour + time.Minute),
						Requested: key.Model, ActualCost: 1, InputTokens: 1, OutputTokens: 10,
					})
				}
			}
			// A subsequent successful HTTP call gets a new generated client ID,
			// even when the caller repeats the same external X-Request-ID header.
			insertModelRuntimeUsage(t, ctx, tx, modelRuntimeUsageFixture{
				ID: 3, APIKeyID: 101, RequestID: "client:generated-third-request", GroupID: key.GroupID,
				CreatedAt: start.Add(3 * time.Hour), Requested: key.Model, ActualCost: 1, InputTokens: 1, OutputTokens: 10,
			})

			buckets := queryModelRuntimeBuckets(t, ctx, tx, start, start.Add(24*time.Hour))[key]
			total := sumModelRuntimeBuckets(buckets)
			require.Equal(t, int64(2), total.Failures, "independent requests sharing an external ID must not collapse")
			require.Equal(t, int64(1), total.Successes, "partial usage from either failed request must not count as success")
			require.Equal(t, int64(1), buckets[1].Failures)
			require.Equal(t, int64(1), buckets[2].Failures)
			require.Equal(t, int64(1), buckets[3].Successes)
		})
	}
}

func TestModelRuntimeAggregationSQLTerminalIdentityCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name     string
		first    modelRuntimeErrorFixture
		second   modelRuntimeErrorFixture
		failures int64
	}{
		{"duplicate_generated_id", modelRuntimeErrorFixture{RequestID: "first-site", ClientRequestID: "same-generated-id"}, modelRuntimeErrorFixture{RequestID: "second-site", ClientRequestID: "same-generated-id"}, 1},
		{"legacy_site_id", modelRuntimeErrorFixture{RequestID: "same-site"}, modelRuntimeErrorFixture{RequestID: "same-site"}, 1},
		{"no_correlation_ids", modelRuntimeErrorFixture{}, modelRuntimeErrorFixture{}, 2},
		{"client_and_local_namespaces", modelRuntimeErrorFixture{RequestID: "other-site", ClientRequestID: "shared"}, modelRuntimeErrorFixture{RequestID: "shared"}, 2},
		{"local_and_row_namespaces", modelRuntimeErrorFixture{}, modelRuntimeErrorFixture{RequestID: "error:1"}, 2},
		{"api_key_isolation", modelRuntimeErrorFixture{ClientRequestID: "same-generated-id"}, modelRuntimeErrorFixture{APIKeyID: 202, ClientRequestID: "same-generated-id"}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tx := testTx(t)
			createModelRuntimeTempTables(t, ctx, tx)
			start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
			key := service.ModelRuntimeKey{GroupID: 11, Model: "requested-model"}
			for i, row := range []modelRuntimeErrorFixture{tt.first, tt.second} {
				row.ID = int64(i + 1)
				if row.APIKeyID == 0 {
					row.APIKeyID = 101
				}
				row.GroupID, row.Requested = key.GroupID, key.Model
				row.CreatedAt = start.Add(time.Duration(i+1) * time.Hour)
				row.StatusCode, row.CustomerVisible, row.SLAImpact = 502, true, true
				insertModelRuntimeError(t, ctx, tx, row)
			}
			buckets := queryModelRuntimeBuckets(t, ctx, tx, start, start.Add(24*time.Hour))[key]
			require.Equal(t, tt.failures, sumModelRuntimeBuckets(buckets).Failures)
			if tt.failures == 1 {
				require.Zero(t, buckets[1].Failures, "duplicate terminal logs retain the latest observation")
				require.Equal(t, int64(1), buckets[2].Failures)
			}
		})
	}
}

func TestModelRuntimeAggregationSQLExclusionsAndLegacyClassification(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	createModelRuntimeTempTables(t, ctx, tx)

	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	groupID := int64(21)
	model := "claude-fable-5"

	baseUsage := modelRuntimeUsageFixture{
		APIKeyID:     701,
		GroupID:      groupID,
		CreatedAt:    start.Add(time.Hour),
		Requested:    model,
		Model:        "upstream-model",
		ActualCost:   1,
		InputTokens:  1,
		OutputTokens: 10,
		RequestType:  2,
		Stream:       true,
		FirstTokenMS: 100,
		DurationMS:   1100,
	}
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(baseUsage, 1, "local:cyber-usage", func(row *modelRuntimeUsageFixture) {
		row.RequestType = int(service.RequestTypeCyberBlocked)
	}))
	insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
		ID: 1, APIKeyID: 701, RequestID: "count-error", GroupID: groupID, CreatedAt: start.Add(time.Hour),
		Requested: model, Model: "upstream-model", StatusCode: 500, CustomerVisible: true, SLAImpact: true,
		IsCountTokens: true,
	})
	insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
		ID: 2, APIKeyID: 701, RequestID: "legacy-recovered", GroupID: groupID, CreatedAt: start.Add(2 * time.Hour),
		Requested: model, Model: "upstream-model", StatusCode: 200, ClassificationVersion: 1,
		ErrorPhase: "upstream", ErrorMessage: "recovered upstream 503",
	})
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(baseUsage, 2, "local:legacy-cyber", func(row *modelRuntimeUsageFixture) {
		row.CreatedAt = start.Add(3*time.Hour + time.Minute)
	}))
	insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
		ID: 3, APIKeyID: 701, RequestID: "legacy-cyber", GroupID: groupID, CreatedAt: start.Add(3 * time.Hour),
		Requested: model, Model: "upstream-model", StatusCode: 200, ClassificationVersion: 1,
		ErrorType: "cyber_policy",
	})
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(baseUsage, 3, "local:image-free", func(row *modelRuntimeUsageFixture) {
		row.CreatedAt = start.Add(4 * time.Hour)
		row.ActualCost = 0
		row.InputTokens = 0
		row.OutputTokens = 0
		row.ImageCount = 1
	}))
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(baseUsage, 4, "local:video-free", func(row *modelRuntimeUsageFixture) {
		row.CreatedAt = start.Add(5 * time.Hour)
		row.ActualCost = 0
		row.InputTokens = 0
		row.OutputTokens = 0
		row.VideoCount = 1
	}))
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(baseUsage, 5, "upstream-id-raw", func(row *modelRuntimeUsageFixture) {
		row.CreatedAt = start.Add(6*time.Hour + time.Minute)
	}))
	insertModelRuntimeError(t, ctx, tx, modelRuntimeErrorFixture{
		ID: 4, APIKeyID: 701, RequestID: "upstream-id-raw", GroupID: groupID, CreatedAt: start.Add(6 * time.Hour),
		Requested: model, Model: "upstream-model", StatusCode: 500, CustomerVisible: true, SLAImpact: true,
	})

	buckets := queryModelRuntimeBuckets(t, ctx, tx, start, end)
	total := sumModelRuntimeBuckets(buckets[service.ModelRuntimeKey{GroupID: groupID, Model: model}])
	require.Equal(t, int64(3), total.Successes)
	require.Equal(t, int64(1), total.Failures)
	require.Equal(t, float64(300), total.FirstTokenSumMS)
	require.Equal(t, int64(3), total.FirstTokenCount)
}

func TestModelRuntimeAggregationSQLKeepsModelRuntimeDimensionsIsolated(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	createModelRuntimeTempTables(t, ctx, tx)

	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	base := modelRuntimeUsageFixture{
		APIKeyID:     801,
		RequestID:    "local:one",
		GroupID:      31,
		CreatedAt:    start.Add(time.Hour),
		Requested:    "same-model",
		Model:        "upstream-model",
		ActualCost:   1,
		InputTokens:  1,
		OutputTokens: 10,
		RequestType:  2,
		Stream:       true,
		FirstTokenMS: 100,
		DurationMS:   1100,
	}
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(base, 1, "local:g31", nil))
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(base, 2, "local:g32", func(row *modelRuntimeUsageFixture) {
		row.GroupID = 32
	}))
	insertModelRuntimeUsage(t, ctx, tx, withModelRuntimeUsage(base, 3, "local:other-model", func(row *modelRuntimeUsageFixture) {
		row.Requested = "other-model"
	}))

	buckets := queryModelRuntimeBuckets(t, ctx, tx, start, end)
	require.Equal(t, int64(1), sumModelRuntimeBuckets(buckets[service.ModelRuntimeKey{GroupID: 31, Model: "same-model"}]).Successes)
	require.Equal(t, int64(1), sumModelRuntimeBuckets(buckets[service.ModelRuntimeKey{GroupID: 32, Model: "same-model"}]).Successes)
	require.Equal(t, int64(1), sumModelRuntimeBuckets(buckets[service.ModelRuntimeKey{GroupID: 31, Model: "other-model"}]).Successes)
}

func TestModelRuntimeAggregationSQLSkipsThroughputPairWhenFirstTokenExceedsDuration(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	createModelRuntimeTempTables(t, ctx, tx)

	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	key := service.ModelRuntimeKey{GroupID: 41, Model: "claude-fable-5"}
	insertModelRuntimeUsage(t, ctx, tx, modelRuntimeUsageFixture{
		ID: 1, APIKeyID: 901, RequestID: "local:bad-pair", GroupID: key.GroupID,
		CreatedAt: start.Add(time.Hour), Requested: key.Model, Model: "upstream-model",
		ActualCost: 1, InputTokens: 1, OutputTokens: 50, RequestType: 2, Stream: true,
		FirstTokenMS: 1500, DurationMS: 1000,
	})

	total := sumModelRuntimeBuckets(queryModelRuntimeBuckets(t, ctx, tx, start, end)[key])
	require.Equal(t, int64(1), total.Successes)
	require.Equal(t, float64(1500), total.FirstTokenSumMS)
	require.Equal(t, int64(1), total.FirstTokenCount)
	require.Equal(t, float64(1000), total.DurationSumMS)
	require.Equal(t, int64(1), total.DurationCount)
	require.Zero(t, total.OutputTokens)
	require.Zero(t, total.GenerationTimeMS)
}

type modelRuntimeUsageFixture struct {
	ID           int64
	APIKeyID     int64
	RequestID    string
	GroupID      int64
	CreatedAt    time.Time
	Requested    string
	Model        string
	ActualCost   float64
	InputTokens  int64
	OutputTokens int64
	RequestType  int
	Stream       bool
	FirstTokenMS int
	DurationMS   int
	ImageCount   int64
	VideoCount   int64
}

type modelRuntimeErrorFixture struct {
	ID                    int64
	APIKeyID              int64
	RequestID             string
	ClientRequestID       string
	GroupID               int64
	CreatedAt             time.Time
	Requested             string
	Model                 string
	StatusCode            int
	ClassificationVersion int
	CustomerVisible       bool
	SLAImpact             bool
	EventScope            string
	IsCountTokens         bool
	ErrorType             string
	ErrorPhase            string
	ErrorMessage          string
}

func createModelRuntimeTempTables(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE usage_logs (
			id bigint PRIMARY KEY,
			api_key_id bigint NOT NULL,
			request_id text NOT NULL DEFAULT '',
			group_id bigint NOT NULL DEFAULT 0,
			created_at timestamptz NOT NULL,
			requested_model text,
			model text,
			first_token_ms integer,
			duration_ms integer,
			output_tokens bigint NOT NULL DEFAULT 0,
			request_type integer,
			stream boolean NOT NULL DEFAULT false,
			input_tokens bigint NOT NULL DEFAULT 0,
			cache_creation_tokens bigint NOT NULL DEFAULT 0,
			cache_read_tokens bigint NOT NULL DEFAULT 0,
			image_input_tokens bigint NOT NULL DEFAULT 0,
			image_output_tokens bigint NOT NULL DEFAULT 0,
			image_count bigint NOT NULL DEFAULT 0,
			video_count bigint NOT NULL DEFAULT 0,
			actual_cost numeric NOT NULL DEFAULT 0
		) ON COMMIT DROP;
		CREATE TEMP TABLE ops_error_logs (
			id bigint PRIMARY KEY,
			api_key_id bigint,
			group_id bigint NOT NULL DEFAULT 0,
			created_at timestamptz NOT NULL,
			requested_model text,
			model text,
			request_id text NOT NULL DEFAULT '',
			client_request_id text NOT NULL DEFAULT '',
			is_count_tokens boolean NOT NULL DEFAULT false,
			event_scope text NOT NULL DEFAULT '',
			status_code integer,
			classification_version integer NOT NULL DEFAULT 2,
			customer_visible boolean,
			sla_impact boolean,
			error_type text NOT NULL DEFAULT '',
			error_phase text NOT NULL DEFAULT '',
			error_message text NOT NULL DEFAULT '',
			stream boolean NOT NULL DEFAULT false,
			is_business_limited boolean NOT NULL DEFAULT false
		) ON COMMIT DROP;
	`)
	require.NoError(t, err)
}

func withModelRuntimeUsage(base modelRuntimeUsageFixture, id int64, requestID string, mutate func(*modelRuntimeUsageFixture)) modelRuntimeUsageFixture {
	row := base
	row.ID = id
	row.RequestID = requestID
	if mutate != nil {
		mutate(&row)
	}
	return row
}

func insertModelRuntimeUsage(t *testing.T, ctx context.Context, tx *sql.Tx, row modelRuntimeUsageFixture) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO usage_logs (
			id, api_key_id, request_id, group_id, created_at, requested_model, model,
			first_token_ms, duration_ms, output_tokens, request_type, stream,
			input_tokens, image_count, video_count, actual_cost
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, row.ID, row.APIKeyID, row.RequestID, row.GroupID, row.CreatedAt, row.Requested, row.Model,
		row.FirstTokenMS, row.DurationMS, row.OutputTokens, row.RequestType, row.Stream,
		row.InputTokens, row.ImageCount, row.VideoCount, row.ActualCost)
	require.NoError(t, err)
}

func insertModelRuntimeError(t *testing.T, ctx context.Context, tx *sql.Tx, row modelRuntimeErrorFixture) {
	t.Helper()
	classificationVersion := row.ClassificationVersion
	if classificationVersion == 0 {
		classificationVersion = 2
	}
	var customerVisible any = row.CustomerVisible
	var slaImpact any = row.SLAImpact
	if classificationVersion < 2 {
		customerVisible = nil
		slaImpact = nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ops_error_logs (
			id, api_key_id, request_id, client_request_id, group_id, created_at,
			requested_model, model, status_code, classification_version, customer_visible,
			sla_impact, event_scope, is_count_tokens, error_type, error_phase, error_message
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, row.ID, row.APIKeyID, row.RequestID, row.ClientRequestID, row.GroupID, row.CreatedAt,
		row.Requested, row.Model, row.StatusCode, classificationVersion, customerVisible,
		slaImpact, row.EventScope, row.IsCountTokens, row.ErrorType, row.ErrorPhase, row.ErrorMessage)
	require.NoError(t, err)
}

func queryModelRuntimeBuckets(t *testing.T, ctx context.Context, tx *sql.Tx, start, end time.Time) map[service.ModelRuntimeKey]map[int]service.ModelRuntimeBucket {
	t.Helper()
	rows, err := tx.QueryContext(ctx, modelRuntimeAggregationSQL, start, end)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	out := make(map[service.ModelRuntimeKey]map[int]service.ModelRuntimeBucket)
	for rows.Next() {
		var row service.ModelRuntimeBucket
		require.NoError(t, rows.Scan(&row.GroupID, &row.Model, &row.Hour, &row.Successes, &row.Failures,
			&row.FirstTokenSumMS, &row.FirstTokenCount, &row.DurationSumMS, &row.DurationCount,
			&row.OutputTokens, &row.GenerationTimeMS))
		key := row.ModelRuntimeKey
		if out[key] == nil {
			out[key] = make(map[int]service.ModelRuntimeBucket)
		}
		out[key][row.Hour] = row
	}
	require.NoError(t, rows.Err())
	return out
}

func sumModelRuntimeBuckets(buckets map[int]service.ModelRuntimeBucket) service.ModelRuntimeBucket {
	var total service.ModelRuntimeBucket
	for _, bucket := range buckets {
		total.Successes += bucket.Successes
		total.Failures += bucket.Failures
		total.FirstTokenSumMS += bucket.FirstTokenSumMS
		total.FirstTokenCount += bucket.FirstTokenCount
		total.DurationSumMS += bucket.DurationSumMS
		total.DurationCount += bucket.DurationCount
		total.OutputTokens += bucket.OutputTokens
		total.GenerationTimeMS += bucket.GenerationTimeMS
	}
	return total
}

func sortedModelRuntimeKeys(buckets map[service.ModelRuntimeKey]map[int]service.ModelRuntimeBucket) []service.ModelRuntimeKey {
	keys := make([]service.ModelRuntimeKey, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].GroupID != keys[j].GroupID {
			return keys[i].GroupID < keys[j].GroupID
		}
		return keys[i].Model < keys[j].Model
	})
	return keys
}
