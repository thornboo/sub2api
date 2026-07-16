package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/enterprisemember"
	"github.com/Wei-Shaw/sub2api/ent/enterprisemembergroupbinding"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseMemberRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewEnterpriseMemberRepository(client *dbent.Client, sqlDB *sql.DB) service.EnterpriseMemberRepository {
	return &enterpriseMemberRepository{client: client, db: sqlDB}
}

func (r *enterpriseMemberRepository) ListByOwner(ctx context.Context, ownerID int64, includeArchived bool) ([]service.EnterpriseMember, error) {
	queryCtx := ctx
	if includeArchived {
		queryCtx = mixins.SkipSoftDelete(ctx)
	}
	rows, err := r.client.EnterpriseMember.Query().
		Where(
			enterprisemember.EnterpriseUserIDEQ(ownerID),
			enterprisemember.RemovedAtIsNil(),
		).
		Order(dbent.Asc(enterprisemember.FieldID)).
		All(queryCtx)
	if err != nil {
		return nil, err
	}
	out := make([]service.EnterpriseMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, *enterpriseMemberEntityToService(row))
	}
	if err := r.enrichMembers(queryCtx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *enterpriseMemberRepository) GetByOwnerAndID(ctx context.Context, ownerID, memberID int64, includeArchived bool) (*service.EnterpriseMember, error) {
	queryCtx := ctx
	if includeArchived {
		queryCtx = mixins.SkipSoftDelete(ctx)
	}
	row, err := r.client.EnterpriseMember.Query().
		Where(
			enterprisemember.IDEQ(memberID),
			enterprisemember.EnterpriseUserIDEQ(ownerID),
			enterprisemember.RemovedAtIsNil(),
		).
		Only(queryCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrEnterpriseMemberNotFound
		}
		return nil, err
	}
	out := enterpriseMemberEntityToService(row)
	items := []service.EnterpriseMember{*out}
	if err := r.enrichMembers(queryCtx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *enterpriseMemberRepository) Create(ctx context.Context, member *service.EnterpriseMember, groupIDs []int64, opening service.EnterpriseMemberOpeningUsage) (err error) {
	if member == nil || r == nil || r.db == nil {
		return service.ErrEnterpriseMemberInvalid
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_members
			(enterprise_user_id, member_code, name, status,
			 monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
		RETURNING id, created_at, updated_at`,
		member.EnterpriseUserID, member.MemberCode, member.Name, member.Status,
		member.MonthlyLimitUSD, member.RateLimit5h, member.RateLimit1d, member.RateLimit7d,
	).Scan(&member.ID, &member.CreatedAt, &member.UpdatedAt)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrEnterpriseMemberConflict)
	}

	for sortOrder, groupID := range groupIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_group_bindings (member_id, group_id, sort_order)
			VALUES ($1, $2, $3)`, member.ID, groupID, sortOrder); err != nil {
			return err
		}
	}

	if opening.MonthlyUsedUSD > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_budget_periods
				(member_id, period_start, timezone, used_usd)
			VALUES ($1, $2, $3, $4)`,
			member.ID, opening.PeriodStart, enterpriseBudgetTimezone(), opening.MonthlyUsedUSD); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_budget_entries
				(member_id, period_start, kind, amount_usd, idempotency_key, actor_user_id, note)
			VALUES ($1, $2, 'migration_opening', $3, $4, $5, $6)`,
			member.ID, opening.PeriodStart, opening.MonthlyUsedUSD,
			opening.IdempotencyKey, opening.ActorUserID, opening.Note); err != nil {
			return err
		}
	}

	if opening.Usage5h > 0 || opening.Usage1d > 0 || opening.Usage7d > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_rate_limit_periods
				(member_id, usage_5h, usage_1d, usage_7d,
				 window_5h_start, window_1d_start, window_7d_start)
			VALUES ($1, $2, $3, $4,
				CASE WHEN CAST($2 AS NUMERIC) > 0 THEN NOW() END,
				CASE WHEN CAST($3 AS NUMERIC) > 0 THEN NOW() END,
				CASE WHEN CAST($4 AS NUMERIC) > 0 THEN NOW() END)`,
			member.ID, opening.Usage5h, opening.Usage1d, opening.Usage7d); err != nil {
			return err
		}
	}

	if opening.HasUsage() {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_audit_logs
				(enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id,
				 before_data, after_data, metadata)
			VALUES ($1, $2, $3, 'member.usage_adjusted', 'member', $2,
				jsonb_build_object(
					'monthly_used_usd', CAST(0 AS NUMERIC),
					'usage_5h', CAST(0 AS NUMERIC),
					'usage_1d', CAST(0 AS NUMERIC),
					'usage_7d', CAST(0 AS NUMERIC)),
				jsonb_build_object(
					'monthly_used_usd', CAST($4 AS NUMERIC),
					'usage_5h', CAST($5 AS NUMERIC),
					'usage_1d', CAST($6 AS NUMERIC),
					'usage_7d', CAST($7 AS NUMERIC)),
				jsonb_build_object(
					'note', CAST($8 AS TEXT),
					'idempotency_key', CAST($9 AS TEXT),
					'source', 'member_create'))`,
			member.EnterpriseUserID, member.ID, opening.ActorUserID,
			opening.MonthlyUsedUSD, opening.Usage5h, opening.Usage1d, opening.Usage7d,
			opening.Note, opening.IdempotencyKey); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	member.Version = 1
	member.GroupIDs = append([]int64{}, groupIDs...)
	member.Usage5h = opening.Usage5h
	member.Usage1d = opening.Usage1d
	member.Usage7d = opening.Usage7d
	return nil
}

