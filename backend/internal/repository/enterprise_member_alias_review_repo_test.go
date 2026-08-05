package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAliasReviewRepositoryListUsesBoundedShadowUsageEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewEnterpriseMemberAliasReviewRepository(db)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	reviewedAt := now.Add(-time.Hour)
	groupID := int64(12)
	channelID := int64(34)
	reviewerID := int64(56)
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAliasReviewListSQL)).
		WithArgs(now.AddDate(0, 0, -30), now, now.AddDate(0, 0, -7), 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"public_model", "public_model_norm", "endpoint", "request_count_7d", "request_count_30d",
			"success_count_7d", "success_count_30d", "affected_enterprise_count_7d", "affected_enterprise_count_30d",
			"last_seen_at", "final_group_id", "channel_id", "reason_codes", "review_status", "reviewed_by",
			"reviewed_at", "review_note", "stable_route_evidence",
		}).AddRow(
			"Alias-Model", "alias-model", "/v1/responses", int64(3), int64(9), int64(3), int64(9),
			int64(2), int64(4), now.Add(-time.Minute), groupID, channelID, `{"model_unpublished","model_unsupported"}`,
			"registered", reviewerID, reviewedAt, "mapped", "registered_exact_stable_route",
		))

	items, err := repo.ListLegacySuccessNewPruned(context.Background(), service.EnterpriseMemberAliasReviewListInput{Now: now})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "success", items[0].LegacyOutcome)
	require.Equal(t, "pruned", items[0].PlannedOutcome)
	require.Equal(t, []string{"model_unpublished", "model_unsupported"}, items[0].ReasonCodes)
	require.Equal(t, &groupID, items[0].FinalGroupID)
	require.Equal(t, &channelID, items[0].ChannelID)
	require.Equal(t, &reviewerID, items[0].ReviewedBy)
	require.Equal(t, &reviewedAt, items[0].ReviewedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAliasReviewRepositoryUpsertStoresSanitizedReviewOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewEnterpriseMemberAliasReviewRepository(db)
	groupID := int64(12)
	channelID := int64(34)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	evidence := json.RawMessage(`{"publication_source":"exact_mapping"}`)
	mock.ExpectQuery("INSERT INTO enterprise_member_model_alias_reviews").
		WithArgs("Alias", "alias", "/v1/responses", "registered", &groupID, &channelID, "ok", string(evidence), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_model", "public_model_norm", "endpoint", "status", "final_group_id", "channel_id",
			"review_note", "validation_evidence", "reviewed_by", "reviewed_at", "created_at", "updated_at",
		}).AddRow(int64(1), "Alias", "alias", "/v1/responses", "registered", groupID, channelID, "ok", []byte(evidence), int64(7), now, now, now))

	record, err := repo.UpsertReview(context.Background(), service.EnterpriseMemberAliasReviewUpsert{
		PublicModel: "Alias", PublicModelNorm: "alias", Endpoint: "/v1/responses", Status: "registered",
		FinalGroupID: &groupID, ChannelID: &channelID, ReviewNote: "ok", ValidationEvidence: evidence, ReviewedBy: 7,
	})
	require.NoError(t, err)
	require.Equal(t, "registered", record.Status)
	require.JSONEq(t, string(evidence), string(record.ValidationEvidence))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAliasReviewRepositoryReadinessBlocksPendingOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewEnterpriseMemberAliasReviewRepository(db)
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAliasReviewReadinessSQL)).
		WithArgs(now.AddDate(0, 0, -30), now, now.AddDate(0, 0, -7)).
		WillReturnRows(sqlmock.NewRows([]string{"active_7d", "active_30d", "blocking_7d", "blocking_30d"}).
			AddRow(int64(2), int64(5), int64(1), int64(1)))

	summary, err := repo.GetReadinessSummary(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, int64(1), summary.BlockingUnreviewedActive7d)
	require.Equal(t, int64(5), summary.LegacySuccessNewPrunedActive30d)
	require.NoError(t, mock.ExpectationsWereMet())
}
