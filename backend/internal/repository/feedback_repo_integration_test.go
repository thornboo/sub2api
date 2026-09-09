//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

type feedbackIntegrationCooldown struct{}

func (feedbackIntegrationCooldown) ClaimFeedbackCooldown(context.Context, string, time.Duration) (bool, time.Duration, error) {
	return true, 0, nil
}

func TestFeedbackTicketTitleMigrationBackfillsHistoricalTitlesOnPostgres(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	_, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE feedbacks (
			content TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT ''
		) ON COMMIT DROP;
		INSERT INTO feedbacks (content, title) VALUES ('  第一行
			第二行  ', '');
	`)
	require.NoError(t, err)

	data, err := migrations.FS.ReadFile("242_feedback_ticket_title.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO pg_temp, public")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(data))
	require.NoError(t, err)

	var title string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT title FROM feedbacks`).Scan(&title))
	require.Equal(t, "第一行 第二行", title)

	_, err = tx.ExecContext(ctx, `INSERT INTO feedbacks (content, title) VALUES ('x', $1)`, strings.Repeat("界", service.FeedbackTitleMaxRunes+1))
	require.Error(t, err)
}

func TestFeedbackRepositoryPostgresDerivesReplyStatusAndFilters(t *testing.T) {
	ctx := context.Background()
	repo := NewFeedbackRepository(integrationDB)
	feedbackSvc := service.NewFeedbackService(repo, feedbackIntegrationCooldown{})
	userID, keyID := createFeedbackIntegrationOwner(t, ctx)
	session := &service.PublicKeyUsageSession{APIKeyID: keyID, UserID: userID}

	createdExplicit, err := feedbackSvc.CreateForUser(ctx, userID, "  显式\n标题  ", "explicit content")
	require.NoError(t, err)
	require.Equal(t, "显式 标题", createdExplicit.Feedback.Title)

	createdLegacy, err := feedbackSvc.CreateForUser(ctx, userID, "", "legacy opening\nsecond line")
	require.NoError(t, err)
	require.Equal(t, "legacy opening second line", createdLegacy.Feedback.Title)

	createdAdminLatest, err := feedbackSvc.CreateForUser(ctx, userID, "admin latest", "admin latest content")
	require.NoError(t, err)
	adminReplyResult, err := feedbackSvc.ReplyAsAdmin(ctx, createdAdminLatest.Feedback.ID, 9001, "admin answer")
	require.NoError(t, err)
	adminReplyID := adminReplyResult.Message.ID

	createdUserLatest, err := feedbackSvc.CreateForUser(ctx, userID, "tie title", "tie content")
	require.NoError(t, err)
	tieTime := time.Date(2026, 9, 9, 12, 5, 0, 0, time.UTC)
	insertFeedbackReplyAt(t, ctx, createdUserLatest.Feedback.ID, service.FeedbackActorAdmin, "admin same timestamp", tieTime)
	insertFeedbackReplyAt(t, ctx, createdUserLatest.Feedback.ID, service.FeedbackActorUser, "user same timestamp wins by id", tieTime)

	createdKeyTicket, err := feedbackSvc.CreateForKey(ctx, session, "key title", "key content")
	require.NoError(t, err)
	insertFeedbackReplyAt(t, ctx, createdKeyTicket.Feedback.ID, service.FeedbackActorAdmin, "key answer", time.Date(2026, 9, 9, 12, 10, 0, 0, time.UTC))

	legacyBlankID := insertLegacyBlankTitleFeedback(t, ctx, userID)
	loadedLegacyBlank, err := repo.Get(ctx, legacyBlankID, service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID}, service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID})
	require.NoError(t, err)
	require.Equal(t, "old blank title", loadedLegacyBlank.Title)
	require.Equal(t, service.FeedbackReplyStatusPending, loadedLegacyBlank.ReplyStatus)

	loadedAdminLatest, err := repo.Get(ctx, createdAdminLatest.Feedback.ID, service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID}, service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackReplyStatusReplied, loadedAdminLatest.ReplyStatus)

	adminProjection, err := repo.Get(ctx, createdAdminLatest.Feedback.ID, service.FeedbackScope{Kind: service.FeedbackActorAdmin}, service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 9001})
	require.NoError(t, err)
	require.Equal(t, "Feedback Owner", adminProjection.UserName)
	adminDTO := dto.AdminFeedbackFromService(adminProjection)
	require.Equal(t, "Feedback Owner", adminDTO.UserName)
	customerDTOJSON, err := json.Marshal(dto.FeedbackFromService(adminProjection))
	require.NoError(t, err)
	require.NotContains(t, string(customerDTOJSON), "user_name")
	require.NotContains(t, string(customerDTOJSON), "Feedback Owner")

	_, err = repo.MarkRead(ctx, createdAdminLatest.Feedback.ID, service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID}, service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID}, adminReplyID)
	require.NoError(t, err)
	loadedAfterRead, err := repo.Get(ctx, createdAdminLatest.Feedback.ID, service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID}, service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackReplyStatusReplied, loadedAfterRead.ReplyStatus)

	loadedUserLatest, err := repo.Get(ctx, createdUserLatest.Feedback.ID, service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID}, service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackReplyStatusPending, loadedUserLatest.ReplyStatus)

	repliedUserItems, repliedUserPage, err := repo.ListByScope(ctx,
		pagination.PaginationParams{Page: 1, PageSize: 10},
		service.FeedbackListFilters{ReplyStatus: service.FeedbackReplyStatusReplied},
		service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID},
		service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID},
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), repliedUserPage.Total)
	require.Len(t, repliedUserItems, 1)
	require.Equal(t, createdAdminLatest.Feedback.ID, repliedUserItems[0].ID)

	pendingOpenItems, pendingOpenPage, err := repo.ListByScope(ctx,
		pagination.PaginationParams{Page: 1, PageSize: 2},
		service.FeedbackListFilters{Status: service.FeedbackStatusOpen, ReplyStatus: service.FeedbackReplyStatusPending},
		service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: userID},
		service.FeedbackReader{Kind: service.FeedbackActorUser, ID: userID},
	)
	require.NoError(t, err)
	require.Equal(t, int64(4), pendingOpenPage.Total)
	require.Equal(t, 2, pendingOpenPage.Pages)
	require.Len(t, pendingOpenItems, 2)

	repliedKeyItems, repliedKeyPage, err := repo.ListByScope(ctx,
		pagination.PaginationParams{Page: 1, PageSize: 10},
		service.FeedbackListFilters{ReplyStatus: service.FeedbackReplyStatusReplied},
		service.FeedbackScope{Kind: service.FeedbackSourceKey, UserID: userID, APIKeyID: keyID},
		service.FeedbackReader{Kind: service.FeedbackSourceKey, ID: keyID},
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), repliedKeyPage.Total)
	require.Len(t, repliedKeyItems, 1)
	require.Equal(t, createdKeyTicket.Feedback.ID, repliedKeyItems[0].ID)

	closed, err := repo.Close(ctx, createdAdminLatest.Feedback.ID, service.FeedbackActorAdmin, service.FeedbackScope{Kind: service.FeedbackActorAdmin}, service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 9001})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackStatusClosed, closed.Status)
	require.Equal(t, service.FeedbackReplyStatusReplied, closed.ReplyStatus)
	require.Equal(t, service.FeedbackReplyStatusReplied, dto.AdminFeedbackFromService(closed).ReplyStatus)
}