func (r *enterpriseMemberRepository) Update(ctx context.Context, member *service.EnterpriseMember, expectedVersion int64) error {
	if member == nil || expectedVersion <= 0 {
		return service.ErrEnterpriseMemberInvalid
	}
	now := time.Now()
	affected, err := r.client.EnterpriseMember.Update().
		Where(
			enterprisemember.IDEQ(member.ID),
			enterprisemember.EnterpriseUserIDEQ(member.EnterpriseUserID),
			enterprisemember.VersionEQ(expectedVersion),
			enterprisemember.DeletedAtIsNil(),
			enterprisemember.RemovedAtIsNil(),
		).
		SetName(member.Name).
		SetMonthlyLimitUsd(member.MonthlyLimitUSD).
		SetRateLimit5h(member.RateLimit5h).
		SetRateLimit1d(member.RateLimit1d).
		SetRateLimit7d(member.RateLimit7d).
		AddVersion(1).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrEnterpriseMemberConflict)
	}
	if affected == 0 {
		return service.ErrEnterpriseMemberVersion
	}
	member.Version = expectedVersion + 1
	member.UpdatedAt = now
	return nil
}

func (r *enterpriseMemberRepository) SetStatus(ctx context.Context, ownerID, memberID, expectedVersion int64, status string) (*service.EnterpriseMember, error) {
	now := time.Now()
	affected, err := r.client.EnterpriseMember.Update().
		Where(
			enterprisemember.IDEQ(memberID),
			enterprisemember.EnterpriseUserIDEQ(ownerID),
			enterprisemember.VersionEQ(expectedVersion),
			enterprisemember.DeletedAtIsNil(),
			enterprisemember.RemovedAtIsNil(),
		).
		SetStatus(status).
		AddVersion(1).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrEnterpriseMemberVersion
	}
	return r.GetByOwnerAndID(ctx, ownerID, memberID, false)
}

func (r *enterpriseMemberRepository) Archive(ctx context.Context, ownerID, memberID, expectedVersion int64) error {
	now := time.Now()
	affected, err := r.client.EnterpriseMember.Update().
		Where(
			enterprisemember.IDEQ(memberID),
			enterprisemember.EnterpriseUserIDEQ(ownerID),
			enterprisemember.VersionEQ(expectedVersion),
			enterprisemember.DeletedAtIsNil(),
			enterprisemember.RemovedAtIsNil(),
		).
		SetStatus(service.EnterpriseMemberStatusDisabled).
		SetDeletedAt(now).
		AddVersion(1).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrEnterpriseMemberVersion
	}
	return nil
}

func (r *enterpriseMemberRepository) Restore(ctx context.Context, ownerID, memberID, expectedVersion int64) (*service.EnterpriseMember, error) {
	queryCtx := mixins.SkipSoftDelete(ctx)
	now := time.Now()
	affected, err := r.client.EnterpriseMember.Update().
		Where(
			enterprisemember.IDEQ(memberID),
			enterprisemember.EnterpriseUserIDEQ(ownerID),
			enterprisemember.VersionEQ(expectedVersion),
			enterprisemember.DeletedAtNotNil(),
			enterprisemember.RemovedAtIsNil(),
		).
		SetStatus(service.EnterpriseMemberStatusDisabled).
		ClearDeletedAt().
		AddVersion(1).
		SetUpdatedAt(now).
		Save(queryCtx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrEnterpriseMemberVersion
	}
	return r.GetByOwnerAndID(ctx, ownerID, memberID, false)
}

