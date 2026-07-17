package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsErrorInsertContractIncludesMemberAttribution(t *testing.T) {
	memberID := int64(42)
	args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{
		MemberID:           &memberID,
		MemberCodeSnapshot: "finance-01",
		MemberNameSnapshot: "Finance",
	})

	require.Len(t, args, 53)
	require.Equal(t, sql.NullInt64{Int64: memberID, Valid: true}, args[4])
	require.Equal(t, sql.NullString{String: "finance-01", Valid: true}, args[5])
	require.Equal(t, sql.NullString{String: "Finance", Valid: true}, args[6])
	require.Contains(t, insertOpsErrorLogSQL, "member_id")
	require.Contains(t, insertOpsErrorLogSQL, "member_code_snapshot")
	require.Contains(t, insertOpsErrorLogSQL, "member_name_snapshot")
	require.Contains(t, insertOpsErrorLogSQL, "$53")
}
