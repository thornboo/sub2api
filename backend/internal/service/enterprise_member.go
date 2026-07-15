package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrEnterpriseAccountRequired       = infraerrors.Forbidden("ENTERPRISE_ACCOUNT_REQUIRED", "enterprise account is required")
	ErrEnterpriseAccountDisabled       = infraerrors.Forbidden("ENTERPRISE_ACCOUNT_DISABLED", "enterprise account is disabled")
	ErrEnterpriseMemberNotFound        = infraerrors.NotFound("ENTERPRISE_MEMBER_NOT_FOUND", "enterprise member not found")
	ErrEnterpriseMemberConflict        = infraerrors.Conflict("ENTERPRISE_MEMBER_CONFLICT", "enterprise member already exists")
	ErrEnterpriseMemberVersion         = infraerrors.Conflict("ENTERPRISE_MEMBER_VERSION_CONFLICT", "enterprise member was modified; reload and retry")
	ErrEnterpriseMemberInvalid         = infraerrors.BadRequest("ENTERPRISE_MEMBER_INVALID", "enterprise member input is invalid")
	ErrEnterpriseMemberKeyNotAdoptable = infraerrors.Conflict("ENTERPRISE_MEMBER_KEY_NOT_ADOPTABLE", "api key is not eligible for enterprise member adoption")
)

const (
	EnterpriseMemberStatusActive   = "active"
	EnterpriseMemberStatusDisabled = "disabled"
	EnterpriseMemberBatchMaxSize   = 500
	// EnterpriseMemberMaxMonetaryValue stays below PostgreSQL NUMERIC(20,8)'s
	// twelve-integer-digit ceiling while remaining exactly inside the product's
	// accepted money/rate range after float64 conversion.
	EnterpriseMemberMaxMonetaryValue = 999_999_999_999.99

	EnterpriseMemberDeletionModeHardDelete = "hard_delete"
	EnterpriseMemberDeletionModeTombstone  = "tombstone"
)

var enterpriseMemberCodePattern = regexp.MustCompile("^[A-Za-z0-9._-]+$")

