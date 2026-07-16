package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustEnterpriseMemberTokenCount(t testing.TB, value string) EnterpriseMemberTokenCount {
	t.Helper()
	count, err := ParseEnterpriseMemberTokenCount(value)
	require.NoError(t, err)
	return count
}

func TestEnterpriseMemberTokenCountPreservesTwoDecimalPlacesAcrossJSONAndSQL(t *testing.T) {
	count, err := parseImportTokenCount("421.63")
	require.NoError(t, err)
	require.Equal(t, "421.63", count.String())

	payload, err := json.Marshal(struct {
		Count EnterpriseMemberTokenCount `json:"count"`
	}{Count: count})
	require.NoError(t, err)
	require.JSONEq(t, `{"count":"421.63"}`, string(payload))

	var decoded struct {
		Count EnterpriseMemberTokenCount `json:"count"`
	}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.True(t, count.Equal(decoded.Count))

	var scanned EnterpriseMemberTokenCount
	require.NoError(t, scanned.Scan("421.63"))
	require.True(t, count.Equal(scanned))
	driverValue, err := scanned.Value()
	require.NoError(t, err)
	require.Equal(t, "421.63", driverValue)
}

func TestEnterpriseMemberTokenCountAddsWithoutBinaryFloatDrift(t *testing.T) {
	left, err := parseImportTokenCount("0.10")
	require.NoError(t, err)
	right, err := parseImportTokenCount("0.20")
	require.NoError(t, err)
	require.Equal(t, "0.30", left.Add(right).String())
}

func TestEnterpriseMemberTokenCountKeepsAggregateRangeSeparateFromPersistedFieldRange(t *testing.T) {
	maximum := mustEnterpriseMemberTokenCount(t, "9223372036854775807.99")
	fraction := mustEnterpriseMemberTokenCount(t, "0.01")
	aggregate := maximum.Add(fraction)
	require.Equal(t, "9223372036854775808.00", aggregate.String())
	require.False(t, aggregate.IsPersistable())

	payload, err := json.Marshal(aggregate)
	require.NoError(t, err)
	require.JSONEq(t, `"9223372036854775808.00"`, string(payload))

	var decoded EnterpriseMemberTokenCount
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.True(t, aggregate.Equal(decoded))

	var scanned EnterpriseMemberTokenCount
	require.NoError(t, scanned.Scan("18446744073709551615.98"))
	require.Equal(t, "18446744073709551615.98", scanned.String())
	_, err = aggregate.Value()
	require.Error(t, err)
}

func TestEnterpriseMemberImportResultRoundTripsAggregateAboveSingleFieldLimit(t *testing.T) {
	maximum := mustEnterpriseMemberTokenCount(t, "9223372036854775807.99")
	result := EnterpriseMemberImportResult{
		MigrationTotalTokens: maximum.Add(maximum),
	}

	payload, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"migration_total_tokens":"18446744073709551615.98"`)

	var decoded EnterpriseMemberImportResult
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, "18446744073709551615.98", decoded.MigrationTotalTokens.String())
}

func TestEnterpriseMemberTokenCountReadsLegacyNumericJSON(t *testing.T) {
	var decoded struct {
		Count EnterpriseMemberTokenCount `json:"count"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"count":421.63}`), &decoded))
	require.Equal(t, "421.63", decoded.Count.String())
}
