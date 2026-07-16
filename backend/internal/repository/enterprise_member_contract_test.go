package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enterprisemember"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestEnterpriseMemberEntityToServiceSerializesEmptyGroupIDsAsArray(t *testing.T) {
	member := enterpriseMemberEntityToService(&dbent.EnterpriseMember{ID: 15})

	require.NotNil(t, member)
	require.NotNil(t, member.GroupIDs)
	require.Empty(t, member.GroupIDs)

	payload, err := json.Marshal(member)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"group_ids":[]`)
	require.NotContains(t, string(payload), `"group_ids":null`)
}

func TestEnterpriseMemberRepositoryListSerializesEmptyGroupIDsAsArray(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, time.July, 17, 8, 9, 10, 0, time.UTC)
	mock.ExpectQuery(`SELECT .* FROM "enterprise_members"`).
		WillReturnRows(sqlmock.NewRows(enterprisemember.Columns).AddRow(
			int64(15), now, now, nil, int64(7), "ceshi1", "测试", "disabled",
			0.0, 0.0, 0.0, 0.0, int64(1), nil,
		))
	mock.ExpectQuery(`SELECT .* FROM "enterprise_member_group_bindings"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "group_id", "sort_order", "created_at", "updated_at"}))
	mock.ExpectQuery(`SELECT .* FROM "api_keys"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "count"}))
	mock.ExpectQuery(`SELECT member_id, usage_5h, usage_1d, usage_7d`).
		WillReturnRows(sqlmock.NewRows([]string{
			"member_id", "usage_5h", "usage_1d", "usage_7d",
			"window_5h_start", "window_1d_start", "window_7d_start",
		}))

	repo := &enterpriseMemberRepository{client: client, db: db}
	members, err := repo.ListByOwner(context.Background(), 7, false)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.NotNil(t, members[0].GroupIDs)
	require.Empty(t, members[0].GroupIDs)

	payload, err := json.Marshal(members)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"group_ids":[]`)
	require.NotContains(t, string(payload), `"group_ids":null`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberRepositoryCreateSerializesEmptyGroupIDsAsArray(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 17, 8, 9, 10, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO enterprise_members`).
		WithArgs(int64(7), "ceshi1", "测试", service.EnterpriseMemberStatusDisabled, 0.0, 0.0, 0.0, 0.0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(15), now, now))
	mock.ExpectCommit()

	member := &service.EnterpriseMember{
		EnterpriseUserID: 7,
		MemberCode:       "ceshi1",
		Name:             "测试",
		Status:           service.EnterpriseMemberStatusDisabled,
	}
	repo := &enterpriseMemberRepository{db: db}
	require.NoError(t, repo.Create(context.Background(), member, nil, service.EnterpriseMemberOpeningUsage{}))
	require.NotNil(t, member.GroupIDs)
	require.Empty(t, member.GroupIDs)

	payload, err := json.Marshal(member)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"group_ids":[]`)
	require.NotContains(t, string(payload), `"group_ids":null`)
	require.NoError(t, mock.ExpectationsWereMet())
}