func (r *enterpriseMemberRepository) DeletePermanently(ctx context.Context, ownerID, memberID int64) (*service.EnterpriseMemberDeletionResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member repository sql db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM enterprise_members
		WHERE id = $1 AND enterprise_user_id = $2
		  AND deleted_at IS NOT NULL AND removed_at IS NULL
		FOR UPDATE`, memberID, ownerID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEnterpriseMemberNotFound
		}
		return nil, err
	}
	historicalMemberIDs, err := enterpriseMembersWithHistoricalFacts(ctx, tx, []int64{memberID})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM enterprise_member_group_bindings WHERE member_id = $1`, memberID); err != nil {
		return nil, err
	}
	if _, hasHistoricalFacts := historicalMemberIDs[memberID]; hasHistoricalFacts {
		if _, err := tx.ExecContext(ctx, `
			UPDATE api_keys
			SET status = $2,
			    disabled_reason = $3,
			    deleted_at = COALESCE(deleted_at, NOW()),
			    updated_at = NOW()
			WHERE member_id = $1 AND deleted_at IS NULL`,
			memberID, service.StatusAPIKeyDisabled, service.APIKeyDisabledReasonMemberRemoved); err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE enterprise_members
			SET member_code = '~deleted~' || id::text,
			    name = 'Deleted member #' || id::text,
			    status = 'disabled',
			    removed_at = NOW(),
			    deleted_at = COALESCE(deleted_at, NOW()),
			    version = version + 1,
			    updated_at = NOW()
			WHERE id = $1 AND enterprise_user_id = $2 AND removed_at IS NULL`, memberID, ownerID)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, service.ErrEnterpriseMemberNotFound
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.EnterpriseMemberDeletionResult{Mode: service.EnterpriseMemberDeletionModeTombstone}, nil
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM enterprise_members WHERE id = $1 AND enterprise_user_id = $2`, memberID, ownerID)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrEnterpriseMemberNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.EnterpriseMemberDeletionResult{Mode: service.EnterpriseMemberDeletionModeHardDelete}, nil
}

func (r *enterpriseMemberRepository) ReplaceGroups(ctx context.Context, ownerID, memberID, expectedVersion int64, groupIDs []int64) (*service.EnterpriseMember, error) {
	err := r.runInTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		update := client.EnterpriseMember.Update().
			Where(
				enterprisemember.IDEQ(memberID),
				enterprisemember.EnterpriseUserIDEQ(ownerID),
				enterprisemember.VersionEQ(expectedVersion),
				enterprisemember.DeletedAtIsNil(),
			).
			AddVersion(1).
			SetUpdatedAt(time.Now())
		if len(groupIDs) == 0 {
			update.SetStatus(service.EnterpriseMemberStatusDisabled)
		}
		affected, err := update.Save(txCtx)
		if err != nil {
			return err
		}
		if affected == 0 {
			return service.ErrEnterpriseMemberVersion
		}
		if _, err := client.EnterpriseMemberGroupBinding.Delete().
			Where(enterprisemembergroupbinding.MemberIDEQ(memberID)).
			Exec(txCtx); err != nil {
			return err
		}
		return createEnterpriseMemberBindings(txCtx, client, memberID, groupIDs)
	})
	if err != nil {
		return nil, err
	}
	return r.GetByOwnerAndID(ctx, ownerID, memberID, false)
}

