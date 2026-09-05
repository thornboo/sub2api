//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newSessionIDUsageLog(sessionID *string) *service.UsageLog {
	return &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-session-id",
		Model:        "claude-3",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.0,
		ActualCost:   1.0,
		SessionID:    sessionID,
		CreatedAt:    time.Now().UTC(),
	}
}

// TestPrepareUsageLogInsert_SessionIDArgWiring pins the session_id column to the
// arg slice / arg-type table so the five INSERT column lists stay in sync. session_id
// is immediately before native_compaction_v2; created_at is always last.
func TestPrepareUsageLogInsert_SessionIDArgWiring(t *testing.T) {
	require.Len(t, usageLogInsertArgTypes, 73,
		"arg-type table must preserve upstream response audit, enterprise attribution, schedule_meta, route_plan, upstream cost evidence, upstream_request_id and session_id")
	sessionID := "sess-persisted-123"
	upstreamRequestID := "upstream-request-123"
	bindingID := int64(42)
	multiplier := 0.8
	currency := service.UpstreamPriceReferenceCurrencyCNY
	fx := 7.2
	expectedCost := 3.5
	log := newSessionIDUsageLog(&sessionID)
	log.UpstreamCostBindingID = &bindingID
	log.UpstreamGroupMultiplier = &multiplier
	log.UpstreamPriceReferenceCurrency = &currency
	log.UpstreamReferenceFXRate = &fx
	log.UpstreamExpectedCost = &expectedCost
	log.UpstreamRequestID = &upstreamRequestID
	prepared := prepareUsageLogInsert(log)

	require.Len(t, prepared.args, len(usageLogInsertArgTypes),
		"prepared args must match the arg-type table length")

	// created_at is last; native_compaction_v2 is penultimate; session_id precedes it.
	sessionArg := prepared.args[len(prepared.args)-3]
	ns, ok := sessionArg.(sql.NullString)
	require.True(t, ok, "session_id arg should be a sql.NullString, got %T", sessionArg)
	require.True(t, ns.Valid)
	require.Equal(t, sessionID, ns.String)

	require.Equal(t, "text", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-3],
		"session_id arg type must be text")
	require.Equal(t, "boolean", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-2],
		"native_compaction_v2 arg type must be boolean")
	require.Equal(t, "bigint", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-9])
	require.Equal(t, "numeric", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-8])
	require.Equal(t, "text", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-7])
	require.Equal(t, "numeric", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-6])
	require.Equal(t, "numeric", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-5])
	require.Equal(t, "text", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-4])
	require.Equal(t, &bindingID, prepared.args[len(prepared.args)-9])
	require.Equal(t, &multiplier, prepared.args[len(prepared.args)-8])
	priceReference, ok := prepared.args[len(prepared.args)-7].(sql.NullString)
	require.True(t, ok)
	require.True(t, priceReference.Valid)
	require.Equal(t, currency, priceReference.String)
	require.Equal(t, &fx, prepared.args[len(prepared.args)-6])
	require.Equal(t, &expectedCost, prepared.args[len(prepared.args)-5])
	upstreamRequestArg, ok := prepared.args[len(prepared.args)-4].(sql.NullString)
	require.True(t, ok)
	require.True(t, upstreamRequestArg.Valid)
	require.Equal(t, upstreamRequestID, upstreamRequestArg.String)
}

// TestPrepareUsageLogInsert_SessionIDNullWhenAbsent proves an absent session id is
// persisted as SQL NULL rather than an empty string.
func TestPrepareUsageLogInsert_SessionIDNullWhenAbsent(t *testing.T) {
	prepared := prepareUsageLogInsert(newSessionIDUsageLog(nil))
	sessionArg := prepared.args[len(prepared.args)-3]
	ns, ok := sessionArg.(sql.NullString)
	require.True(t, ok, "session_id arg should be a sql.NullString, got %T", sessionArg)
	require.False(t, ns.Valid, "absent session id must be NULL, not empty string")

	empty := ""
	preparedEmpty := prepareUsageLogInsert(newSessionIDUsageLog(&empty))
	nsEmpty := preparedEmpty.args[len(preparedEmpty.args)-3].(sql.NullString)
	require.False(t, nsEmpty.Valid, "empty session id must also be NULL")
}

func TestPrepareUsageLogInsert_RequestedReasoningEffortArgWiring(t *testing.T) {
	requested := "max"
	forwarded := "xhigh"
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:                   1,
		APIKeyID:                 2,
		AccountID:                3,
		RequestID:                "req-requested-effort",
		Model:                    "gpt-5.4",
		ReasoningEffort:          &forwarded,
		RequestedReasoningEffort: &requested,
		CreatedAt:                time.Now().UTC(),
	})

	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
	require.Equal(t, "text", usageLogInsertArgTypes[51], "requested_reasoning_effort must follow reasoning_effort")
	require.Equal(t, "text", usageLogInsertArgTypes[50], "reasoning_effort arg type must stay text")

	forwardedArg, ok := prepared.args[50].(sql.NullString)
	require.True(t, ok)
	require.True(t, forwardedArg.Valid)
	require.Equal(t, forwarded, forwardedArg.String)

	requestedArg, ok := prepared.args[51].(sql.NullString)
	require.True(t, ok)
	require.True(t, requestedArg.Valid)
	require.Equal(t, requested, requestedArg.String)
}

// TestUsageLogInsertQueries_IncludeSessionID guards that every generated INSERT path
// and the SELECT column list reference session_id.
func TestUsageLogInsertQueries_IncludeSessionID(t *testing.T) {
	require.Contains(t, usageLogSelectColumns, "requested_reasoning_effort",
		"SELECT column list must include requested_reasoning_effort")
	require.Contains(t, usageLogSelectColumns, "session_id",
		"SELECT column list must include session_id")

	sessionID := "sess-in-query"
	log := newSessionIDUsageLog(&sessionID)
	prepared := prepareUsageLogInsert(log)
	key := usageLogBatchKey(log.RequestID, log.APIKeyID)

	batchQuery, batchArgs := buildUsageLogBatchInsertQuery([]string{key},
		map[string]usageLogInsertPrepared{key: prepared})
	require.Contains(t, batchQuery, "session_id")
	require.Contains(t, batchQuery, "requested_reasoning_effort")
	// Two column references (INSERT column list + SELECT ... FROM input) plus the CTE def.
	require.GreaterOrEqual(t, strings.Count(batchQuery, "session_id"), 3)
	require.Len(t, batchArgs, len(prepared.args)+1,
		"batch args include the synthetic input_index before usage-log values")

	bestEffortQuery, bestEffortArgs := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.Contains(t, bestEffortQuery, "session_id")
	require.Len(t, bestEffortArgs, len(prepared.args))
}