func createFeedbackIntegrationOwner(t *testing.T, ctx context.Context) (int64, int64) {
	t.Helper()
	email := uniqueTestValue(t, "feedback-ticket") + "@example.test"
	var userID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, status, balance, concurrency)
		VALUES ($1, 'Feedback Owner', 'integration-hash', $2, $3, 0, 5)
		RETURNING id`, email, service.RoleUser, service.StatusActive).Scan(&userID))

	keyValue := "sk-" + integrationHash(t.Name())[:32]
	var keyID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, status)
		VALUES ($1, $2, 'Feedback integration key', $3)
		RETURNING id`, userID, keyValue, service.StatusActive).Scan(&keyID))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, err := integrationDB.ExecContext(cleanupCtx, `DELETE FROM feedbacks WHERE user_id = $1`, userID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(cleanupCtx, `DELETE FROM api_keys WHERE user_id = $1`, userID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		require.NoError(t, err)
	})

	return userID, keyID
}

func insertFeedbackReplyAt(t *testing.T, ctx context.Context, feedbackID int64, authorRole string, content string, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO feedback_replies (feedback_id, author_role, content, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, feedbackID, authorRole, content, createdAt).Scan(&id))
	return id
}

func insertLegacyBlankTitleFeedback(t *testing.T, ctx context.Context, userID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO feedbacks (title, content, source, status, user_id)
		VALUES ('', 'old blank title', $1, $2, $3)
		RETURNING id`, service.FeedbackSourceUser, service.FeedbackStatusOpen, userID).Scan(&id))
	return id
}
