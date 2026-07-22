package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelProtocolCapabilityRepositorySyncObservedNeverOverwritesOverride(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelProtocolCapabilityRepository{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectExec("pg_advisory_xact_lock").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("ON CONFLICT (account_id, upstream_model, protocol) DO UPDATE SET")).
		WithArgs(int64(7), "MiniMax-M3", service.ModelProtocolAnthropicMessages, service.ModelProtocolStateSupported, "upstream_model_list", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("ON CONFLICT (account_id, upstream_model, protocol) DO NOTHING")).
		WithArgs(int64(7), "Kimi-K2", service.ModelProtocolAnthropicMessages, service.ModelProtocolStateUnknown, "upstream_model_list_missing", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.SyncObserved(context.Background(), 7, []service.ModelProtocolObservation{
		{UpstreamModel: "MiniMax-M3", Protocol: service.ModelProtocolAnthropicMessages, State: service.ModelProtocolStateSupported, Source: "upstream_model_list", ObservedAt: now},
		{UpstreamModel: "Kimi-K2", Protocol: service.ModelProtocolAnthropicMessages, State: service.ModelProtocolStateUnknown, Source: "upstream_model_list_missing", ObservedAt: now},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelProtocolCapabilityRepositoryListsAccountsInOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelProtocolCapabilityRepository{db: db}
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{
		"id", "account_id", "upstream_model", "protocol", "override_state", "observed_state",
		"observed_source", "observed_at", "created_at", "updated_at",
	}).
		AddRow(int64(1), int64(7), "MiniMax-M3", service.ModelProtocolAnthropicMessages, service.ModelProtocolStateAuto, service.ModelProtocolStateSupported, "upstream_model_list", now, now, now).
		AddRow(int64(2), int64(8), "Kimi-K2", service.ModelProtocolOpenAIChat, service.ModelProtocolStateSupported, service.ModelProtocolStateUnknown, "", nil, now, now)
	mock.ExpectQuery("WHERE account_id = ANY").WithArgs(sqlmock.AnyArg()).WillReturnRows(rows)

	items, err := repo.ListByAccountIDs(context.Background(), []int64{7, 8})
	require.NoError(t, err)
	require.Len(t, items[7], 1)
	require.Len(t, items[8], 1)
	require.Equal(t, "MiniMax-M3", items[7][0].UpstreamModel)
	require.Equal(t, service.ModelProtocolStateSupported, items[8][0].OverrideState)
	require.NoError(t, mock.ExpectationsWereMet())
}
