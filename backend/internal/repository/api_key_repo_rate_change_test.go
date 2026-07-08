package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepository_DisableKeysForGroupUsersRateChange_ExecutesMatchingUpdate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := &apiKeyRepository{sql: db}
	mock.ExpectExec(`(?s)UPDATE api_keys.*WHERE group_id = \$1.*AND user_id = ANY\(\$2\).*AND status = \$5.*AND deleted_at IS NULL`).
		WithArgs(
			int64(11),
			sqlmock.AnyArg(),
			service.StatusAPIKeyDisabled,
			service.APIKeyDisabledReasonRateChanged,
			service.StatusAPIKeyActive,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := repo.DisableKeysForGroupUsersRateChange(context.Background(), 11, []int64{42})

	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_DisableKeysForGroupRateChange_ExecutesDefaultRateTargetingUpdate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := &apiKeyRepository{sql: db}
	mock.ExpectExec(`(?s)UPDATE api_keys AS k.*WHERE k\.group_id = \$1.*AND k\.status = \$4.*AND k\.deleted_at IS NULL.*AND NOT EXISTS.*FROM user_group_rate_multipliers AS ugr.*ugr\.group_id = \$1.*ugr\.user_id = k\.user_id.*ugr\.rate_multiplier IS NOT NULL`).
		WithArgs(
			int64(11),
			service.StatusAPIKeyDisabled,
			service.APIKeyDisabledReasonRateChanged,
			service.StatusAPIKeyActive,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := repo.DisableKeysForGroupRateChange(context.Background(), 11)

	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_DisableKeysForGroupUsersRateChange_EmptyUsersSkipsUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &apiKeyRepository{sql: db}
	affected, err := repo.DisableKeysForGroupUsersRateChange(context.Background(), 11, nil)

	require.NoError(t, err)
	require.Equal(t, int64(0), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}
