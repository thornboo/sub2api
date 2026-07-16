package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func mustEnterpriseMemberTokenCount(t testing.TB, value string) service.EnterpriseMemberTokenCount {
	t.Helper()
	count, err := service.ParseEnterpriseMemberTokenCount(value)
	require.NoError(t, err)
	return count
}