func (r *enterpriseMemberRepository) BatchReplaceGroups(ctx context.Context, ownerID int64, targets []service.BatchEnterpriseMemberGroupTarget) ([]service.BatchEnterpriseMemberGroupUpdate, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrEnterpriseMemberInvalid
	}
	ordered := append([]service.BatchEnterpriseMemberGroupTarget(nil), targets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	groupSet := make(map[int64]struct{})
	for _, target := range ordered {
		for _, groupID := range target.GroupIDs {
			groupSet[groupID] = struct{}{}
		}
	}
	groupIDs := make([]int64, 0, len(groupSet))
	for groupID := range groupSet {
		groupIDs = append(groupIDs, groupID)
	}
	if err := validateEnterpriseMemberGroupAuthorization(ctx, tx, ownerID, groupIDs); err != nil {
		return nil, err
	}

	updatesByID := make(map[int64]service.BatchEnterpriseMemberGroupUpdate, len(ordered))
	for _, target := range ordered {
		var version int64
		var status string
		var updatedAt time.Time
		updateErr := tx.QueryRowContext(ctx, `
			UPDATE enterprise_members
			SET version = version + 1, updated_at = NOW(),
			    status = CASE WHEN $4 THEN 'disabled' ELSE status END
			WHERE id = $1 AND enterprise_user_id = $2 AND version = $3 AND deleted_at IS NULL
			RETURNING version, status, updated_at`,
			target.ID, ownerID, target.ExpectedVersion, len(target.GroupIDs) == 0).
			Scan(&version, &status, &updatedAt)
		if updateErr != nil {
			if errors.Is(updateErr, sql.ErrNoRows) {
				return nil, service.ErrEnterpriseMemberVersion
			}
			return nil, updateErr
		}
		if _, deleteErr := tx.ExecContext(ctx, `DELETE FROM enterprise_member_group_bindings WHERE member_id = $1`, target.ID); deleteErr != nil {
			return nil, deleteErr
		}
		for order, groupID := range target.GroupIDs {
			if _, insertErr := tx.ExecContext(ctx, `INSERT INTO enterprise_member_group_bindings (member_id, group_id, sort_order) VALUES ($1, $2, $3)`, target.ID, groupID, order); insertErr != nil {
				return nil, insertErr
			}
		}
		updatesByID[target.ID] = service.BatchEnterpriseMemberGroupUpdate{
			ID: target.ID, Version: version, GroupIDs: append([]int64{}, target.GroupIDs...), Status: status, UpdatedAt: updatedAt,
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	updated := make([]service.BatchEnterpriseMemberGroupUpdate, 0, len(targets))
	for _, target := range targets {
		updated = append(updated, updatesByID[target.ID])
	}
	return updated, nil
}

func (r *enterpriseMemberRepository) BatchUpdate(ctx context.Context, ownerID int64, targets []service.EnterpriseMemberBatchTarget, patch service.BatchEnterpriseMemberPolicyPatch) ([]service.BatchEnterpriseMemberUpdate, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrEnterpriseMemberInvalid
	}
	ordered := append([]service.EnterpriseMemberBatchTarget(nil), targets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if patch.GroupMode != "keep" {
		if err := validateEnterpriseMemberGroupAuthorization(ctx, tx, ownerID, patch.GroupIDs); err != nil {
			return nil, err
		}
	}

	updatesByID := make(map[int64]service.BatchEnterpriseMemberUpdate, len(ordered))
	for _, target := range ordered {
		var currentStatus string
		var currentVersion int64
		if err := tx.QueryRowContext(ctx, `
			SELECT status, version
			FROM enterprise_members
			WHERE id = $1 AND enterprise_user_id = $2
			  AND deleted_at IS NULL AND removed_at IS NULL
			FOR UPDATE`, target.ID, ownerID).Scan(&currentStatus, &currentVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, service.ErrEnterpriseMemberNotFound
			}
			return nil, err
		}
		if currentVersion != target.ExpectedVersion {
			return nil, service.ErrEnterpriseMemberVersion
		}

		currentGroups, err := enterpriseMemberGroupIDsInTx(ctx, tx, target.ID)
		if err != nil {
			return nil, err
		}
		desiredGroups := append([]int64(nil), currentGroups...)
		switch patch.GroupMode {
		case "replace":
			desiredGroups = append([]int64(nil), patch.GroupIDs...)
		case "append":
			desiredGroups = appendUniqueEnterpriseMemberGroupIDsForBatch(currentGroups, patch.GroupIDs)
			if err := validateEnterpriseMemberGroupAuthorization(ctx, tx, ownerID, desiredGroups); err != nil {
				return nil, err
			}
		}

		desiredStatus := currentStatus
		if patch.Status != nil {
			desiredStatus = *patch.Status
		}
		if len(desiredGroups) == 0 {
			if desiredStatus == service.EnterpriseMemberStatusActive {
				return nil, service.ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{
					"field": "group_ids", "reason": "required_to_enable", "member_id": fmt.Sprint(target.ID),
				})
			}
			if patch.GroupMode != "keep" {
				desiredStatus = service.EnterpriseMemberStatusDisabled
			}
		}
		if desiredStatus == service.EnterpriseMemberStatusActive && patch.GroupMode == "keep" {
			if err := validateEnterpriseMemberGroupAuthorization(ctx, tx, ownerID, desiredGroups); err != nil {
				return nil, err
			}
		}

		var updated service.BatchEnterpriseMemberUpdate
		updated.ID = target.ID
		if err := tx.QueryRowContext(ctx, `
			UPDATE enterprise_members
			SET monthly_limit_usd = COALESCE($4, monthly_limit_usd),
			    rate_limit_5h = COALESCE($5, rate_limit_5h),
			    rate_limit_1d = COALESCE($6, rate_limit_1d),
			    rate_limit_7d = COALESCE($7, rate_limit_7d),
			    status = $8,
			    version = version + 1,
			    updated_at = NOW()
			WHERE id = $1 AND enterprise_user_id = $2 AND version = $3
			  AND deleted_at IS NULL AND removed_at IS NULL
			RETURNING version, status, monthly_limit_usd, rate_limit_5h,
			          rate_limit_1d, rate_limit_7d, updated_at`,
			target.ID, ownerID, target.ExpectedVersion,
			optionalBatchFloat(patch.MonthlyLimitUSD), optionalBatchFloat(patch.RateLimit5h),
			optionalBatchFloat(patch.RateLimit1d), optionalBatchFloat(patch.RateLimit7d), desiredStatus).
			Scan(&updated.Version, &updated.Status, &updated.MonthlyLimitUSD, &updated.RateLimit5h,
				&updated.RateLimit1d, &updated.RateLimit7d, &updated.UpdatedAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, service.ErrEnterpriseMemberVersion
			}
			return nil, err
		}

		if patch.GroupMode != "keep" {
			if _, err := tx.ExecContext(ctx, `DELETE FROM enterprise_member_group_bindings WHERE member_id = $1`, target.ID); err != nil {
				return nil, err
			}
			for order, groupID := range desiredGroups {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO enterprise_member_group_bindings (member_id, group_id, sort_order)
					VALUES ($1, $2, $3)`, target.ID, groupID, order); err != nil {
					return nil, err
				}
			}
		}
		updated.GroupIDs = append([]int64{}, desiredGroups...)
		updatesByID[target.ID] = updated
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	updated := make([]service.BatchEnterpriseMemberUpdate, 0, len(targets))
	for _, target := range targets {
		updated = append(updated, updatesByID[target.ID])
	}
	return updated, nil
}

func enterpriseMemberGroupIDsInTx(ctx context.Context, tx *sql.Tx, memberID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT group_id
		FROM enterprise_member_group_bindings
		WHERE member_id = $1
		ORDER BY sort_order, group_id`, memberID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	groupIDs := make([]int64, 0)
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groupIDs, nil
}

func optionalBatchFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func appendUniqueEnterpriseMemberGroupIDsForBatch(existing, appended []int64) []int64 {
	result := append([]int64(nil), existing...)
	seen := make(map[int64]struct{}, len(existing)+len(appended))
	for _, groupID := range existing {
		seen[groupID] = struct{}{}
	}
	for _, groupID := range appended {
		if _, exists := seen[groupID]; exists {
			continue
		}
		seen[groupID] = struct{}{}
		result = append(result, groupID)
	}
	return result
}

func validateEnterpriseMemberGroupAuthorization(ctx context.Context, tx *sql.Tx, ownerID int64, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	var authorizedCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM groups g
		WHERE g.id = ANY($2) AND g.deleted_at IS NULL AND g.status = 'active' AND (
			(g.subscription_type = 'subscription' AND EXISTS (
				SELECT 1 FROM user_subscriptions us
				WHERE us.user_id = $1 AND us.group_id = g.id AND us.deleted_at IS NULL
				  AND us.status = 'active' AND us.starts_at <= NOW() AND us.expires_at > NOW()
			)) OR
			(g.subscription_type <> 'subscription' AND (
				NOT g.is_exclusive OR EXISTS (
					SELECT 1 FROM user_allowed_groups uag WHERE uag.user_id = $1 AND uag.group_id = g.id
				)
			))
		)`, ownerID, pq.Array(groupIDs)).Scan(&authorizedCount); err != nil {
		return err
	}
	if authorizedCount != len(groupIDs) {
		return service.ErrGroupNotAllowed
	}
	return nil
}

func (r *enterpriseMemberRepository) ListKeys(ctx context.Context, ownerID, memberID int64) ([]service.APIKey, error) {
	rows, err := r.client.APIKey.Query().
		Where(
			apikey.UserIDEQ(ownerID),
			apikey.MemberIDEQ(memberID),
			apikey.DeletedAtIsNil(),
		).
		Order(dbent.Asc(apikey.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.APIKey, 0, len(rows))
	for _, row := range rows {
		out = append(out, *apiKeyEntityToService(row))
	}
	return out, nil
}

func (r *enterpriseMemberRepository) ListAdoptableKeys(ctx context.Context, ownerID int64) ([]service.APIKey, error) {
	rows, err := r.client.APIKey.Query().
		Where(
			apikey.UserIDEQ(ownerID),
			apikey.MemberIDIsNil(),
			apikey.GroupIDNotNil(),
			apikey.StatusEQ(service.StatusAPIKeyActive),
			apikey.DeletedAtIsNil(),
		).
		WithGroup().
		Order(dbent.Asc(apikey.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.APIKey, 0, len(rows))
	for _, row := range rows {
		out = append(out, *apiKeyEntityToService(row))
	}
	return out, nil
}

func (r *enterpriseMemberRepository) AdoptKey(ctx context.Context, ownerID, memberID, keyID, expectedVersion int64) (*service.EnterpriseMemberKeyAdoptionResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member repository sql db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var memberStatus string
	var memberDeletedAt sql.NullTime
	var currentVersion int64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, deleted_at, version
		FROM enterprise_members
		WHERE id = $1 AND enterprise_user_id = $2
		FOR UPDATE`, memberID, ownerID).Scan(&memberStatus, &memberDeletedAt, &currentVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEnterpriseMemberNotFound
		}
		return nil, err
	}
	if memberDeletedAt.Valid {
		return nil, service.ErrEnterpriseMemberNotFound
	}
	if currentVersion != expectedVersion {
		return nil, service.ErrEnterpriseMemberVersion
	}
	if memberStatus != service.EnterpriseMemberStatusActive {
		return nil, service.ErrEnterpriseMemberKeyNotAdoptable
	}

	var originalGroupID sql.NullInt64
	var existingMemberID sql.NullInt64
	var keyStatus string
	var keyDeletedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT group_id, member_id, status, deleted_at
		FROM api_keys
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, keyID, ownerID).Scan(&originalGroupID, &existingMemberID, &keyStatus, &keyDeletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEnterpriseMemberKeyNotAdoptable
		}
		return nil, err
	}
	if !originalGroupID.Valid || existingMemberID.Valid || keyDeletedAt.Valid || keyStatus != service.StatusAPIKeyActive {
		return nil, service.ErrEnterpriseMemberKeyNotAdoptable
	}

	var groupAuthorized bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM groups g
			WHERE g.id = $2 AND g.deleted_at IS NULL AND g.status = 'active' AND (
				(g.subscription_type = 'subscription' AND EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = $1 AND us.group_id = g.id AND us.deleted_at IS NULL
					  AND us.status = 'active' AND us.starts_at <= NOW() AND us.expires_at > NOW()
				)) OR
				(g.subscription_type <> 'subscription' AND (
					NOT g.is_exclusive OR EXISTS (
						SELECT 1 FROM user_allowed_groups uag WHERE uag.user_id = $1 AND uag.group_id = g.id
					)
				))
			)
		)`, ownerID, originalGroupID.Int64).Scan(&groupAuthorized); err != nil {
		return nil, err
	}
	if !groupAuthorized {
		return nil, service.ErrGroupNotAllowed.WithMetadata(map[string]string{"group_id": fmt.Sprintf("%d", originalGroupID.Int64)})
	}

	groupAdded := false
	var insertedGroupID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_group_bindings (member_id, group_id, sort_order, created_at, updated_at)
		SELECT $1, $2, COALESCE(MAX(sort_order), -1) + 1, NOW(), NOW()
		FROM enterprise_member_group_bindings
		WHERE member_id = $1
		ON CONFLICT (member_id, group_id) DO NOTHING
		RETURNING group_id`, memberID, originalGroupID.Int64).Scan(&insertedGroupID)
	if err == nil {
		groupAdded = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE api_keys
		SET member_id = $1, group_id = NULL, updated_at = NOW()
		WHERE id = $2 AND user_id = $3 AND member_id IS NULL AND group_id = $4`, memberID, keyID, ownerID, originalGroupID.Int64)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrEnterpriseMemberKeyNotAdoptable
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE enterprise_members
		SET version = version + 1, updated_at = NOW()
		WHERE id = $1 AND enterprise_user_id = $2 AND version = $3 AND deleted_at IS NULL`, memberID, ownerID, expectedVersion)
	if err != nil {
		return nil, err
	}
	affected, err = result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrEnterpriseMemberVersion
	}

	groupRows, err := tx.QueryContext(ctx, `
		SELECT group_id
		FROM enterprise_member_group_bindings
		WHERE member_id = $1
		ORDER BY sort_order, group_id`, memberID)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0)
	for groupRows.Next() {
		var groupID int64
		if err := groupRows.Scan(&groupID); err != nil {
			_ = groupRows.Close()
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	if err := groupRows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.EnterpriseMemberKeyAdoptionResult{
		KeyID: keyID, OriginalGroupID: originalGroupID.Int64, GroupAdded: groupAdded,
		GroupIDs: groupIDs, MemberVersion: expectedVersion + 1,
	}, nil
}

func (r *enterpriseMemberRepository) ListUsageRecords(ctx context.Context, ownerID, memberID int64, page, pageSize int) ([]service.EnterpriseMemberUsageRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("enterprise member repository sql db is nil")
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM usage_logs
		WHERE user_id = $1 AND member_id = $2`, ownerID, memberID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []service.EnterpriseMemberUsageRecord{}, 0, nil
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT ul.id, ul.request_id, ul.api_key_id, COALESCE(k.name, ''),
		       COALESCE(NULLIF(ul.requested_model, ''), ul.model),
		       ul.group_id, COALESCE(g.name, ''), ul.request_type,
		       ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
		       ul.actual_cost, ul.duration_ms, ul.first_token_ms,
		       COALESCE(ul.billing_mode, ''), COALESCE(ul.inbound_endpoint, ''),
		       ul.image_count, ul.video_count, ul.created_at
		FROM usage_logs ul
		LEFT JOIN api_keys k ON k.id = ul.api_key_id AND k.user_id = ul.user_id
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.user_id = $1 AND ul.member_id = $2
		ORDER BY ul.created_at DESC, ul.id DESC
		LIMIT $3 OFFSET $4`, ownerID, memberID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.EnterpriseMemberUsageRecord, 0, pageSize)
	for rows.Next() {
		var item service.EnterpriseMemberUsageRecord
		var groupID sql.NullInt64
		var requestType int16
		var durationMs, firstTokenMs sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.RequestID, &item.APIKeyID, &item.APIKeyName,
			&item.Model, &groupID, &item.GroupName, &requestType,
			&item.InputTokens, &item.OutputTokens, &item.CacheCreationTokens, &item.CacheReadTokens,
			&item.ActualCost, &durationMs, &firstTokenMs, &item.BillingMode, &item.InboundEndpoint,
			&item.ImageCount, &item.VideoCount, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
		}
		if durationMs.Valid {
			value := int(durationMs.Int64)
			item.DurationMs = &value
		}
		if firstTokenMs.Valid {
			value := int(firstTokenMs.Int64)
			item.FirstTokenMs = &value
		}
		item.RequestType = service.RequestTypeFromInt16(requestType).String()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *enterpriseMemberRepository) enrichMembers(ctx context.Context, members []service.EnterpriseMember) error {
	if len(members) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(members))
	byID := make(map[int64]int, len(members))
	for i := range members {
		ids = append(ids, members[i].ID)
		byID[members[i].ID] = i
	}
	bindings, err := r.client.EnterpriseMemberGroupBinding.Query().
		Where(enterprisemembergroupbinding.MemberIDIn(ids...)).
		Order(
			dbent.Asc(enterprisemembergroupbinding.FieldMemberID),
			dbent.Asc(enterprisemembergroupbinding.FieldSortOrder),
			dbent.Asc(enterprisemembergroupbinding.FieldGroupID),
		).
		All(ctx)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if idx, ok := byID[binding.MemberID]; ok {
			members[idx].GroupIDs = append(members[idx].GroupIDs, binding.GroupID)
		}
	}

	type memberKeyCount struct {
		MemberID int64 `json:"member_id"`
		Count    int64 `json:"count"`
	}
	var counts []memberKeyCount
	if err := r.client.APIKey.Query().
		Where(apikey.MemberIDIn(ids...), apikey.DeletedAtIsNil()).
		GroupBy(apikey.FieldMemberID).
		Aggregate(dbent.Count()).
		Scan(ctx, &counts); err != nil {
		return err
	}
	for _, count := range counts {
		if idx, ok := byID[count.MemberID]; ok {
			members[idx].KeyCount = count.Count
		}
	}
	archivedIDs := make([]int64, 0, len(members))
	for i := range members {
		if members[i].DeletedAt == nil {
			continue
		}
		members[i].CanPermanentlyDelete = true
		members[i].DeleteStrategy = service.EnterpriseMemberDeletionModeHardDelete
		archivedIDs = append(archivedIDs, members[i].ID)
	}
	historicalMemberIDs, err := enterpriseMembersWithHistoricalFacts(ctx, r.db, archivedIDs)
	if err != nil {
		return err
	}
	for memberID := range historicalMemberIDs {
		if idx, ok := byID[memberID]; ok {
			members[idx].DeleteStrategy = service.EnterpriseMemberDeletionModeTombstone
		}
	}
	rateRows, err := r.db.QueryContext(ctx, `
		SELECT member_id, usage_5h, usage_1d, usage_7d,
		       window_5h_start, window_1d_start, window_7d_start
		FROM enterprise_member_rate_limit_periods
		WHERE member_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rateRows.Close() }()
	for rateRows.Next() {
		var memberID int64
		var usage5h, usage1d, usage7d float64
		var window5h, window1d, window7d sql.NullTime
		if err := rateRows.Scan(&memberID, &usage5h, &usage1d, &usage7d, &window5h, &window1d, &window7d); err != nil {
			return err
		}
		idx, ok := byID[memberID]
		if !ok {
			continue
		}
		member := &members[idx]
		if window5h.Valid {
			member.Window5hStart = &window5h.Time
		}
		if window1d.Valid {
			member.Window1dStart = &window1d.Time
		}
		if window7d.Valid {
			member.Window7dStart = &window7d.Time
		}
		if !service.IsWindowExpired(member.Window5hStart, service.RateLimitWindow5h) {
			member.Usage5h = usage5h
		}
		if !service.IsWindowExpired(member.Window1dStart, service.RateLimitWindow1d) {
			member.Usage1d = usage1d
		}
		if !service.IsWindowExpired(member.Window7dStart, service.RateLimitWindow7d) {
			member.Usage7d = usage7d
		}
	}
	if err := rateRows.Err(); err != nil {
		return err
	}
	return nil
}

type enterpriseMemberSQLQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// enterpriseMembersWithHistoricalFacts is the single source of truth for the
// list deletion hint and the final transactional deletion strategy. Members
// without facts can be physically deleted; members with facts become hidden
// tombstones so billing and audit relationships remain intact.
func enterpriseMembersWithHistoricalFacts(ctx context.Context, queryer enterpriseMemberSQLQueryer, memberIDs []int64) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	if len(memberIDs) == 0 {
		return result, nil
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT candidate.member_id
		FROM unnest($1::bigint[]) AS candidate(member_id)
		WHERE EXISTS (SELECT 1 FROM api_keys WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM usage_logs WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM enterprise_member_import_usage_baselines WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM enterprise_member_budget_entries WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM enterprise_member_budget_reservations WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM enterprise_member_budget_periods WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM enterprise_member_rate_limit_periods WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM ops_error_logs WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM batch_image_jobs WHERE member_id = candidate.member_id)
			OR EXISTS (SELECT 1 FROM grok_media_tasks WHERE member_id = candidate.member_id)`, pq.Array(memberIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var memberID int64
		if err := rows.Scan(&memberID); err != nil {
			return nil, err
		}
		result[memberID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func createEnterpriseMemberBindings(ctx context.Context, client *dbent.Client, memberID int64, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	creates := make([]*dbent.EnterpriseMemberGroupBindingCreate, 0, len(groupIDs))
	now := time.Now()
	for sortOrder, groupID := range groupIDs {
		creates = append(creates, client.EnterpriseMemberGroupBinding.Create().
			SetMemberID(memberID).
			SetGroupID(groupID).
			SetSortOrder(sortOrder).
			SetCreatedAt(now).
			SetUpdatedAt(now))
	}
	return client.EnterpriseMemberGroupBinding.CreateBulk(creates...).Exec(ctx)
}

func enterpriseMemberEntityToService(row *dbent.EnterpriseMember) *service.EnterpriseMember {
	if row == nil {
		return nil
	}
	return &service.EnterpriseMember{
		ID:               row.ID,
		EnterpriseUserID: row.EnterpriseUserID,
		MemberCode:       row.MemberCode,
		Name:             row.Name,
		Status:           row.Status,
		MonthlyLimitUSD:  row.MonthlyLimitUsd,
		RateLimit5h:      row.RateLimit5h,
		RateLimit1d:      row.RateLimit1d,
		RateLimit7d:      row.RateLimit7d,
		Version:          row.Version,
		GroupIDs:         make([]int64, 0),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		DeletedAt:        row.DeletedAt,
	}
}

func (r *enterpriseMemberRepository) runInTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if fn == nil {
		return nil
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	if errors.Is(err, dbent.ErrTxStarted) {
		return fn(ctx, r.client)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}
