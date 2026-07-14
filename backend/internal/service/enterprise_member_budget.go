package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrEnterpriseMemberBudgetExceeded      = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_BUDGET_EXCEEDED", "enterprise member monthly budget is exhausted")
	ErrEnterpriseMemberRateLimit5hExceeded = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_5H_EXCEEDED", "enterprise member 5-hour spending limit is exhausted")
	ErrEnterpriseMemberRateLimit1dExceeded = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_1D_EXCEEDED", "enterprise member daily spending limit is exhausted")
	ErrEnterpriseMemberRateLimit7dExceeded = infraerrors.TooManyRequests("ENTERPRISE_MEMBER_RATE_7D_EXCEEDED", "enterprise member 7-day spending limit is exhausted")
	ErrEnterpriseMemberBudgetUnbounded     = infraerrors.BadRequest("ENTERPRISE_MEMBER_BUDGET_UNBOUNDED_REQUEST", "request cost cannot be bounded for the enterprise member budget")
	ErrEnterpriseMemberBudgetConflict      = infraerrors.Conflict("ENTERPRISE_MEMBER_BUDGET_REQUEST_CONFLICT", "member budget request id was reused with different parameters")
)

// EnterpriseMemberBudgetTimezone is the authoritative calendar timezone for member budgets and import openings.
const EnterpriseMemberBudgetTimezone = "Asia/Shanghai"
const enterpriseMemberBudgetTimezone = EnterpriseMemberBudgetTimezone

const enterpriseMemberUsageAuditNote = "usage values updated by %s"

type EnterpriseMemberBudgetReservation struct {
	ID          int64
	RequestID   string
	MemberID    int64
	PeriodStart time.Time
	ReservedUSD float64
	ActualUSD   float64
	Status      string
	UsageLogID  *int64
	ExpiresAt   time.Time
}

type EnterpriseMemberBudgetSummary struct {
	MemberID                  int64      `json:"member_id"`
	PeriodStart               time.Time  `json:"period_start"`
	PeriodEnd                 time.Time  `json:"period_end"`
	Timezone                  string     `json:"timezone"`
	LimitUSD                  float64    `json:"limit_usd"`
	UsedUSD                   float64    `json:"used_usd"`
	ReservedUSD               float64    `json:"reserved_usd"`
	RemainingUSD              float64    `json:"remaining_usd"`
	RequestCount              int64      `json:"request_count"`
	InputTokens               int64      `json:"input_tokens"`
	OutputTokens              int64      `json:"output_tokens"`
	MigrationBilledUSD        float64    `json:"migration_billed_usd"`
	MigrationTotalTokens      int64      `json:"migration_total_tokens"`
	MigrationInputTokens      int64      `json:"migration_input_tokens"`
	MigrationOutputTokens     int64      `json:"migration_output_tokens"`
	MigrationCacheTokens      int64      `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens int64      `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  int64      `json:"migration_cache_read_tokens"`
	RateLimit5h               float64    `json:"rate_limit_5h"`
	RateLimit1d               float64    `json:"rate_limit_1d"`
	RateLimit7d               float64    `json:"rate_limit_7d"`
	Usage5h                   float64    `json:"usage_5h"`
	Usage1d                   float64    `json:"usage_1d"`
	Usage7d                   float64    `json:"usage_7d"`
	Reset5hAt                 *time.Time `json:"reset_5h_at,omitempty"`
	Reset1dAt                 *time.Time `json:"reset_1d_at,omitempty"`
	Reset7dAt                 *time.Time `json:"reset_7d_at,omitempty"`
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
	MemberID                  int64   `json:"member_id"`
	MemberCode                string  `json:"member_code"`
	MemberName                string  `json:"member_name"`
	Status                    string  `json:"status"`
	LimitUSD                  float64 `json:"limit_usd"`
	UsedUSD                   float64 `json:"used_usd"`
	ReservedUSD               float64 `json:"reserved_usd"`
	RemainingUSD              float64 `json:"remaining_usd"`
	RequestCount              int64   `json:"request_count"`
	InputTokens               int64   `json:"input_tokens"`
	OutputTokens              int64   `json:"output_tokens"`
	MigrationBilledUSD        float64 `json:"migration_billed_usd"`
	MigrationTotalTokens      int64   `json:"migration_total_tokens"`
	MigrationInputTokens      int64   `json:"migration_input_tokens"`
	MigrationOutputTokens     int64   `json:"migration_output_tokens"`
	MigrationCacheTokens      int64   `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens int64   `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  int64   `json:"migration_cache_read_tokens"`
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
	MigrationTotalTokens      int64                            `json:"migration_total_tokens"`
	MigrationInputTokens      int64                            `json:"migration_input_tokens"`
	MigrationOutputTokens     int64                            `json:"migration_output_tokens"`
	MigrationCacheTokens      int64                            `json:"migration_cache_tokens"`
	MigrationCacheWriteTokens int64                            `json:"migration_cache_write_tokens"`
	MigrationCacheReadTokens  int64                            `json:"migration_cache_read_tokens"`
	Members                   []EnterpriseMemberOwnerUsageItem `json:"members"`
}