type EnterpriseMember struct {
	ID               int64      `json:"id"`
	EnterpriseUserID int64      `json:"enterprise_user_id"`
	MemberCode       string     `json:"member_code"`
	Name             string     `json:"name"`
	Status           string     `json:"status"`
	MonthlyLimitUSD  float64    `json:"monthly_limit_usd"`
	RateLimit5h      float64    `json:"rate_limit_5h"`
	RateLimit1d      float64    `json:"rate_limit_1d"`
	RateLimit7d      float64    `json:"rate_limit_7d"`
	Usage5h          float64    `json:"usage_5h"`
	Usage1d          float64    `json:"usage_1d"`
	Usage7d          float64    `json:"usage_7d"`
	Window5hStart    *time.Time `json:"window_5h_start,omitempty"`
	Window1dStart    *time.Time `json:"window_1d_start,omitempty"`
	Window7dStart    *time.Time `json:"window_7d_start,omitempty"`
	Version          int64      `json:"version"`
	GroupIDs         []int64    `json:"group_ids"`
	Groups           []Group    `json:"-"`
	KeyCount         int64      `json:"key_count"`
	DeleteStrategy   string     `json:"delete_strategy,omitempty"`
	// CanPermanentlyDelete is retained for rolling compatibility with older
	// frontends. Every archived member can now be removed from the owner's
	// workspace; DeleteStrategy describes how the server preserves evidence.
	CanPermanentlyDelete bool       `json:"can_permanently_delete"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `json:"deleted_at,omitempty"`
}

type EnterpriseMemberDeletionResult struct {
	Mode string `json:"mode"`
}

type EnterpriseMemberGroupBinding struct {
	MemberID  int64     `json:"member_id"`
	GroupID   int64     `json:"group_id"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateEnterpriseMemberInput struct {
	MemberCode      string  `json:"member_code"`
	Name            string  `json:"name"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	RateLimit5h     float64 `json:"rate_limit_5h"`
	RateLimit1d     float64 `json:"rate_limit_1d"`
	RateLimit7d     float64 `json:"rate_limit_7d"`
	MonthlyUsedUSD  float64 `json:"monthly_used_usd"`
	Usage5h         float64 `json:"usage_5h"`
	Usage1d         float64 `json:"usage_1d"`
	Usage7d         float64 `json:"usage_7d"`
	GroupIDs        []int64 `json:"group_ids"`
}

type EnterpriseMemberOpeningUsage struct {
	PeriodStart    time.Time
	MonthlyUsedUSD float64
	Usage5h        float64
	Usage1d        float64
	Usage7d        float64
	ActorUserID    int64
	IdempotencyKey string
	Note           string
}

func (o EnterpriseMemberOpeningUsage) HasUsage() bool {
	return o.MonthlyUsedUSD > 0 || o.Usage5h > 0 || o.Usage1d > 0 || o.Usage7d > 0
}

type UpdateEnterpriseMemberInput struct {
	ExpectedVersion int64    `json:"expected_version"`
	MemberCode      *string  `json:"member_code"`
	Name            *string  `json:"name"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
	RateLimit5h     *float64 `json:"rate_limit_5h"`
	RateLimit1d     *float64 `json:"rate_limit_1d"`
	RateLimit7d     *float64 `json:"rate_limit_7d"`
}

type ReplaceEnterpriseMemberGroupsInput struct {
	ExpectedVersion int64   `json:"expected_version"`
	GroupIDs        []int64 `json:"group_ids"`
}

type BatchEnterpriseMemberGroupMember struct {
	ID              int64 `json:"id"`
	ExpectedVersion int64 `json:"expected_version"`
}

type BatchReplaceEnterpriseMemberGroupsInput struct {
	Members  []BatchEnterpriseMemberGroupMember `json:"members"`
	GroupIDs []int64                            `json:"group_ids"`
	Mode     string                             `json:"mode"`
}

type BatchEnterpriseMemberGroupTarget struct {
	ID              int64
	ExpectedVersion int64
	GroupIDs        []int64
}

type BatchEnterpriseMemberGroupUpdate struct {
	ID        int64     `json:"id"`
	Version   int64     `json:"version"`
	GroupIDs  []int64   `json:"group_ids"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EnterpriseMemberBatchTarget struct {
	ID              int64 `json:"id"`
	ExpectedVersion int64 `json:"expected_version"`
}

type BatchUpdateEnterpriseMembersInput struct {
	Members         []EnterpriseMemberBatchTarget `json:"members"`
	MonthlyLimitUSD *float64                      `json:"monthly_limit_usd"`
	RateLimit5h     *float64                      `json:"rate_limit_5h"`
	RateLimit1d     *float64                      `json:"rate_limit_1d"`
	RateLimit7d     *float64                      `json:"rate_limit_7d"`
	Status          *string                       `json:"status"`
	GroupMode       string                        `json:"group_mode"`
	GroupIDs        []int64                       `json:"group_ids"`
}

type BatchEnterpriseMemberPolicyPatch struct {
	MonthlyLimitUSD *float64
	RateLimit5h     *float64
	RateLimit1d     *float64
	RateLimit7d     *float64
	Status          *string
	GroupMode       string
	GroupIDs        []int64
}

type BatchEnterpriseMemberUpdate struct {
	ID              int64     `json:"id"`
	Version         int64     `json:"version"`
	Status          string    `json:"status"`
	MonthlyLimitUSD float64   `json:"monthly_limit_usd"`
	RateLimit5h     float64   `json:"rate_limit_5h"`
	RateLimit1d     float64   `json:"rate_limit_1d"`
	RateLimit7d     float64   `json:"rate_limit_7d"`
	GroupIDs        []int64   `json:"group_ids"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AdoptEnterpriseMemberKeyInput struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type EnterpriseMemberKeyAdoptionResult struct {
	KeyID           int64   `json:"key_id"`
	OriginalGroupID int64   `json:"original_group_id"`
	GroupAdded      bool    `json:"group_added"`
	GroupIDs        []int64 `json:"group_ids"`
	MemberVersion   int64   `json:"member_version"`
}

type EnterpriseMemberUsageRecord struct {
	ID                  int64     `json:"id"`
	RequestID           string    `json:"request_id"`
	APIKeyID            int64     `json:"api_key_id"`
	APIKeyName          string    `json:"api_key_name"`
	Model               string    `json:"model"`
	GroupID             *int64    `json:"group_id,omitempty"`
	GroupName           string    `json:"group_name"`
	RequestType         string    `json:"request_type"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	ActualCost          float64   `json:"actual_cost"`
	DurationMs          *int      `json:"duration_ms,omitempty"`
	FirstTokenMs        *int      `json:"first_token_ms,omitempty"`
	BillingMode         string    `json:"billing_mode"`
	InboundEndpoint     string    `json:"inbound_endpoint"`
	ImageCount          int       `json:"image_count"`
	VideoCount          int       `json:"video_count"`
	CreatedAt           time.Time `json:"created_at"`
}

type EnterpriseMemberRepository interface {
	ListByOwner(ctx context.Context, ownerID int64, includeArchived bool) ([]EnterpriseMember, error)
	GetByOwnerAndID(ctx context.Context, ownerID, memberID int64, includeArchived bool) (*EnterpriseMember, error)
	Create(ctx context.Context, member *EnterpriseMember, groupIDs []int64, opening EnterpriseMemberOpeningUsage) error
	Update(ctx context.Context, member *EnterpriseMember, expectedVersion int64) error
	SetStatus(ctx context.Context, ownerID, memberID, expectedVersion int64, status string) (*EnterpriseMember, error)
	Archive(ctx context.Context, ownerID, memberID, expectedVersion int64) error
	Restore(ctx context.Context, ownerID, memberID, expectedVersion int64) (*EnterpriseMember, error)
	DeletePermanently(ctx context.Context, ownerID, memberID int64) (*EnterpriseMemberDeletionResult, error)
	ReplaceGroups(ctx context.Context, ownerID, memberID, expectedVersion int64, groupIDs []int64) (*EnterpriseMember, error)
	BatchReplaceGroups(ctx context.Context, ownerID int64, targets []BatchEnterpriseMemberGroupTarget) ([]BatchEnterpriseMemberGroupUpdate, error)
	BatchUpdate(ctx context.Context, ownerID int64, targets []EnterpriseMemberBatchTarget, patch BatchEnterpriseMemberPolicyPatch) ([]BatchEnterpriseMemberUpdate, error)
	ListKeys(ctx context.Context, ownerID, memberID int64) ([]APIKey, error)
	ListAdoptableKeys(ctx context.Context, ownerID int64) ([]APIKey, error)
	AdoptKey(ctx context.Context, ownerID, memberID, keyID, expectedVersion int64) (*EnterpriseMemberKeyAdoptionResult, error)
	ListUsageRecords(ctx context.Context, ownerID, memberID int64, page, pageSize int) ([]EnterpriseMemberUsageRecord, int64, error)
}

type EnterpriseMemberService struct {
	repo          EnterpriseMemberRepository
	userRepo      UserRepository
	apiKeyService *APIKeyService
}

func NewEnterpriseMemberService(
	repo EnterpriseMemberRepository,
	userRepo UserRepository,
	apiKeyService *APIKeyService,
) *EnterpriseMemberService {
	return &EnterpriseMemberService{
		repo:          repo,
		userRepo:      userRepo,
		apiKeyService: apiKeyService,
	}
}

func (s *EnterpriseMemberService) List(ctx context.Context, ownerID int64, includeArchived bool) ([]EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	return s.repo.ListByOwner(ctx, ownerID, includeArchived)
}

func (s *EnterpriseMemberService) Get(ctx context.Context, ownerID, memberID int64, includeArchived bool) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	return s.repo.GetByOwnerAndID(ctx, ownerID, memberID, includeArchived)
}

func (s *EnterpriseMemberService) Create(ctx context.Context, ownerID int64, input CreateEnterpriseMemberInput, idempotencyKey string) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	memberCode, name, err := normalizeEnterpriseMemberIdentity(input.MemberCode, input.Name)
	if err != nil {
		return nil, err
	}
	if err := validateEnterpriseSpendingLimits(input.MonthlyLimitUSD, input.RateLimit5h, input.RateLimit1d, input.RateLimit7d); err != nil {
		return nil, err
	}
	if err := validateEnterpriseUsageValues(input.MonthlyUsedUSD, input.Usage5h, input.Usage1d, input.Usage7d); err != nil {
		return nil, err
	}
	hasOpeningUsage := input.MonthlyUsedUSD > 0 || input.Usage5h > 0 || input.Usage1d > 0 || input.Usage7d > 0
	idempotencyKey, err = NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.validateAndNormalizeGroupIDs(ctx, ownerID, input.GroupIDs)
	if err != nil {
		return nil, err
	}
	status := EnterpriseMemberStatusDisabled
	if len(groupIDs) > 0 {
		status = EnterpriseMemberStatusActive
	}
	member := &EnterpriseMember{
		EnterpriseUserID: ownerID,
		MemberCode:       memberCode,
		Name:             name,
		Status:           status,
		MonthlyLimitUSD:  input.MonthlyLimitUSD,
		RateLimit5h:      input.RateLimit5h,
		RateLimit1d:      input.RateLimit1d,
		RateLimit7d:      input.RateLimit7d,
		Version:          1,
	}
	periodStart, _ := enterpriseMemberCurrentBudgetPeriod(time.Now())
	opening := EnterpriseMemberOpeningUsage{
		PeriodStart:    periodStart,
		MonthlyUsedUSD: input.MonthlyUsedUSD,
		Usage5h:        input.Usage5h,
		Usage1d:        input.Usage1d,
		Usage7d:        input.Usage7d,
		ActorUserID:    ownerID,
		IdempotencyKey: fmt.Sprintf("member-opening:%d:%s", ownerID, HashIdempotencyKey(idempotencyKey)),
		Note:           enterpriseMemberSystemUsageNote(hasOpeningUsage, "member creation"),
	}
	if err := s.repo.Create(ctx, member, groupIDs, opening); err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return member, nil
}

func (s *EnterpriseMemberService) Update(ctx context.Context, ownerID, memberID int64, input UpdateEnterpriseMemberInput) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "expected_version"})
	}
	member, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false)
	if err != nil {
		return nil, err
	}
	if input.MemberCode != nil {
		code, _, err := normalizeEnterpriseMemberIdentity(*input.MemberCode, member.Name)
		if err != nil {
			return nil, err
		}
		if code != member.MemberCode {
			return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "member_code", "reason": "immutable"})
		}
	}
	if input.Name != nil {
		_, name, err := normalizeEnterpriseMemberIdentity(member.MemberCode, *input.Name)
		if err != nil {
			return nil, err
		}
		member.Name = name
	}
	if input.MonthlyLimitUSD != nil {
		if err := validateEnterpriseSpendingLimit("monthly_limit_usd", *input.MonthlyLimitUSD); err != nil {
			return nil, err
		}
		member.MonthlyLimitUSD = *input.MonthlyLimitUSD
	}
	if input.RateLimit5h != nil {
		if err := validateEnterpriseSpendingLimit("rate_limit_5h", *input.RateLimit5h); err != nil {
			return nil, err
		}
		member.RateLimit5h = *input.RateLimit5h
	}
	if input.RateLimit1d != nil {
		if err := validateEnterpriseSpendingLimit("rate_limit_1d", *input.RateLimit1d); err != nil {
			return nil, err
		}
		member.RateLimit1d = *input.RateLimit1d
	}
	if input.RateLimit7d != nil {
		if err := validateEnterpriseSpendingLimit("rate_limit_7d", *input.RateLimit7d); err != nil {
			return nil, err
		}
		member.RateLimit7d = *input.RateLimit7d
	}
	if err := s.repo.Update(ctx, member, input.ExpectedVersion); err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return member, nil
}

func (s *EnterpriseMemberService) ReplaceGroups(ctx context.Context, ownerID, memberID int64, input ReplaceEnterpriseMemberGroupsInput) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "expected_version"})
	}
	groupIDs, err := s.validateAndNormalizeGroupIDs(ctx, ownerID, input.GroupIDs)
	if err != nil {
		return nil, err
	}
	member, err := s.repo.ReplaceGroups(ctx, ownerID, memberID, input.ExpectedVersion, groupIDs)
	if err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return member, nil
}

func (s *EnterpriseMemberService) BatchReplaceGroups(ctx context.Context, ownerID int64, input BatchReplaceEnterpriseMemberGroupsInput) ([]BatchEnterpriseMemberGroupUpdate, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if len(input.Members) == 0 || len(input.Members) > enterpriseMemberImportMaxRows || (input.Mode != "replace" && input.Mode != "append") {
		return nil, ErrEnterpriseMemberInvalid
	}
	groupIDs, err := s.validateAndNormalizeGroupIDs(ctx, ownerID, input.GroupIDs)
	if err != nil {
		return nil, err
	}
	currentByID := make(map[int64]EnterpriseMember)
	if input.Mode == "append" {
		current, listErr := s.repo.ListByOwner(ctx, ownerID, false)
		if listErr != nil {
			return nil, listErr
		}
		for i := range current {
			currentByID[current[i].ID] = current[i]
		}
	}
	targets := make([]BatchEnterpriseMemberGroupTarget, 0, len(input.Members))
	seenMembers := make(map[int64]struct{}, len(input.Members))
	for _, member := range input.Members {
		if member.ID <= 0 || member.ExpectedVersion <= 0 {
			return nil, ErrEnterpriseMemberInvalid
		}
		if _, duplicate := seenMembers[member.ID]; duplicate {
			return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "members", "reason": "duplicate"})
		}
		seenMembers[member.ID] = struct{}{}
		desired := append([]int64(nil), groupIDs...)
		if input.Mode == "append" {
			current, ok := currentByID[member.ID]
			if !ok || current.Version != member.ExpectedVersion {
				return nil, ErrEnterpriseMemberVersion
			}
			desired = appendUniqueEnterpriseMemberGroupIDs(current.GroupIDs, groupIDs)
		}
		targets = append(targets, BatchEnterpriseMemberGroupTarget{ID: member.ID, ExpectedVersion: member.ExpectedVersion, GroupIDs: desired})
	}
	updated, err := s.repo.BatchReplaceGroups(ctx, ownerID, targets)
	// Authorization writes are security-sensitive and transaction commit errors
	// can be ambiguous. Conservatively invalidate even when the repository
	// reports an error; an unnecessary eviction is safer than stale access.
	s.invalidateOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *EnterpriseMemberService) BatchUpdate(ctx context.Context, ownerID int64, input BatchUpdateEnterpriseMembersInput) ([]BatchEnterpriseMemberUpdate, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if err := validateEnterpriseMemberBatchTargets(input.Members); err != nil {
		return nil, err
	}
	groupMode := strings.TrimSpace(input.GroupMode)
	if groupMode == "" {
		groupMode = "keep"
	}
	if groupMode != "keep" && groupMode != "replace" && groupMode != "append" {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "group_mode"})
	}
	if groupMode == "keep" && len(input.GroupIDs) > 0 {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "group_ids", "reason": "requires_group_mode"})
	}
	if input.MonthlyLimitUSD == nil && input.RateLimit5h == nil && input.RateLimit1d == nil && input.RateLimit7d == nil && input.Status == nil &&
		(groupMode == "keep" || (groupMode == "append" && len(input.GroupIDs) == 0)) {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "changes"})
	}
	for field, value := range map[string]*float64{
		"monthly_limit_usd": input.MonthlyLimitUSD,
		"rate_limit_5h":     input.RateLimit5h,
		"rate_limit_1d":     input.RateLimit1d,
		"rate_limit_7d":     input.RateLimit7d,
	} {
		if value != nil {
			if err := validateEnterpriseSpendingLimit(field, *value); err != nil {
				return nil, err
			}
		}
	}
	if input.Status != nil && *input.Status != EnterpriseMemberStatusActive && *input.Status != EnterpriseMemberStatusDisabled {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "status"})
	}
	groupIDs := []int64(nil)
	if groupMode != "keep" {
		var err error
		groupIDs, err = s.validateAndNormalizeGroupIDs(ctx, ownerID, input.GroupIDs)
		if err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.BatchUpdate(ctx, ownerID, input.Members, BatchEnterpriseMemberPolicyPatch{
		MonthlyLimitUSD: input.MonthlyLimitUSD,
		RateLimit5h:     input.RateLimit5h,
		RateLimit1d:     input.RateLimit1d,
		RateLimit7d:     input.RateLimit7d,
		Status:          input.Status,
		GroupMode:       groupMode,
		GroupIDs:        groupIDs,
	})
	// Policy changes may affect authorization and the commit result can be
	// ambiguous, so evict owner snapshots even when the repository returns an error.
	s.invalidateOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *EnterpriseMemberService) EnsureEnterpriseOwner(ctx context.Context, ownerID int64) error {
	_, err := s.requireEnterpriseOwner(ctx, ownerID)
	return err
}

func validateEnterpriseMemberBatchTargets(targets []EnterpriseMemberBatchTarget) error {
	if len(targets) == 0 || len(targets) > EnterpriseMemberBatchMaxSize {
		return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "members"})
	}
	seen := make(map[int64]struct{}, len(targets))
	for _, target := range targets {
		if target.ID <= 0 || target.ExpectedVersion <= 0 {
			return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "members"})
		}
		if _, exists := seen[target.ID]; exists {
			return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "members", "reason": "duplicate"})
		}
		seen[target.ID] = struct{}{}
	}
	return nil
}

func appendUniqueEnterpriseMemberGroupIDs(existing, appended []int64) []int64 {
	out := append([]int64(nil), existing...)
	seen := make(map[int64]struct{}, len(existing)+len(appended))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	for _, id := range appended {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *EnterpriseMemberService) SetStatus(ctx context.Context, ownerID, memberID, expectedVersion int64, status string) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if expectedVersion <= 0 || (status != EnterpriseMemberStatusActive && status != EnterpriseMemberStatusDisabled) {
		return nil, ErrEnterpriseMemberInvalid
	}
	if status == EnterpriseMemberStatusActive {
		member, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false)
		if err != nil {
			return nil, err
		}
		if member.Version != expectedVersion {
			return nil, ErrEnterpriseMemberVersion
		}
		if len(member.GroupIDs) == 0 {
			return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "group_ids", "reason": "required_to_enable"})
		}
	}
	member, err := s.repo.SetStatus(ctx, ownerID, memberID, expectedVersion, status)
	if err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return member, nil
}

func (s *EnterpriseMemberService) Archive(ctx context.Context, ownerID, memberID, expectedVersion int64) error {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return err
	}
	if err := s.repo.Archive(ctx, ownerID, memberID, expectedVersion); err != nil {
		return err
	}
	s.invalidateOwner(ctx, ownerID)
	return nil
}

func (s *EnterpriseMemberService) Restore(ctx context.Context, ownerID, memberID, expectedVersion int64) (*EnterpriseMember, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if expectedVersion <= 0 {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "expected_version"})
	}
	member, err := s.repo.Restore(ctx, ownerID, memberID, expectedVersion)
	if err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return member, nil
}

func (s *EnterpriseMemberService) DeletePermanently(ctx context.Context, ownerID, memberID int64) (*EnterpriseMemberDeletionResult, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	result, err := s.repo.DeletePermanently(ctx, ownerID, memberID)
	if err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return result, nil
}

func (s *EnterpriseMemberService) ListKeys(ctx context.Context, ownerID, memberID int64) ([]APIKey, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, true); err != nil {
		return nil, err
	}
	return s.repo.ListKeys(ctx, ownerID, memberID)
}

func (s *EnterpriseMemberService) ListAdoptableKeys(ctx context.Context, ownerID, memberID int64) ([]APIKey, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	member, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false)
	if err != nil {
		return nil, err
	}
	if member.Status != EnterpriseMemberStatusActive {
		return nil, infraerrors.Forbidden("ENTERPRISE_MEMBER_DISABLED", "enterprise member is disabled")
	}
	availableGroups, err := s.apiKeyService.GetAvailableGroups(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(availableGroups))
	for i := range availableGroups {
		allowed[availableGroups[i].ID] = struct{}{}
	}
	keys, err := s.repo.ListAdoptableKeys(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]APIKey, 0, len(keys))
	for i := range keys {
		if keys[i].GroupID == nil {
			continue
		}
		if _, ok := allowed[*keys[i].GroupID]; ok {
			out = append(out, keys[i])
		}
	}
	return out, nil
}

func (s *EnterpriseMemberService) AdoptKey(ctx context.Context, ownerID, memberID, keyID int64, input AdoptEnterpriseMemberKeyInput) (*EnterpriseMemberKeyAdoptionResult, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if keyID <= 0 || input.ExpectedVersion <= 0 {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "expected_version"})
	}
	member, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false)
	if err != nil {
		return nil, err
	}
	if member.Status != EnterpriseMemberStatusActive {
		return nil, infraerrors.Forbidden("ENTERPRISE_MEMBER_DISABLED", "enterprise member is disabled")
	}
	result, err := s.repo.AdoptKey(ctx, ownerID, memberID, keyID, input.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	s.invalidateOwner(ctx, ownerID)
	return result, nil
}

func (s *EnterpriseMemberService) ListUsageRecords(ctx context.Context, ownerID, memberID int64, page, pageSize int) ([]EnterpriseMemberUsageRecord, int64, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, 0, err
	}
	if _, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, true); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return s.repo.ListUsageRecords(ctx, ownerID, memberID, page, pageSize)
}

func (s *EnterpriseMemberService) CreateKey(ctx context.Context, ownerID, memberID int64, input CreateAPIKeyRequest) (*APIKey, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	member, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false)
	if err != nil {
		return nil, err
	}
	if member.Status != EnterpriseMemberStatusActive {
		return nil, infraerrors.Forbidden("ENTERPRISE_MEMBER_DISABLED", "enterprise member is disabled")
	}
	input.GroupID = nil
	input.MemberID = &memberID
	return s.apiKeyService.Create(ctx, ownerID, input)
}

func (s *EnterpriseMemberService) UpdateKey(ctx context.Context, ownerID, memberID, keyID int64, input UpdateAPIKeyRequest) (*APIKey, error) {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false); err != nil {
		return nil, err
	}
	if input.GroupID != nil {
		return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "group_id"})
	}
	keys, err := s.repo.ListKeys(ctx, ownerID, memberID)
	if err != nil {
		return nil, err
	}
	found := false
	for i := range keys {
		if keys[i].ID == keyID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrAPIKeyNotFound
	}
	return s.apiKeyService.UpdateEnterpriseMemberKey(ctx, keyID, ownerID, memberID, input)
}

func (s *EnterpriseMemberService) DeleteKey(ctx context.Context, ownerID, memberID, keyID int64) error {
	if _, err := s.requireEnterpriseOwner(ctx, ownerID); err != nil {
		return err
	}
	if _, err := s.repo.GetByOwnerAndID(ctx, ownerID, memberID, false); err != nil {
		return err
	}
	keys, err := s.repo.ListKeys(ctx, ownerID, memberID)
	if err != nil {
		return err
	}
	found := false
	for i := range keys {
		if keys[i].ID == keyID {
			found = true
			break
		}
	}
	if !found {
		return ErrAPIKeyNotFound
	}
	return s.apiKeyService.DeleteEnterpriseMemberKey(ctx, keyID, ownerID, memberID)
}

func (s *EnterpriseMemberService) requireEnterpriseOwner(ctx context.Context, ownerID int64) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if user.Role != RoleUser || user.AccountType != UserAccountTypeEnterprise {
		return nil, ErrEnterpriseAccountRequired
	}
	if user.EnterpriseDisabledAt != nil || user.Status != StatusActive {
		return nil, ErrEnterpriseAccountDisabled
	}
	return user, nil
}

func (s *EnterpriseMemberService) validateAndNormalizeGroupIDs(ctx context.Context, ownerID int64, requested []int64) ([]int64, error) {
	available, err := s.apiKeyService.GetAvailableGroups(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(available))
	for i := range available {
		allowed[available[i].ID] = struct{}{}
	}
	out := make([]int64, 0, len(requested))
	seen := make(map[int64]struct{}, len(requested))
	for _, id := range requested {
		if id <= 0 {
			return nil, ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "group_ids"})
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		if _, ok := allowed[id]; !ok {
			return nil, ErrGroupNotAllowed.WithMetadata(map[string]string{"group_id": fmt.Sprintf("%d", id)})
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (s *EnterpriseMemberService) invalidateOwner(ctx context.Context, ownerID int64) {
	if s.apiKeyService != nil {
		invalidationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		s.apiKeyService.InvalidateAuthCacheByUserID(invalidationCtx, ownerID)
	}
}

func normalizeEnterpriseMemberIdentity(memberCode, name string) (string, string, error) {
	memberCode = strings.TrimSpace(memberCode)
	name = strings.TrimSpace(name)
	if memberCode == "" || len(memberCode) > 100 || !enterpriseMemberCodePattern.MatchString(memberCode) {
		return "", "", ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "member_code"})
	}
	if name == "" || len([]rune(name)) > 100 {
		return "", "", ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "name"})
	}
	return memberCode, name, nil
}

func validateEnterpriseSpendingLimit(field string, limit float64) error {
	if math.IsNaN(limit) || math.IsInf(limit, 0) || limit < 0 || limit > EnterpriseMemberMaxMonetaryValue {
		return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": field, "reason": "out_of_range"})
	}
	return nil
}

func validateEnterpriseSpendingLimits(monthly, limit5h, limit1d, limit7d float64) error {
	for field, value := range map[string]float64{
		"monthly_limit_usd": monthly,
		"rate_limit_5h":     limit5h,
		"rate_limit_1d":     limit1d,
		"rate_limit_7d":     limit7d,
	} {
		if err := validateEnterpriseSpendingLimit(field, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *EnterpriseMember) HasSpendingLimits() bool {
	return m != nil && (m.MonthlyLimitUSD > 0 || m.RateLimit5h > 0 || m.RateLimit1d > 0 || m.RateLimit7d > 0)
}
