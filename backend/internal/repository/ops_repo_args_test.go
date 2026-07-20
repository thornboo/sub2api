//go:build unit

package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsInsertErrorLogArgsPreservesExplicitZeroUpstreamStatus(t *testing.T) {
	zero := 0
	args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{UpstreamStatusCode: &zero})

	require.Len(t, args, 50)
	encoded, ok := args[39].(sql.NullInt64)
	require.True(t, ok)
	require.True(t, encoded.Valid)
	require.Zero(t, encoded.Int64)
}

func TestOpsNullableIntPointerDistinguishesNilZeroAndStatus(t *testing.T) {
	missing := opsNullableIntPointer(nil).(sql.NullInt64)
	require.False(t, missing.Valid)

	zeroValue := 0
	zero := opsNullableIntPointer(&zeroValue).(sql.NullInt64)
	require.True(t, zero.Valid)
	require.Zero(t, zero.Int64)

	statusValue := 503
	status := opsNullableIntPointer(&statusValue).(sql.NullInt64)
	require.True(t, status.Valid)
	require.EqualValues(t, 503, status.Int64)
}

func TestOpsInsertErrorLogArgsPersistsV2Classification(t *testing.T) {
	slaImpact := true
	args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{
		EventScope:            service.OpsEventScopeRequestTerminal,
		CustomerVisible:       true,
		FailureDomain:         service.OpsFailureDomainPlatform,
		FailureCategory:       service.OpsFailureCategoryRouting,
		FailureReason:         service.OpsFailureReasonNoAvailableAccounts,
		ResolutionOwner:       service.OpsResolutionOwnerPlatformOps,
		PoolOwnership:         service.OpsPoolOwnershipPlatform,
		SLAImpact:             &slaImpact,
		ClassificationVersion: service.OpsFailureClassificationVersion,
	})

	require.Equal(t, sql.NullString{String: service.OpsEventScopeRequestTerminal, Valid: true}, args[26])
	require.Equal(t, sql.NullBool{Bool: true, Valid: true}, args[27])
	require.Equal(t, sql.NullString{String: service.OpsFailureDomainPlatform, Valid: true}, args[28])
	require.Equal(t, sql.NullBool{Bool: true, Valid: true}, args[33])
	require.Equal(t, sql.NullInt64{Int64: int64(service.OpsFailureClassificationVersion), Valid: true}, args[34])
}
