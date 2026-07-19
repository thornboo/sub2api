package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrEnterpriseMemberBudgetExceeded         = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_BUDGET_EXCEEDED", "enterprise member monthly budget is exhausted")
	ErrEnterpriseMemberRateLimit5hExceeded    = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_5H_EXCEEDED", "enterprise member 5-hour spending limit is exhausted")
	ErrEnterpriseMemberRateLimit1dExceeded    = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_1D_EXCEEDED", "enterprise member daily spending limit is exhausted")
	ErrEnterpriseMemberRateLimit7dExceeded    = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_7D_EXCEEDED", "enterprise member 7-day spending limit is exhausted")
	ErrEnterpriseMemberAsyncBudgetUnavailable = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE", "available enterprise member budget is insufficient for this asynchronous task after accounting for active task holds and this task's estimated cost")
	ErrEnterpriseMemberBudgetUnbounded        = infraerrors.BadRequest("ENTERPRISE_MEMBER_BUDGET_UNBOUNDED_REQUEST", "request cost cannot be bounded for the enterprise member budget")
	ErrEnterpriseMemberBudgetConflict         = infraerrors.Conflict("ENTERPRISE_MEMBER_BUDGET_REQUEST_CONFLICT", "member budget request id was reused with different parameters")
	ErrEnterpriseMemberBudgetReceiptNotFound  = infraerrors.NotFound("ENTERPRISE_MEMBER_BUDGET_RECEIPT_NOT_FOUND", "enterprise member budget receipt not found")
)

// EnterpriseMemberBudgetTimezone is the authoritative calendar timezone for member budgets and import openings.
const EnterpriseMemberBudgetTimezone = "Asia/Shanghai"
const enterpriseMemberBudgetTimezone = EnterpriseMemberBudgetTimezone

const enterpriseMemberUsageAuditNote = "usage values updated by %s"

const (
	EnterpriseMemberReceiptKindLegacy     = "legacy"
	EnterpriseMemberReceiptKindSync       = "sync"
	EnterpriseMemberReceiptKindAsyncImage = "async_image"
	EnterpriseMemberReceiptKindAsyncVideo = "async_video"
	EnterpriseMemberReceiptKindBatchImage = "batch_image"

	EnterpriseMemberAsyncTaskPhaseQueued    = "queued"
	EnterpriseMemberAsyncTaskPhaseExecuting = "executing"
)

