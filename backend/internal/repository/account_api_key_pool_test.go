package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestReplaceAccountAPIKeys_PreservesExistingSecretWhenInputBlank(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	keyID := int64(11)
	accountID := int64(7)
	mapping := map[string]string{"haiku4.5": "claude-haiku-4.5"}
	mappingJSON, err := json.Marshal(mapping)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, api_key\s+FROM account_api_keys\s+WHERE account_id = \$1\s+FOR UPDATE`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key"}).AddRow(keyID, "old-secret"))
	mock.ExpectExec(`UPDATE account_api_keys\s+SET name = \$1, api_key = \$2, priority = \$3, status = \$4,\s+model_restriction_mode = \$5, model_mapping = \$6::jsonb, updated_at = NOW\(\)\s+WHERE id = \$7 AND account_id = \$8`).
		WithArgs("packycode group", "old-secret", 1, service.AccountAPIKeyStatusActive, "mapping", string(mappingJSON), keyID, accountID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM account_api_keys\s+WHERE account_id = \$1 AND NOT \(id = ANY\(\$2\)\)`).
		WithArgs(accountID, pq.Array([]int64{keyID})).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = repo.ReplaceAccountAPIKeys(context.Background(), accountID, []service.AccountAPIKeyInput{
		{
			ID:                   &keyID,
			Name:                 " packycode group ",
			APIKey:               "",
			Priority:             0,
			Status:               service.AccountAPIKeyStatusActive,
			ModelRestrictionMode: "mapping",
			ModelMapping:         mapping,
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReplaceAccountAPIKeys_DeletesExistingKeysWhenNoValidInputsRemain(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	accountID := int64(8)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, api_key\s+FROM account_api_keys\s+WHERE account_id = \$1\s+FOR UPDATE`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key"}).AddRow(int64(21), "old-secret"))
	mock.ExpectExec(`DELETE FROM account_api_keys WHERE account_id = \$1`).
		WithArgs(accountID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ReplaceAccountAPIKeys(context.Background(), accountID, []service.AccountAPIKeyInput{
		{Name: "blank row", APIKey: ""},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReplaceAccountAPIKeys_RejectsNonPositiveAccountID(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	err = repo.ReplaceAccountAPIKeys(context.Background(), 0, nil)
	require.ErrorIs(t, err, service.ErrAccountNotFound)
}

func TestReplaceAccountAPIKeys_InvalidStoredKeyIDInsertsAsNewWhenSecretProvided(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	unknownID := int64(999)
	accountID := int64(9)
	insertedID := int64(31)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, api_key\s+FROM account_api_keys\s+WHERE account_id = \$1\s+FOR UPDATE`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key"}))
	mock.ExpectQuery(`INSERT INTO account_api_keys \(account_id, name, api_key, priority, status, model_restriction_mode, model_mapping\)\s+VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, \$7::jsonb\)\s+RETURNING id`).
		WithArgs(accountID, "new key", "new-secret", 3, service.AccountAPIKeyStatusActive, "whitelist", sqlmockJSONMap{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(insertedID))
	mock.ExpectExec(`DELETE FROM account_api_keys\s+WHERE account_id = \$1 AND NOT \(id = ANY\(\$2\)\)`).
		WithArgs(accountID, pq.Array([]int64{insertedID})).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = repo.ReplaceAccountAPIKeys(context.Background(), accountID, []service.AccountAPIKeyInput{
		{ID: &unknownID, Name: "new key", APIKey: "new-secret", Priority: 3, Status: service.AccountAPIKeyStatusActive},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReplaceAccountAPIKeys_NormalizesLegacyErrorStatusToInactive(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	accountID := int64(10)
	insertedID := int64(41)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, api_key\s+FROM account_api_keys\s+WHERE account_id = \$1\s+FOR UPDATE`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key"}))
	mock.ExpectQuery(`INSERT INTO account_api_keys \(account_id, name, api_key, priority, status, model_restriction_mode, model_mapping\)\s+VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, \$7::jsonb\)\s+RETURNING id`).
		WithArgs(accountID, "old error key", "secret", service.DefaultAccountAPIKeyPriority, service.AccountAPIKeyStatusInactive, "whitelist", sqlmockJSONMap{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(insertedID))
	mock.ExpectExec(`DELETE FROM account_api_keys\s+WHERE account_id = \$1 AND NOT \(id = ANY\(\$2\)\)`).
		WithArgs(accountID, pq.Array([]int64{insertedID})).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = repo.ReplaceAccountAPIKeys(context.Background(), accountID, []service.AccountAPIKeyInput{
		{Name: "old error key", APIKey: "secret", Status: "error"},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

type sqlmockJSONMap struct{}

func (sqlmockJSONMap) Match(v driver.Value) bool {
	value, ok := v.(string)
	return ok && value == "{}"
}