type EnterpriseMemberBudgetRepository interface {
	Reserve(ctx context.Context, requestID string, memberID int64, amount float64, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error)
	Release(ctx context.Context, requestID string) error
	GetPeriod(ctx context.Context, memberID int64, periodStart time.Time) (usedUSD, reservedUSD float64, err error)
	GetSummary(ctx context.Context, memberID int64, periodStart, periodEnd time.Time) (*EnterpriseMemberBudgetSummary, error)
	ListEntries(ctx context.Context, memberID int64, periodStart time.Time, limit, offset int) ([]EnterpriseMemberBudgetEntry, int64, error)
	CreateAdjustment(ctx context.Context, memberID int64, periodStart time.Time, amount float64, actorUserID int64, idempotencyKey, note string) error
	SetUsage(ctx context.Context, ownerID, memberID int64, periodStart time.Time, monthlyUsed, usage5h, usage1d, usage7d float64, actorUserID int64, idempotencyKey, note string) error
	GetUsageAnalytics(ctx context.Context, memberID int64, start, end time.Time) (*EnterpriseMemberUsageAnalytics, error)
	GetOwnerUsageSummary(ctx context.Context, ownerID int64, periodStart, periodEnd time.Time) (*EnterpriseMemberOwnerUsageSummary, error)
	GetOwnerUsageTrend(ctx context.Context, ownerID int64, start, end time.Time) ([]EnterpriseMemberUsageTrendPoint, error)
	RecoverExpired(ctx context.Context, limit int) (int, error)
	ReconcilePeriods(ctx context.Context, limit int) (EnterpriseMemberBudgetReconciliationResult, error)
}

type EnterpriseMemberUsageAdjustmentInput struct {
	MonthlyUsedUSD float64 `json:"monthly_used_usd"`
	Usage5h        float64 `json:"usage_5h"`
	Usage1d        float64 `json:"usage_1d"`
	Usage7d        float64 `json:"usage_7d"`
	Note           string  `json:"note"`
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
	Endpoint       string
	Body           []byte
}

type EnterpriseMemberBudgetService struct {
	repo              EnterpriseMemberBudgetRepository
	pricingResolver   *ModelPricingResolver
	userGroupRateRepo UserGroupRateRepository
}