type EnterpriseMemberBudgetReservation struct {
	ID          int64
	RequestID   string
	MemberID    int64
	GroupID     *int64
	PayloadHash string
	PeriodStart time.Time
	ReservedUSD float64
	ActualUSD   float64
	Status      string
	ReceiptKind string
	TaskID      string
	TaskPhase   string
	UsageLogID  *int64
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

type EnterpriseMemberAmbiguousReceipt struct {
	ID                int64      `json:"id"`
	RequestID         string     `json:"request_id"`
	EnterpriseUserID  int64      `json:"enterprise_user_id"`
	MemberID          int64      `json:"member_id"`
	MemberCode        string     `json:"member_code"`
	MemberName        string     `json:"member_name"`
	GroupID           *int64     `json:"group_id,omitempty"`
	PeriodStart       time.Time  `json:"period_start"`
	ReservedUSD       float64    `json:"reserved_usd"`
	ReceiptKind       string     `json:"receipt_kind"`
	TaskID            string     `json:"task_id,omitempty"`
	TaskPhase         string     `json:"task_phase,omitempty"`
	OutcomeReason     string     `json:"outcome_reason"`
	ReconcileAttempts int        `json:"reconcile_attempts"`
	LastReconcileAt   *time.Time `json:"last_reconcile_at,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type EnterpriseMemberAmbiguousReceiptResolution struct {
	Decision                  string `json:"decision"`
	ExpectedReconcileAttempts int    `json:"expected_reconcile_attempts"`
	Reason                    string `json:"reason"`
}

const (
	EnterpriseMemberReceiptDecisionRelease = "release"
)

type EnterpriseMemberBudgetSummary struct {
	MemberID                  int64                      `json:"member_id"`
	PeriodStart               time.Time                  `json:"period_start"`
	PeriodEnd                 time.Time                  `json:"period_end"`
	Timezone                  string                     `json:"timezone"`
	LimitUSD                  float64                    `json:"limit_usd"`
	UsedUSD                   float64                    `json:"used_usd"`
	ReservedUSD               float64                    `json:"reserved_usd"`
	RemainingUSD              float64                    `json:"remaining_usd"`
	RequestCount              int64                      `json:"request_count"`
	InputTokens               int64                      `json:"input_tokens"`
	OutputTokens              int64                      `json:"output_tokens"`
	MigrationBilledUSD        float64                    `json:"migration_billed_usd"`
	MigrationTotalTokens      EnterpriseMemberTokenCount `json:"migration_total_tokens"`
	MigrationInputTokens      EnterpriseMemberTokenCount `json:"migration_input_tokens"`
	MigrationOutputTokens     EnterpriseMemberTokenCount `json:"migration_output_tokens"`
	MigrationCacheTokens      EnterpriseMemberTokenCount `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens EnterpriseMemberTokenCount `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  EnterpriseMemberTokenCount `json:"migration_cache_read_tokens"`
	RateLimit5h               float64                    `json:"rate_limit_5h"`
	RateLimit1d               float64                    `json:"rate_limit_1d"`
	RateLimit7d               float64                    `json:"rate_limit_7d"`
	Usage5h                   float64                    `json:"usage_5h"`
	Usage1d                   float64                    `json:"usage_1d"`
	Usage7d                   float64                    `json:"usage_7d"`
	Reset5hAt                 *time.Time                 `json:"reset_5h_at,omitempty"`
	Reset1dAt                 *time.Time                 `json:"reset_1d_at,omitempty"`
	Reset7dAt                 *time.Time                 `json:"reset_7d_at,omitempty"`
}

type EnterpriseMemberBudgetEntry struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	RequestID   *string   `json:"request_id,omitempty"`
	AmountUSD   float64   `json:"amount_usd"`
	UsageLogID  *int64    `json:"usage_log_id,omitempty"`
	ActorUserID *int64    `json:"actor_user_id,omitempty"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type EnterpriseMemberUsageTrendPoint struct {
	Date         string  `json:"date"`
	RequestCount int64   `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	ActualCost   float64 `json:"actual_cost"`
}

type EnterpriseMemberUsageBreakdown struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	RequestCount int64   `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	ActualCost   float64 `json:"actual_cost"`
}

type EnterpriseMemberUsageAnalytics struct {
	Start  time.Time                         `json:"start"`
	End    time.Time                         `json:"end"`
	Trend  []EnterpriseMemberUsageTrendPoint `json:"trend"`
	Models []EnterpriseMemberUsageBreakdown  `json:"models"`
	Groups []EnterpriseMemberUsageBreakdown  `json:"groups"`
}

type EnterpriseMemberOwnerUsageItem struct {
	MemberID                  int64                      `json:"member_id"`
	MemberCode                string                     `json:"member_code"`
	MemberName                string                     `json:"member_name"`
	Status                    string                     `json:"status"`
	LimitUSD                  float64                    `json:"limit_usd"`
	UsedUSD                   float64                    `json:"used_usd"`
	ReservedUSD               float64                    `json:"reserved_usd"`
	RemainingUSD              float64                    `json:"remaining_usd"`
	RequestCount              int64                      `json:"request_count"`
	InputTokens               int64                      `json:"input_tokens"`
	OutputTokens              int64                      `json:"output_tokens"`
	MigrationBilledUSD        float64                    `json:"migration_billed_usd"`
	MigrationTotalTokens      EnterpriseMemberTokenCount `json:"migration_total_tokens"`
	MigrationInputTokens      EnterpriseMemberTokenCount `json:"migration_input_tokens"`
	MigrationOutputTokens     EnterpriseMemberTokenCount `json:"migration_output_tokens"`
	MigrationCacheTokens      EnterpriseMemberTokenCount `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens EnterpriseMemberTokenCount `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  EnterpriseMemberTokenCount `json:"migration_cache_read_tokens"`
}

type EnterpriseMemberOwnerUsageSummary struct {
	PeriodStart               time.Time                        `json:"period_start"`
	PeriodEnd                 time.Time                        `json:"period_end"`
	Timezone                  string                           `json:"timezone"`
	UsedUSD                   float64                          `json:"used_usd"`
	ReservedUSD               float64                          `json:"reserved_usd"`
	RequestCount              int64                            `json:"request_count"`
	InputTokens               int64                            `json:"input_tokens"`
	OutputTokens              int64                            `json:"output_tokens"`
	MigrationBilledUSD        float64                          `json:"migration_billed_usd"`
	MigrationTotalTokens      EnterpriseMemberTokenCount       `json:"migration_total_tokens"`
	MigrationInputTokens      EnterpriseMemberTokenCount       `json:"migration_input_tokens"`
	MigrationOutputTokens     EnterpriseMemberTokenCount       `json:"migration_output_tokens"`
	MigrationCacheTokens      EnterpriseMemberTokenCount       `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens EnterpriseMemberTokenCount       `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  EnterpriseMemberTokenCount       `json:"migration_cache_read_tokens"`
	Members                   []EnterpriseMemberOwnerUsageItem `json:"members"`
}

type EnterpriseMemberBudgetRepository interface {
	Reserve(ctx context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error)
	GetReservation(ctx context.Context, requestID string) (*EnterpriseMemberBudgetReservation, error)
	Release(ctx context.Context, requestID string) error
	MarkAmbiguous(ctx context.Context, requestID, outcomeReason string) error
	GetPeriod(ctx context.Context, memberID int64, periodStart time.Time) (usedUSD, reservedUSD float64, err error)
	GetSummary(ctx context.Context, memberID int64, periodStart, periodEnd time.Time) (*EnterpriseMemberBudgetSummary, error)
	ListEntries(ctx context.Context, memberID int64, periodStart time.Time, limit, offset int) ([]EnterpriseMemberBudgetEntry, int64, error)
	CreateAdjustment(ctx context.Context, memberID int64, periodStart time.Time, amount float64, actorUserID int64, idempotencyKey, note string) error
	SetUsage(ctx context.Context, ownerID, memberID int64, periodStart time.Time, monthlyUsed, usage5h, usage1d, usage7d float64, actorUserID int64, idempotencyKey, note string) error
	BatchAdjustUsage(ctx context.Context, ownerID int64, periodStart time.Time, targets []EnterpriseMemberBatchTarget, delta EnterpriseMemberUsageDelta, actorUserID int64, idempotencyKey, note string) ([]BatchEnterpriseMemberUsageUpdate, error)
	GetUsageAnalytics(ctx context.Context, memberID int64, start, end time.Time) (*EnterpriseMemberUsageAnalytics, error)
	GetOwnerUsageSummary(ctx context.Context, ownerID int64, periodStart, periodEnd time.Time) (*EnterpriseMemberOwnerUsageSummary, error)
	GetOwnerUsageTrend(ctx context.Context, ownerID int64, start, end time.Time) ([]EnterpriseMemberUsageTrendPoint, error)
	RecoverExpired(ctx context.Context, limit int) (int, error)
	ReconcilePeriods(ctx context.Context, limit int) (EnterpriseMemberBudgetReconciliationResult, error)
	ListAmbiguousReceipts(ctx context.Context, limit, offset int) ([]EnterpriseMemberAmbiguousReceipt, int64, error)
	ResolveAmbiguousReceipt(ctx context.Context, receiptID int64, input EnterpriseMemberAmbiguousReceiptResolution, actorUserID int64) (*EnterpriseMemberAmbiguousReceipt, error)
}

type EnterpriseMemberTypedBudgetRepository interface {
	ReserveWithKind(ctx context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, receiptKind string, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error)
}

type EnterpriseMemberAsyncTaskRepository interface {
	AttachAsyncTask(ctx context.Context, requestID, taskID string, expiresAt time.Time) error
	MarkAsyncTaskExecuting(ctx context.Context, requestID, taskID string) error
}

type EnterpriseMemberAsyncTaskReleaseRepository interface {
	ReleaseAsyncTask(ctx context.Context, requestID, taskID string) (*EnterpriseMemberBudgetReservation, error)
	MarkAsyncTaskAmbiguous(ctx context.Context, requestID, taskID, outcomeReason string) (*EnterpriseMemberBudgetReservation, error)
}

type EnterpriseMemberAsyncTaskLookupRepository interface {
	GetReservationByTaskID(ctx context.Context, taskID string) (*EnterpriseMemberBudgetReservation, error)
}

type EnterpriseMemberUsageAdjustmentInput struct {
	MonthlyUsedUSD float64 `json:"monthly_used_usd"`
	Usage5h        float64 `json:"usage_5h"`
	Usage1d        float64 `json:"usage_1d"`
	Usage7d        float64 `json:"usage_7d"`
	Note           string  `json:"note"`
}

type EnterpriseMemberUsageDelta struct {
	MonthlyUsedUSD float64 `json:"monthly_used_delta"`
	Usage5h        float64 `json:"usage_5h_delta"`
	Usage1d        float64 `json:"usage_1d_delta"`
	Usage7d        float64 `json:"usage_7d_delta"`
}

type BatchAdjustEnterpriseMemberUsageInput struct {
	Members []EnterpriseMemberBatchTarget `json:"members"`
	EnterpriseMemberUsageDelta
}

type BatchEnterpriseMemberUsageUpdate struct {
	ID             int64   `json:"id"`
	MonthlyUsedUSD float64 `json:"monthly_used_usd"`
	Usage5h        float64 `json:"usage_5h"`
	Usage1d        float64 `json:"usage_1d"`
	Usage7d        float64 `json:"usage_7d"`
}

type EnterpriseMemberBudgetReconciliationResult struct {
	PeriodsChecked        int `json:"periods_checked"`
	EvidenceLinksRepaired int `json:"evidence_links_repaired"`
	MissingEntriesCreated int `json:"missing_entries_created"`
	ProjectionsRebuilt    int `json:"projections_rebuilt"`
}

type EnterpriseMemberBudgetEstimateInput struct {
	RequestID      string
	APIKey         *APIKey
	RequestedModel string
	Method         string
	Endpoint       string
	ContentType    string
	Body           []byte
}

const enterpriseMemberMaxOutputTokensUpperBound = 1_000_000
const enterpriseMemberMaxInputTokensUpperBound = 2_000_000
const enterpriseMemberMaxRequestCountUpperBound = 1_024

type enterpriseMemberBudgetRequestShape struct {
	Model           string `json:"model"`
	MaxTokens       int    `json:"max_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	N               int    `json:"n"`
	Duration        int    `json:"duration"`
}

type EnterpriseMemberBudgetService struct {
	repo              EnterpriseMemberBudgetRepository
	pricingResolver   *ModelPricingResolver
	userGroupRateRepo UserGroupRateRepository
	accountRepo       EnterpriseMemberBudgetAccountRepository
}

type EnterpriseMemberBudgetAccountRepository interface {
	ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error)
}

func NewEnterpriseMemberBudgetService(repo EnterpriseMemberBudgetRepository, pricingResolver *ModelPricingResolver, rateRepo UserGroupRateRepository) *EnterpriseMemberBudgetService {
	return &EnterpriseMemberBudgetService{repo: repo, pricingResolver: pricingResolver, userGroupRateRepo: rateRepo}
}

func ProvideEnterpriseMemberBudgetService(repo EnterpriseMemberBudgetRepository, pricingResolver *ModelPricingResolver, rateRepo UserGroupRateRepository, accountRepo AccountRepository) *EnterpriseMemberBudgetService {
	service := NewEnterpriseMemberBudgetService(repo, pricingResolver, rateRepo)
	service.accountRepo = accountRepo
	return service
}

func (s *EnterpriseMemberBudgetService) GetSummary(ctx context.Context, memberID int64) (*EnterpriseMemberBudgetSummary, error) {
	start, end := enterpriseMemberCurrentBudgetPeriod(time.Now())
	return s.repo.GetSummary(ctx, memberID, start, end)
}

func (s *EnterpriseMemberBudgetService) ListEntries(ctx context.Context, memberID int64, page, pageSize int) ([]EnterpriseMemberBudgetEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	start, _ := enterpriseMemberCurrentBudgetPeriod(time.Now())
	return s.repo.ListEntries(ctx, memberID, start, pageSize, (page-1)*pageSize)
}

func (s *EnterpriseMemberBudgetService) ListAmbiguousReceipts(ctx context.Context, page, pageSize int) ([]EnterpriseMemberAmbiguousReceipt, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, ErrEnterpriseMemberBudgetReceiptNotFound
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
	return s.repo.ListAmbiguousReceipts(ctx, pageSize, (page-1)*pageSize)
}

func (s *EnterpriseMemberBudgetService) ResolveAmbiguousReceipt(ctx context.Context, receiptID int64, input EnterpriseMemberAmbiguousReceiptResolution, actorUserID int64) (*EnterpriseMemberAmbiguousReceipt, error) {
	if s == nil || s.repo == nil || receiptID <= 0 {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	input.Decision = strings.ToLower(strings.TrimSpace(input.Decision))
	input.Reason = strings.TrimSpace(input.Reason)
	// A manual decision may only release a hold after an administrator has
	// proved the upstream request did not become billable. Successful outcomes
	// must flow through UsageBilling so enterprise balance, key quota, usage log,
	// and member budget settle atomically instead of creating split accounting.
	if input.Decision != EnterpriseMemberReceiptDecisionRelease {
		return nil, ErrEnterpriseMemberBudgetConflict.WithMetadata(map[string]string{"field": "decision"})
	}
	if input.ExpectedReconcileAttempts < 0 {
		return nil, ErrEnterpriseMemberBudgetConflict.WithMetadata(map[string]string{"field": "expected_reconcile_attempts"})
	}
	if input.Reason == "" || len(input.Reason) > 500 {
		return nil, ErrEnterpriseMemberBudgetConflict.WithMetadata(map[string]string{"field": "reason"})
	}
	return s.repo.ResolveAmbiguousReceipt(ctx, receiptID, input, actorUserID)
}

func (s *EnterpriseMemberBudgetService) CreateAdjustment(ctx context.Context, memberID, actorUserID int64, amount float64, idempotencyKey, note string) error {
	if amount == 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.Abs(amount) > 1_000_000 {
		return ErrEnterpriseMemberInvalid
	}
	note = strings.TrimSpace(note)
	if note == "" || len(note) > 1000 {
		return ErrEnterpriseMemberInvalid
	}
	idempotencyKey, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return err
	}
	start, _ := enterpriseMemberCurrentBudgetPeriod(time.Now())
	ledgerKey := fmt.Sprintf("manual:%d:%s", memberID, HashIdempotencyKey(idempotencyKey))
	return s.repo.CreateAdjustment(ctx, memberID, start, amount, actorUserID, ledgerKey, note)
}

func (s *EnterpriseMemberBudgetService) SetUsage(ctx context.Context, ownerID, memberID int64, input EnterpriseMemberUsageAdjustmentInput, idempotencyKey string) error {
	if err := validateEnterpriseUsageValues(input.MonthlyUsedUSD, input.Usage5h, input.Usage1d, input.Usage7d); err != nil {
		return err
	}
	note := strings.TrimSpace(input.Note)
	if len(note) > 1000 {
		return ErrEnterpriseMemberInvalid
	}
	if note == "" {
		note = enterpriseMemberSystemUsageNote(true, "member editor")
	}
	idempotencyKey, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return err
	}
	start, _ := enterpriseMemberCurrentBudgetPeriod(time.Now())
	ledgerKey := fmt.Sprintf("usage-adjustment:%d:%s", memberID, HashIdempotencyKey(idempotencyKey))
	return s.repo.SetUsage(ctx, ownerID, memberID, start, input.MonthlyUsedUSD, input.Usage5h, input.Usage1d, input.Usage7d, ownerID, ledgerKey, note)
}

func (s *EnterpriseMemberBudgetService) BatchAdjustUsage(ctx context.Context, ownerID int64, input BatchAdjustEnterpriseMemberUsageInput, idempotencyKey string) ([]BatchEnterpriseMemberUsageUpdate, error) {
	if err := validateEnterpriseMemberBatchTargets(input.Members); err != nil {
		return nil, err
	}
	delta := input.EnterpriseMemberUsageDelta
	if err := validateEnterpriseUsageDeltas(delta.MonthlyUsedUSD, delta.Usage5h, delta.Usage1d, delta.Usage7d); err != nil {
		return nil, err
	}
	idempotencyKey, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	if idempotencyKey == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	start, _ := enterpriseMemberCurrentBudgetPeriod(time.Now())
	ledgerKey := fmt.Sprintf("usage-batch:%d:%s", ownerID, HashIdempotencyKey(idempotencyKey))
	return s.repo.BatchAdjustUsage(ctx, ownerID, start, input.Members, delta, ownerID, ledgerKey, enterpriseMemberSystemUsageNote(true, "batch member editor"))
}

func enterpriseMemberSystemUsageNote(hasUsage bool, source string) string {
	if !hasUsage {
		return ""
	}
	return fmt.Sprintf(enterpriseMemberUsageAuditNote, source)
}

func validateEnterpriseUsageValues(values ...float64) error {
	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || value > EnterpriseMemberMaxMonetaryValue {
			return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "usage", "reason": "out_of_range"})
		}
	}
	return nil
}

func validateEnterpriseUsageDeltas(values ...float64) error {
	hasChange := false
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > EnterpriseMemberMaxMonetaryValue {
			return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "usage_delta", "reason": "out_of_range"})
		}
		if math.Abs(value) > 1e-8 {
			hasChange = true
		}
	}
	if !hasChange {
		return ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{"field": "changes"})
	}
	return nil
}

func (s *EnterpriseMemberBudgetService) GetUsageAnalytics(ctx context.Context, memberID int64, days int) (*EnterpriseMemberUsageAnalytics, error) {
	if days < 1 || days > 365 {
		days = 30
	}
	location, err := time.LoadLocation(enterpriseMemberBudgetTimezone)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(location)
	endLocal := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	startLocal := endLocal.AddDate(0, 0, -days)
	return s.repo.GetUsageAnalytics(ctx, memberID, startLocal.UTC(), endLocal.UTC())
}

func (s *EnterpriseMemberBudgetService) GetOwnerUsageSummary(ctx context.Context, ownerID int64) (*EnterpriseMemberOwnerUsageSummary, error) {
	start, end := enterpriseMemberCurrentBudgetPeriod(time.Now())
	return s.repo.GetOwnerUsageSummary(ctx, ownerID, start, end)
}

func (s *EnterpriseMemberBudgetService) GetOwnerUsageTrend(ctx context.Context, ownerID int64, days int) ([]EnterpriseMemberUsageTrendPoint, time.Time, time.Time, error) {
	start, end, err := enterpriseMemberUsageRange(days)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	trend, err := s.repo.GetOwnerUsageTrend(ctx, ownerID, start, end)
	return trend, start, end, err
}

func enterpriseMemberUsageRange(days int) (time.Time, time.Time, error) {
	if days < 1 || days > 365 {
		days = 30
	}
	location, err := time.LoadLocation(enterpriseMemberBudgetTimezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	now := time.Now().In(location)
	endLocal := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
	return endLocal.AddDate(0, 0, -days).UTC(), endLocal.UTC(), nil
}

func enterpriseMemberCurrentBudgetPeriod(now time.Time) (time.Time, time.Time) {
	return EnterpriseMemberCurrentBudgetPeriod(now)
}

// EnterpriseMemberCurrentBudgetPeriod returns the containing calendar-month boundaries in the authoritative budget timezone.
func EnterpriseMemberCurrentBudgetPeriod(now time.Time) (time.Time, time.Time) {
	location, err := time.LoadLocation(enterpriseMemberBudgetTimezone)
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}

func (s *EnterpriseMemberBudgetService) Reserve(ctx context.Context, input EnterpriseMemberBudgetEstimateInput) (*EnterpriseMemberBudgetReservation, error) {
	if input.APIKey == nil || input.APIKey.Member == nil || input.APIKey.MemberID == nil {
		return nil, nil
	}
	if !enterpriseMemberEndpointIsBillable(input.Method, input.Endpoint) {
		return nil, nil
	}
	amount := 0.0
	var err error
	if input.APIKey.Member.HasSpendingLimits() && enterpriseMemberBudgetRequiresAmountHold(input.Method, input.Endpoint) {
		amount, err = s.estimateUpperBound(ctx, input)
		if err != nil {
			return nil, err
		}
		if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return nil, ErrEnterpriseMemberBudgetUnbounded
		}
	}
	billingRequestID, err := normalizeEnterpriseMemberBudgetRequestID(input.RequestID)
	if err != nil {
		return nil, err
	}
	receiptKind := enterpriseMemberReceiptKind(input.Method, input.Endpoint)
	expiresAt := time.Now().Add(2 * time.Hour)
	if receiptKind == EnterpriseMemberReceiptKindAsyncImage {
		// Until the Redis task is linked, no background execution is allowed.
		// A short expiry lets recovery safely release a receipt orphaned by a
		// process crash between PostgreSQL reservation and Redis task creation.
		expiresAt = time.Now().Add(5 * time.Minute)
	}
	requestID := EnterpriseMemberBudgetRequestID(input.APIKey.ID, billingRequestID)
	var reservation *EnterpriseMemberBudgetReservation
	if typedRepo, ok := s.repo.(EnterpriseMemberTypedBudgetRepository); ok {
		reservation, err = typedRepo.ReserveWithKind(ctx, requestID, input.APIKey.Member.ID, input.APIKey.GroupID,
			HashUsageRequestPayload(input.Body), amount, receiptKind, expiresAt)
	} else {
		reservation, err = s.repo.Reserve(ctx, requestID, input.APIKey.Member.ID, input.APIKey.GroupID,
			HashUsageRequestPayload(input.Body), amount, expiresAt)
	}
	RecordEnterpriseMemberBudgetReservation(err)
	return reservation, err
}

func enterpriseMemberReceiptKind(method, endpoint string) string {
	if enterpriseMemberBudgetRequiresAmountHold(method, endpoint) {
		endpoint = strings.ToLower(strings.TrimSpace(endpoint))
		if strings.Contains(endpoint, "/images/") {
			return EnterpriseMemberReceiptKindAsyncImage
		}
		return EnterpriseMemberReceiptKindAsyncVideo
	}
	return EnterpriseMemberReceiptKindSync
}

func enterpriseMemberBudgetRequiresAmountHold(method, endpoint string) bool {
	if !strings.EqualFold(strings.TrimSpace(method), "POST") {
		return false
	}
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	return strings.HasSuffix(endpoint, "/images/generations/async") ||
		strings.HasSuffix(endpoint, "/images/edits/async") ||
		strings.HasSuffix(endpoint, "/videos/generations") ||
		strings.HasSuffix(endpoint, "/videos/edits") ||
		strings.HasSuffix(endpoint, "/videos/extensions")
}

func (s *EnterpriseMemberBudgetService) Release(ctx context.Context, apiKeyID int64, requestID string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	billingRequestID, err := normalizeEnterpriseMemberBudgetRequestID(requestID)
	if err != nil {
		return err
	}
	err = s.repo.Release(ctx, EnterpriseMemberBudgetRequestID(apiKeyID, billingRequestID))
	RecordEnterpriseMemberBudgetRelease(err)
	return err
}

// GetReservationByRequestID reads a fully scoped durable receipt identifier.
// It is used by asynchronous task recovery and must not be exposed to clients.
func (s *EnterpriseMemberBudgetService) GetReservationByRequestID(ctx context.Context, requestID string) (*EnterpriseMemberBudgetReservation, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	return s.repo.GetReservation(ctx, strings.TrimSpace(requestID))
}

func (s *EnterpriseMemberBudgetService) GetReservationByTaskID(ctx context.Context, taskID string) (*EnterpriseMemberBudgetReservation, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(taskID) == "" {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	typedRepo, ok := s.repo.(EnterpriseMemberAsyncTaskLookupRepository)
	if !ok {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	return typedRepo.GetReservationByTaskID(ctx, strings.TrimSpace(taskID))
}

func (s *EnterpriseMemberBudgetService) ReleaseReservationByRequestID(ctx context.Context, requestID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" {
		return ErrEnterpriseMemberBudgetReceiptNotFound
	}
	err := s.repo.Release(ctx, strings.TrimSpace(requestID))
	RecordEnterpriseMemberBudgetRelease(err)
	return err
}

func (s *EnterpriseMemberBudgetService) ReleaseImageTaskReservationByRequestID(ctx context.Context, requestID, taskID string) (*EnterpriseMemberBudgetReservation, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	typedRepo, ok := s.repo.(EnterpriseMemberAsyncTaskReleaseRepository)
	if !ok {
		return nil, ErrEnterpriseMemberBudgetConflict
	}
	receipt, err := typedRepo.ReleaseAsyncTask(ctx, strings.TrimSpace(requestID), strings.TrimSpace(taskID))
	RecordEnterpriseMemberBudgetRelease(err)
	return receipt, err
}

func (s *EnterpriseMemberBudgetService) MarkImageTaskReservationAmbiguousByRequestID(ctx context.Context, requestID, taskID, outcomeReason string) (*EnterpriseMemberBudgetReservation, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, ErrEnterpriseMemberBudgetReceiptNotFound
	}
	outcomeReason = strings.TrimSpace(outcomeReason)
	if outcomeReason == "" || len(outcomeReason) > 64 {
		return nil, ErrEnterpriseMemberBudgetConflict
	}
	typedRepo, ok := s.repo.(EnterpriseMemberAsyncTaskReleaseRepository)
	if !ok {
		return nil, ErrEnterpriseMemberBudgetConflict
	}
	return typedRepo.MarkAsyncTaskAmbiguous(ctx, strings.TrimSpace(requestID), strings.TrimSpace(taskID), outcomeReason)
}

func (s *EnterpriseMemberBudgetService) MarkReservationAmbiguousByRequestID(ctx context.Context, requestID, outcomeReason string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" {
		return ErrEnterpriseMemberBudgetReceiptNotFound
	}
	outcomeReason = strings.TrimSpace(outcomeReason)
	if outcomeReason == "" || len(outcomeReason) > 64 {
		return ErrEnterpriseMemberBudgetConflict
	}
	return s.repo.MarkAmbiguous(ctx, strings.TrimSpace(requestID), outcomeReason)
}

func (s *EnterpriseMemberBudgetService) AttachImageTask(ctx context.Context, requestID, taskID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return ErrEnterpriseMemberBudgetConflict
	}
	typedRepo, ok := s.repo.(EnterpriseMemberAsyncTaskRepository)
	if !ok {
		return ErrEnterpriseMemberBudgetConflict
	}
	return typedRepo.AttachAsyncTask(ctx, strings.TrimSpace(requestID), strings.TrimSpace(taskID), time.Now().Add(defaultImageTaskDispatchTimeout+defaultImageTaskRecoveryGrace))
}

func (s *EnterpriseMemberBudgetService) MarkImageTaskExecuting(ctx context.Context, requestID, taskID string) error {
	if s == nil || s.repo == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return ErrEnterpriseMemberBudgetConflict
	}
	typedRepo, ok := s.repo.(EnterpriseMemberAsyncTaskRepository)
	if !ok {
		return ErrEnterpriseMemberBudgetConflict
	}
	return typedRepo.MarkAsyncTaskExecuting(ctx, strings.TrimSpace(requestID), strings.TrimSpace(taskID))
}

func (s *EnterpriseMemberBudgetService) MarkAmbiguous(ctx context.Context, apiKeyID int64, requestID, outcomeReason string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	billingRequestID, err := normalizeEnterpriseMemberBudgetRequestID(requestID)
	if err != nil {
		return err
	}
	outcomeReason = strings.TrimSpace(outcomeReason)
	if outcomeReason == "" || len(outcomeReason) > 64 {
		return ErrEnterpriseMemberBudgetConflict
	}
	return s.repo.MarkAmbiguous(ctx, EnterpriseMemberBudgetRequestID(apiKeyID, billingRequestID), outcomeReason)
}

func EnterpriseMemberBudgetRequestID(apiKeyID int64, requestID string) string {
	return fmt.Sprintf("%d:%s", apiKeyID, strings.TrimSpace(requestID))
}

func normalizeEnterpriseMemberBudgetRequestID(requestID string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return "", ErrEnterpriseMemberBudgetConflict
	}
	if strings.HasPrefix(requestID, "client:") || strings.HasPrefix(requestID, "local:") || strings.HasPrefix(requestID, "generated:") {
		return requestID, nil
	}
	return "client:" + requestID, nil
}

func enterpriseMemberEndpointIsBillable(method, endpoint string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if strings.Contains(endpoint, "/count_tokens") || strings.Contains(endpoint, ":counttokens") {
		return false
	}
	if strings.HasSuffix(endpoint, "/responses/input_tokens") {
		return false
	}
	if strings.HasSuffix(endpoint, "/videos/generations") || strings.HasSuffix(endpoint, "/videos/edits") || strings.HasSuffix(endpoint, "/videos/extensions") {
		return true
	}
	if method == "GET" && (strings.HasSuffix(endpoint, "/models") || strings.Contains(endpoint, "/models/") || strings.Contains(endpoint, "/videos/")) {
		return false
	}
	if method == "GET" && strings.Contains(endpoint, "/images/tasks") {
		return false
	}
	if strings.Contains(endpoint, "/usage") || strings.HasSuffix(endpoint, "/images/batches") || strings.Contains(endpoint, "/batches/") || strings.Contains(endpoint, "/videos/") && !strings.HasSuffix(endpoint, "/generations") {
		return false
	}
	return true
}

func (s *EnterpriseMemberBudgetService) estimateUpperBound(ctx context.Context, input EnterpriseMemberBudgetEstimateInput) (float64, error) {
	if s == nil || s.pricingResolver == nil || input.APIKey == nil || input.APIKey.Member == nil {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	shape, err := parseEnterpriseMemberBudgetRequestShape(input.ContentType, input.Body)
	if err != nil {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	declaredOutputTokens := max(shape.MaxTokens, shape.MaxOutputTokens)
	if declaredOutputTokens > enterpriseMemberMaxOutputTokensUpperBound {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	inputTokensUpper := len(input.Body)
	if inputTokensUpper < 1 {
		inputTokensUpper = 1
	}
	count := shape.N
	if count <= 0 {
		count = 1
	}
	if count > enterpriseMemberMaxRequestCountUpperBound {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}

	maxCost := 0.0
	expandableInput := enterpriseMemberRequestMayExpandInput(input.Body)
	for i := range input.APIKey.Member.Groups {
		group := &input.APIKey.Member.Groups[i]
		baseMultiplier := group.RateMultiplier
		if s.userGroupRateRepo != nil {
			override, rateErr := s.userGroupRateRepo.GetByUserAndGroup(ctx, input.APIKey.UserID, group.ID)
			if rateErr != nil {
				return 0, fmt.Errorf("%w: user group rate unavailable", ErrEnterpriseMemberBudgetUnbounded)
			}
			if override != nil {
				baseMultiplier = *override
			}
		}
		if baseMultiplier <= 0 {
			baseMultiplier = 1
		}
		tokenMultiplier := baseMultiplier
		if group.PeakRateEnabled && group.PeakRateMultiplier > 1 {
			tokenMultiplier *= group.PeakRateMultiplier
		}
		imageMultiplier := baseMultiplier
		if group.ImageRateIndependent {
			imageMultiplier = math.Max(0, group.ImageRateMultiplier)
		}
		videoMultiplier := baseMultiplier
		if group.VideoRateIndependent {
			videoMultiplier = math.Max(0, group.VideoRateMultiplier)
		}

		requestedModel := strings.TrimSpace(input.RequestedModel)
		if requestedModel == "" {
			requestedModel = strings.TrimSpace(shape.Model)
		}
		if strings.HasSuffix(strings.ToLower(input.Endpoint), "/alpha/search") {
			unitPrice := defaultWebSearchPricePerCall
			if group.WebSearchPricePerCall != nil {
				unitPrice = math.Max(0, *group.WebSearchPricePerCall)
			}
			groupCost := float64(count) * unitPrice * baseMultiplier
			if groupCost <= 0 || math.IsNaN(groupCost) || math.IsInf(groupCost, 0) {
				return 0, ErrEnterpriseMemberBudgetUnbounded
			}
			maxCost = math.Max(maxCost, groupCost)
			continue
		}
		pricingModels, candidateErr := s.enterpriseMemberBudgetReachableModelCandidates(ctx, requestedModel, group)
		if candidateErr != nil {
			return 0, fmt.Errorf("%w: mapped model candidates unavailable", ErrEnterpriseMemberBudgetUnbounded)
		}
		groupCost := 0.0
		for _, pricingModel := range pricingModels {
			candidateCost := 0.0
			resolved := s.pricingResolver.Resolve(ctx, PricingInput{Model: pricingModel, GroupID: &group.ID})
			if resolved != nil {
				modelOutputLimit := 0
				if resolved.BasePricing != nil {
					modelOutputLimit = resolved.BasePricing.MaxOutputTokens
				}
				outputTokens := enterpriseMemberOutputTokenUpperBound(declaredOutputTokens, modelOutputLimit)
				groupInputTokensUpper := inputTokensUpper
				if expandableInput {
					modelInputLimit := 0
					if resolved.BasePricing != nil {
						modelInputLimit = resolved.BasePricing.MaxInputTokens
					}
					groupInputTokensUpper = max(groupInputTokensUpper, enterpriseMemberInputTokenUpperBound(modelInputLimit))
				}
				candidateCost = math.Max(candidateCost, resolvedPricingUpperBound(resolved, groupInputTokensUpper, outputTokens, count)*tokenMultiplier)
			}
			if strings.Contains(input.Endpoint, "/images/") {
				candidateCost = math.Max(candidateCost, enterpriseMemberImageUpperBound(s.pricingResolver.billingService, pricingModel, group, count)*imageMultiplier)
			}
			if strings.Contains(input.Endpoint, "/videos/") {
				candidateCost = math.Max(candidateCost, enterpriseMemberVideoUpperBound(s.pricingResolver.billingService, pricingModel, group, count, shape.Duration)*videoMultiplier)
			}
			// A single group may route through several channel/account mappings.
			// Every reachable model must be priceable; one cheap priced candidate
			// cannot justify a failover target whose cost is unknown.
			if candidateCost <= 0 || math.IsNaN(candidateCost) || math.IsInf(candidateCost, 0) {
				return 0, ErrEnterpriseMemberBudgetUnbounded
			}
			groupCost = math.Max(groupCost, candidateCost)
		}
		// Every authorized routing candidate must have a defensible upper bound.
		// Accepting a request because one group is priced while another candidate
		// is not would allow group failover to escape the amount reserved here.
		if groupCost <= 0 || math.IsNaN(groupCost) || math.IsInf(groupCost, 0) {
			return 0, ErrEnterpriseMemberBudgetUnbounded
		}
		maxCost = math.Max(maxCost, groupCost)
	}
	if maxCost <= 0 {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	// Direct input is bounded by request bytes; server-side references use the
	// model input cap. Output uses the declared or model cap. The margin covers
	// protocol transformations and pricing-mode normalization.
	return math.Ceil(maxCost*1.25*1e8) / 1e8, nil
}

func enterpriseMemberBudgetModelCandidates(requestedModel string, group *Group) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	appendModel := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" {
			return
		}
		key := strings.ToLower(model)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, model)
	}
	appendModel(requestedModel)
	if group != nil {
		appendModel(group.DefaultMappedModel)
		cfg := group.MessagesDispatchModelConfig
		appendModel(cfg.OpusMappedModel)
		appendModel(cfg.SonnetMappedModel)
		appendModel(cfg.HaikuMappedModel)
		for _, mapped := range cfg.ExactModelMappings {
			appendModel(mapped)
		}
	}
	return out
}

func (s *EnterpriseMemberBudgetService) enterpriseMemberBudgetReachableModelCandidates(ctx context.Context, requestedModel string, group *Group) ([]string, error) {
	models := enterpriseMemberBudgetModelCandidates(requestedModel, group)
	seen := make(map[string]struct{}, len(models)*3)
	for _, model := range models {
		seen[strings.ToLower(strings.TrimSpace(model))] = struct{}{}
	}
	appendModel := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" {
			return
		}
		key := strings.ToLower(model)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		models = append(models, model)
	}

	baseModels := append([]string(nil), models...)
	if group != nil && s.pricingResolver != nil && s.pricingResolver.channelService != nil {
		for _, model := range baseModels {
			mapping, err := s.pricingResolver.channelService.ResolveChannelMappingStrict(ctx, group.ID, model)
			if err != nil {
				return nil, err
			}
			appendModel(mapping.MappedModel)
		}
	}

	if group == nil || s.accountRepo == nil {
		return models, nil
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	routingModels := append([]string(nil), models...)
	for i := range accounts {
		for _, model := range routingModels {
			appendModel(resolveAccountUpstreamModel(&accounts[i], model))
		}
	}
	return models, nil
}

// ExtractEnterpriseMemberBudgetRequestModel reads the model without consuming
// the handler body. JSON and multipart image-edit requests share this parser so
// routing eligibility and budget pricing use the same request fact.
func ExtractEnterpriseMemberBudgetRequestModel(contentType string, body []byte) (string, error) {
	shape, err := parseEnterpriseMemberBudgetRequestShape(contentType, body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(shape.Model), nil
}

func parseEnterpriseMemberBudgetRequestShape(contentType string, body []byte) (enterpriseMemberBudgetRequestShape, error) {
	var shape enterpriseMemberBudgetRequestShape
	if len(bytes.TrimSpace(body)) == 0 {
		return shape, nil
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil && strings.TrimSpace(contentType) != "" {
		return shape, err
	}
	if strings.EqualFold(mediaType, "multipart/form-data") {
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			return shape, errors.New("multipart boundary is required")
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			part, nextErr := reader.NextPart()
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if nextErr != nil {
				return shape, nextErr
			}
			name := strings.ToLower(strings.TrimSpace(part.FormName()))
			if part.FileName() != "" || (name != "model" && name != "max_tokens" && name != "max_output_tokens" && name != "n" && name != "duration") {
				_ = part.Close()
				continue
			}
			value, readErr := io.ReadAll(io.LimitReader(part, 64<<10))
			_ = part.Close()
			if readErr != nil {
				return shape, readErr
			}
			trimmed := strings.TrimSpace(string(value))
			switch name {
			case "model":
				shape.Model = trimmed
			case "max_tokens", "max_output_tokens", "n", "duration":
				parsed, parseErr := strconv.Atoi(trimmed)
				if parseErr != nil {
					return shape, parseErr
				}
				switch name {
				case "max_tokens":
					shape.MaxTokens = parsed
				case "max_output_tokens":
					shape.MaxOutputTokens = parsed
				case "n":
					shape.N = parsed
				case "duration":
					shape.Duration = parsed
				}
			}
		}
		return shape, nil
	}
	if err := json.Unmarshal(body, &shape); err != nil {
		return shape, err
	}
	return shape, nil
}

func enterpriseMemberInputTokenUpperBound(modelLimit int) int {
	if modelLimit > 0 && modelLimit <= enterpriseMemberMaxInputTokensUpperBound {
		return modelLimit
	}
	return enterpriseMemberMaxInputTokensUpperBound
}

func enterpriseMemberRequestMayExpandInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var value any
	if json.Unmarshal(body, &value) != nil {
		return true
	}
	var inspect func(any) bool
	inspect = func(current any) bool {
		switch typed := current.(type) {
		case []any:
			for _, item := range typed {
				if inspect(item) {
					return true
				}
			}
		case map[string]any:
			for key, item := range typed {
				normalizedKey := strings.ToLower(strings.TrimSpace(key))
				if normalizedKey == "previous_response_id" || normalizedKey == "conversation" || normalizedKey == "attachments" || normalizedKey == "file_id" || normalizedKey == "image_url" {
					if item != nil && strings.TrimSpace(fmt.Sprint(item)) != "" {
						return true
					}
				}
				if normalizedKey == "type" {
					typeName := strings.ToLower(strings.TrimSpace(fmt.Sprint(item)))
					switch typeName {
					case "image", "image_url", "input_image", "file", "input_file", "document", "pdf":
						return true
					}
				}
				if inspect(item) {
					return true
				}
			}
		}
		return false
	}
	return inspect(value)
}

func enterpriseMemberOutputTokenUpperBound(declared, modelLimit int) int {
	if declared > 0 {
		return declared
	}
	if modelLimit > 0 && modelLimit <= enterpriseMemberMaxOutputTokensUpperBound {
		return modelLimit
	}
	return enterpriseMemberMaxOutputTokensUpperBound
}

func enterpriseMemberImageUpperBound(billingService *BillingService, model string, group *Group, count int) float64 {
	if billingService == nil || group == nil {
		return 0
	}
	if count <= 0 {
		count = 1
	}
	prices := make([]float64, 0, 3)
	for _, tier := range []struct {
		name       string
		groupPrice *float64
	}{
		{name: "1K", groupPrice: group.ImagePrice1K},
		{name: "2K", groupPrice: group.ImagePrice2K},
		{name: "4K", groupPrice: group.ImagePrice4K},
	} {
		if tier.groupPrice != nil {
			prices = append(prices, math.Max(0, *tier.groupPrice))
			continue
		}
		prices = append(prices, billingService.getDefaultImagePrice(model, tier.name))
	}
	return float64(count) * maxFloats(prices...)
}

func enterpriseMemberVideoUpperBound(billingService *BillingService, model string, group *Group, count, durationSeconds int) float64 {
	if billingService == nil || group == nil {
		return 0
	}
	if count <= 0 {
		count = 1
	}
	durationSeconds = NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds)
	prices := make([]float64, 0, 3)
	for _, tier := range []struct {
		name       string
		groupPrice *float64
	}{
		{name: VideoBillingResolution480P, groupPrice: group.VideoPrice480P},
		{name: VideoBillingResolution720P, groupPrice: group.VideoPrice720P},
		{name: VideoBillingResolution1080P, groupPrice: group.VideoPrice1080P},
	} {
		if tier.groupPrice != nil {
			prices = append(prices, math.Max(0, *tier.groupPrice))
			continue
		}
		prices = append(prices, billingService.getDefaultVideoPrice(model, tier.name))
	}
	unitPrice := maxFloats(prices...)
	return float64(count*durationSeconds) * unitPrice
}

func resolvedPricingUpperBound(resolved *ResolvedPricing, inputTokens, outputTokens, count int) float64 {
	if resolved == nil {
		return 0
	}
	if resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage {
		price := resolved.DefaultPerRequestPrice
		for i := range resolved.RequestTiers {
			if resolved.RequestTiers[i].PerRequestPrice != nil {
				price = math.Max(price, *resolved.RequestTiers[i].PerRequestPrice)
			}
		}
		return price * float64(count)
	}
	inputPrice, outputPrice := 0.0, 0.0
	if resolved.BasePricing != nil {
		p := resolved.BasePricing
		inputPrice = maxFloats(p.InputPricePerToken, p.InputPricePerTokenPriority, p.ImageInputPricePerToken, p.CacheCreationPricePerToken, p.CacheCreationPricePerTokenPriority, p.CacheCreation5mPrice, p.CacheCreation1hPrice, p.CacheReadPricePerToken, p.CacheReadPricePerTokenPriority)
		outputPrice = maxFloats(p.OutputPricePerToken, p.OutputPricePerTokenPriority, p.ImageOutputPricePerToken)
		inputPrice *= math.Max(1, p.LongContextInputMultiplier)
		outputPrice *= math.Max(1, p.LongContextOutputMultiplier)
	}
	for i := range resolved.Intervals {
		iv := &resolved.Intervals[i]
		inputPrice = math.Max(inputPrice, maxFloatPointers(iv.InputPrice, iv.CacheWritePrice, iv.CacheReadPrice))
		outputPrice = math.Max(outputPrice, maxFloatPointers(iv.OutputPrice))
	}
	return float64(inputTokens)*inputPrice + float64(outputTokens*count)*outputPrice
}

func maxFloats(values ...float64) float64 {
	max := 0.0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func maxFloatPointers(values ...*float64) float64 {
	max := 0.0
	for _, value := range values {
		if value != nil && *value > max {
			max = *value
		}
	}
	return max
}

func IsEnterpriseMemberBudgetExceeded(err error) bool {
	return errors.Is(err, ErrEnterpriseMemberBudgetExceeded) ||
		errors.Is(err, ErrEnterpriseMemberRateLimit5hExceeded) ||
		errors.Is(err, ErrEnterpriseMemberRateLimit1dExceeded) ||
		errors.Is(err, ErrEnterpriseMemberRateLimit7dExceeded) ||
		errors.Is(err, ErrEnterpriseMemberAsyncBudgetUnavailable)
}

type EnterpriseMemberBudgetRecoveryService struct {
	repo        EnterpriseMemberBudgetRepository
	billingRepo EnterpriseMemberUsageSettlementRepository
	cancel      context.CancelFunc
}

func NewEnterpriseMemberBudgetRecoveryService(repo EnterpriseMemberBudgetRepository, billingRepo EnterpriseMemberUsageSettlementRepository) *EnterpriseMemberBudgetRecoveryService {
	return &EnterpriseMemberBudgetRecoveryService{repo: repo, billingRepo: billingRepo}
}

func (s *EnterpriseMemberBudgetRecoveryService) Start() {
	if s == nil || s.repo == nil || s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if s.billingRepo != nil {
				replayCtx, replayCancel := context.WithTimeout(ctx, 30*time.Second)
				replayed, replayErr := s.billingRepo.ReplayPendingEnterpriseMemberSettlements(replayCtx, 100)
				if replayErr != nil {
					logger.LegacyPrintf("service.enterprise_member_budget", "enterprise member settlement replay failed: %v", replayErr)
				} else if replayed > 0 {
					logger.LegacyPrintf("service.enterprise_member_budget", "replayed %d pending enterprise member settlements", replayed)
				}
				replayCancel()
			}
			recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 30*time.Second)
			recovered, recoverErr := s.repo.RecoverExpired(recoveryCtx, 100)
			RecordEnterpriseMemberBudgetRecovery(recovered, recoverErr)
			recoveryCancel()
			reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 30*time.Second)
			reconciliation, reconcileErr := s.repo.ReconcilePeriods(reconcileCtx, 100)
			RecordEnterpriseMemberBudgetReconciliation(reconciliation, reconcileErr)
			reconcileCancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *EnterpriseMemberBudgetRecoveryService) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func ProvideEnterpriseMemberBudgetRecoveryService(repo EnterpriseMemberBudgetRepository, billingRepo EnterpriseMemberUsageSettlementRepository) *EnterpriseMemberBudgetRecoveryService {
	service := NewEnterpriseMemberBudgetRecoveryService(repo, billingRepo)
	service.Start()
	return service
}