func NewEnterpriseMemberBudgetService(repo EnterpriseMemberBudgetRepository, pricingResolver *ModelPricingResolver, rateRepo UserGroupRateRepository) *EnterpriseMemberBudgetService {
	return &EnterpriseMemberBudgetService{repo: repo, pricingResolver: pricingResolver, userGroupRateRepo: rateRepo}
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

func enterpriseMemberSystemUsageNote(hasUsage bool, source string) string {
	if !hasUsage {
		return ""
	}
	return fmt.Sprintf(enterpriseMemberUsageAuditNote, source)
}

func validateEnterpriseUsageValues(values ...float64) error {
	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || value > 1_000_000_000_000 {
			return ErrEnterpriseMemberInvalid
		}
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
	if !input.APIKey.Member.HasSpendingLimits() || !enterpriseMemberEndpointIsBillable(input.Endpoint) {
		return nil, nil
	}
	amount, err := s.estimateUpperBound(ctx, input)
	if err != nil {
		return nil, err
	}
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, ErrEnterpriseMemberBudgetUnbounded
	}
	billingRequestID, err := normalizeEnterpriseMemberBudgetRequestID(input.RequestID)
	if err != nil {
		return nil, err
	}
	reservation, err := s.repo.Reserve(ctx, EnterpriseMemberBudgetRequestID(input.APIKey.ID, billingRequestID), input.APIKey.Member.ID, amount, time.Now().Add(2*time.Hour))
	RecordEnterpriseMemberBudgetReservation(err)
	return reservation, err
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

func enterpriseMemberEndpointIsBillable(endpoint string) bool {
	endpoint = strings.ToLower(endpoint)
	if strings.HasSuffix(endpoint, "/models") || strings.Contains(endpoint, "/usage") || strings.HasSuffix(endpoint, "/images/batches") || strings.Contains(endpoint, "/batches/") || strings.Contains(endpoint, "/videos/") && !strings.HasSuffix(endpoint, "/generations") {
		return false
	}
	return true
}

func (s *EnterpriseMemberBudgetService) estimateUpperBound(ctx context.Context, input EnterpriseMemberBudgetEstimateInput) (float64, error) {
	if s == nil || s.pricingResolver == nil || input.APIKey == nil || input.APIKey.Member == nil {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	var shape struct {
		MaxTokens       int `json:"max_tokens"`
		MaxOutputTokens int `json:"max_output_tokens"`
		N               int `json:"n"`
	}
	if len(input.Body) > 0 && json.Unmarshal(input.Body, &shape) != nil {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	outputTokens := shape.MaxOutputTokens
	if outputTokens <= 0 {
		outputTokens = shape.MaxTokens
	}
	if outputTokens <= 0 {
		outputTokens = 8192
	}
	if outputTokens > 1_000_000 {
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

	maxCost := 0.0
	for i := range input.APIKey.Member.Groups {
		group := &input.APIKey.Member.Groups[i]
		resolved := s.pricingResolver.Resolve(ctx, PricingInput{Model: input.RequestedModel, GroupID: &group.ID})
		if resolved == nil {
			continue
		}
		multiplier := group.RateMultiplier
		if s.userGroupRateRepo != nil {
			if override, err := s.userGroupRateRepo.GetByUserAndGroup(ctx, input.APIKey.UserID, group.ID); err == nil && override != nil {
				multiplier = *override
			}
		}
		if multiplier <= 0 {
			multiplier = 1
		}
		if group.PeakRateEnabled && group.PeakRateMultiplier > 1 {
			multiplier *= group.PeakRateMultiplier
		}
		cost := resolvedPricingUpperBound(resolved, inputTokensUpper, outputTokens, count)
		if strings.Contains(input.Endpoint, "/images/") {
			cost = math.Max(cost, float64(count)*maxFloatPointers(group.ImagePrice1K, group.ImagePrice2K, group.ImagePrice4K))
		}
		if strings.Contains(input.Endpoint, "/videos/") {
			cost = math.Max(cost, float64(count)*maxFloatPointers(group.VideoPrice480P, group.VideoPrice720P, group.VideoPrice1080P))
		}
		maxCost = math.Max(maxCost, cost*multiplier)
	}
	if maxCost <= 0 {
		return 0, ErrEnterpriseMemberBudgetUnbounded
	}
	// Token count is bounded by request bytes and declared output tokens. The
	// margin covers protocol transformations and pricing-mode normalization.
	return math.Ceil(maxCost*1.25*1e8) / 1e8, nil
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
	return float64(inputTokens)*inputPrice + float64(outputTokens)*outputPrice
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
		errors.Is(err, ErrEnterpriseMemberRateLimit7dExceeded)
}

type EnterpriseMemberBudgetRecoveryService struct {
	repo   EnterpriseMemberBudgetRepository
	cancel context.CancelFunc
}

func NewEnterpriseMemberBudgetRecoveryService(repo EnterpriseMemberBudgetRepository) *EnterpriseMemberBudgetRecoveryService {
	return &EnterpriseMemberBudgetRecoveryService{repo: repo}
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

func ProvideEnterpriseMemberBudgetRecoveryService(repo EnterpriseMemberBudgetRepository) *EnterpriseMemberBudgetRecoveryService {
	service := NewEnterpriseMemberBudgetRecoveryService(repo)
	service.Start()
	return service
}
