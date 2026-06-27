package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	UserModelStatusUnknown = "unknown"

	userModelMessageNormal      = "normal"
	userModelMessagePartial     = "partial"
	userModelMessageUnavailable = "unavailable"
	userModelMessageNoData      = "no_data"

	modelStatusWindow24h      = 1
	modelSelfCheckFreshWindow = 10 * time.Minute
)

// ModelSelfCheckRepository is the read model behind the public model status
// page. It deliberately reads model_self_check_* storage, not channel_monitor_*.
type ModelSelfCheckRepository interface {
	ListStatusTargets(ctx context.Context) ([]ModelSelfCheckTarget, error)
	ListTargetAccounts(ctx context.Context, groupIDs []int64) ([]ModelSelfCheckTargetAccount, error)
	ListLatestByModels(ctx context.Context, models []string) ([]ModelSelfCheckHistory, error)
	ListHistoriesSince(ctx context.Context, models []string, since time.Time) ([]ModelSelfCheckHistory, error)
	ListRecentHistories(ctx context.Context, model string, accountIDs []int64, limit int) ([]ModelSelfCheckHistory, error)
	CreateHistory(ctx context.Context, history *ModelSelfCheckHistory) error
}

type ModelSelfCheckAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
}

type ModelSelfCheckTarget struct {
	GroupID       int64
	GroupName     string
	GroupPlatform string
	Model         string
}

type ModelSelfCheckTargetAccount struct {
	GroupID   int64
	AccountID int64
	Platform  string
}

type ModelSelfCheckHistory struct {
	ID         int64
	Model      string
	AccountID  int64
	Platform   string
	Status     string
	LatencyMs  *int
	HTTPStatus *int
	ErrorCode  string
	CheckedAt  time.Time
}

// UserModelStatusView is the user-facing model health row. It deliberately
// omits account IDs, providers, upstream endpoints, raw errors, and costs.
type UserModelStatusView struct {
	GroupID          int64
	GroupName        string
	Model            string
	DisplayName      string
	Status           string
	MessageCode      string
	LatestLatencyMs  *int
	AvgLatency24hMs  *int
	AvgLatency7dMs   *int
	Availability24h  *float64
	Availability7d   *float64
	Availability30d  *float64
	DegradedRatio24h *float64
	LastCheckedAt    *time.Time
	Timeline         []UserModelTimelinePoint
}

// UserModelStatusDetail extends the user-facing model health row with a larger
// recent timeline. It keeps the same public-only boundary as the list row.
type UserModelStatusDetail struct {
	UserModelStatusView
}

type UserModelTimelinePoint struct {
	Status    string
	LatencyMs *int
	CheckedAt time.Time
}

type ModelSelfCheckService struct {
	repo          ModelSelfCheckRepository
	accountRepo   ModelSelfCheckAccountRepository
	probeExecutor ModelSelfCheckProbeExecutor
	now           func() time.Time
}

func NewModelSelfCheckService(repo ModelSelfCheckRepository) *ModelSelfCheckService {
	return &ModelSelfCheckService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *ModelSelfCheckService) SetProbeDependencies(accountRepo ModelSelfCheckAccountRepository, executor ModelSelfCheckProbeExecutor) {
	if s == nil {
		return
	}
	s.accountRepo = accountRepo
	s.probeExecutor = executor
}

func (s *ModelSelfCheckService) RecordHistory(ctx context.Context, history *ModelSelfCheckHistory) error {
	if history == nil {
		return fmt.Errorf("record model self check history: nil history")
	}
	history.Model = strings.TrimSpace(history.Model)
	history.Platform = strings.TrimSpace(history.Platform)
	history.Status = strings.TrimSpace(history.Status)
	if history.Model == "" {
		return fmt.Errorf("record model self check history: model is required")
	}
	if history.AccountID <= 0 {
		return fmt.Errorf("record model self check history: account_id is required")
	}
	if history.Platform == "" {
		return fmt.Errorf("record model self check history: platform is required")
	}
	if history.Status == "" {
		history.Status = MonitorStatusError
	}
	if history.CheckedAt.IsZero() {
		history.CheckedAt = s.now().UTC()
	}
	if err := s.repo.CreateHistory(ctx, history); err != nil {
		return fmt.Errorf("record model self check history: %w", err)
	}
	return nil
}

type modelSelfCheckStatusData struct {
	now             time.Time
	targets         []ModelSelfCheckTarget
	accountsByGroup map[int64][]ModelSelfCheckTargetAccount
	accountsByID    map[int64]*Account
	latestByModel   map[string]map[int64]*ModelSelfCheckHistory
	historyByModel  map[string]map[int64][]ModelSelfCheckHistory
}

type modelSelfCheckAvailabilityAggregate struct {
	Availability  *float64
	AvgLatencyMs  *int
	DegradedRatio *float64
}

// ListUserModelStatus returns a global, user-safe model status list. This is a
// site-level public health surface; it is not filtered per current user.
func (s *ModelSelfCheckService) ListUserModelStatus(ctx context.Context) ([]*UserModelStatusView, error) {
	data, err := s.loadStatusData(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*UserModelStatusView, 0, len(data.targets))
	for _, target := range data.targets {
		out = append(out, s.buildStatusView(ctx, target, data, nil))
	}
	return out, nil
}

// GetUserModelStatus returns public status detail for a single (group, model).
// groupID may be 0 for compatibility; in that case the first matching model is
// returned after the normal sorted target order.
func (s *ModelSelfCheckService) GetUserModelStatus(ctx context.Context, groupID int64, model string) (*UserModelStatusDetail, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, ErrChannelMonitorNotFound
	}
	data, err := s.loadStatusData(ctx)
	if err != nil {
		return nil, err
	}
	target, ok := findSelfCheckTarget(data.targets, groupID, model)
	if !ok {
		return nil, ErrChannelMonitorNotFound
	}
	accountIDs := s.accountIDsForTarget(ctx, target, data)
	timeline, err := s.loadTimeline(ctx, target.Model, accountIDs)
	if err != nil {
		return nil, err
	}
	return &UserModelStatusDetail{
		UserModelStatusView: *s.buildStatusView(ctx, target, data, timeline),
	}, nil
}

func (s *ModelSelfCheckService) loadStatusData(ctx context.Context) (*modelSelfCheckStatusData, error) {
	now := s.now().UTC()
	targets, err := s.repo.ListStatusTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list model self check targets: %w", err)
	}
	sortSelfCheckTargets(targets)
	data := &modelSelfCheckStatusData{
		now:             now,
		targets:         targets,
		accountsByGroup: map[int64][]ModelSelfCheckTargetAccount{},
		accountsByID:    map[int64]*Account{},
		latestByModel:   map[string]map[int64]*ModelSelfCheckHistory{},
		historyByModel:  map[string]map[int64][]ModelSelfCheckHistory{},
	}
	if len(targets) == 0 {
		return data, nil
	}

	groupIDs := uniqueSelfCheckGroupIDs(targets)
	accounts, err := s.repo.ListTargetAccounts(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("list model self check target accounts: %w", err)
	}
	for _, account := range accounts {
		data.accountsByGroup[account.GroupID] = append(data.accountsByGroup[account.GroupID], account)
	}
	if s.accountRepo != nil {
		fullAccounts, err := s.accountRepo.GetByIDs(ctx, uniqueSelfCheckAccountIDs(accounts))
		if err != nil {
			return nil, fmt.Errorf("list model self check account details: %w", err)
		}
		for _, account := range fullAccounts {
			if account == nil {
				continue
			}
			cp := *account
			data.accountsByID[account.ID] = &cp
		}
	}

	models := uniqueSelfCheckModels(targets)
	latestRows, err := s.repo.ListLatestByModels(ctx, models)
	if err != nil {
		return nil, fmt.Errorf("list model self check latest: %w", err)
	}
	for i := range latestRows {
		row := latestRows[i]
		if data.latestByModel[row.Model] == nil {
			data.latestByModel[row.Model] = map[int64]*ModelSelfCheckHistory{}
		}
		data.latestByModel[row.Model][row.AccountID] = &row
	}

	historyRows, err := s.repo.ListHistoriesSince(ctx, models, now.AddDate(0, 0, -monitorAvailability30Days))
	if err != nil {
		return nil, fmt.Errorf("list model self check histories: %w", err)
	}
	for _, row := range historyRows {
		if data.historyByModel[row.Model] == nil {
			data.historyByModel[row.Model] = map[int64][]ModelSelfCheckHistory{}
		}
		data.historyByModel[row.Model][row.AccountID] = append(data.historyByModel[row.Model][row.AccountID], row)
	}
	return data, nil
}

func (s *ModelSelfCheckService) buildStatusView(
	ctx context.Context,
	target ModelSelfCheckTarget,
	data *modelSelfCheckStatusData,
	timeline []UserModelTimelinePoint,
) *UserModelStatusView {
	accountIDs := s.accountIDsForTarget(ctx, target, data)
	latestRows := collectSelfCheckLatest(target.Model, accountIDs, data.latestByModel)
	freshLatest := filterFreshSelfCheckLatest(latestRows, data.now)
	status := aggregateSelfCheckStatus(freshLatest, len(accountIDs))
	availability24h := aggregateSelfCheckAvailability(target.Model, accountIDs, data.historyByModel, data.now, modelStatusWindow24h)
	availability7d := aggregateSelfCheckAvailability(target.Model, accountIDs, data.historyByModel, data.now, monitorAvailability7Days)
	availability30d := aggregateSelfCheckAvailability(target.Model, accountIDs, data.historyByModel, data.now, monitorAvailability30Days)

	return &UserModelStatusView{
		GroupID:          target.GroupID,
		GroupName:        target.GroupName,
		Model:            target.Model,
		DisplayName:      target.Model,
		Status:           status,
		MessageCode:      messageCodeForModelStatus(status),
		LatestLatencyMs:  bestSelfCheckLatency(freshLatest),
		AvgLatency24hMs:  availability24h.AvgLatencyMs,
		AvgLatency7dMs:   availability7d.AvgLatencyMs,
		Availability24h:  availability24h.Availability,
		Availability7d:   availability7d.Availability,
		Availability30d:  availability30d.Availability,
		DegradedRatio24h: availability24h.DegradedRatio,
		LastCheckedAt:    latestSelfCheckCheckedAt(freshLatest),
		Timeline:         timeline,
	}
}

func findSelfCheckTarget(targets []ModelSelfCheckTarget, groupID int64, model string) (ModelSelfCheckTarget, bool) {
	modelLower := strings.ToLower(strings.TrimSpace(model))
	for _, target := range targets {
		if groupID > 0 && target.GroupID != groupID {
			continue
		}
		if strings.ToLower(target.Model) == modelLower {
			return target, true
		}
	}
	return ModelSelfCheckTarget{}, false
}

func (s *ModelSelfCheckService) accountIDsForTarget(ctx context.Context, target ModelSelfCheckTarget, data *modelSelfCheckStatusData) []int64 {
	if !s.targetAllowsSelfCheckModel(ctx, target) {
		return []int64{}
	}
	accounts := data.accountsByGroup[target.GroupID]
	ids := make([]int64, 0, len(accounts))
	seen := map[int64]struct{}{}
	for _, account := range accounts {
		if !samePlatform(target.GroupPlatform, account.Platform) {
			continue
		}
		if !s.accountCanSelfCheckTarget(ctx, target, data, account.AccountID) {
			continue
		}
		if _, ok := seen[account.AccountID]; ok {
			continue
		}
		seen[account.AccountID] = struct{}{}
		ids = append(ids, account.AccountID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *ModelSelfCheckService) targetAllowsSelfCheckModel(ctx context.Context, target ModelSelfCheckTarget) bool {
	gateway := s.gatewayServiceForModelSupport()
	if gateway == nil {
		return true
	}
	groupID := target.GroupID
	return !gateway.checkChannelPricingRestriction(ctx, &groupID, target.Model)
}

func (s *ModelSelfCheckService) accountCanSelfCheckTarget(ctx context.Context, target ModelSelfCheckTarget, data *modelSelfCheckStatusData, accountID int64) bool {
	if data == nil || len(data.accountsByID) == 0 {
		return true
	}
	account := data.accountsByID[accountID]
	if account == nil {
		return false
	}
	if !isAccountEligibleForSelfCheck(account) || !s.isModelSupportedBySelfCheckAccount(ctx, account, target.Model) {
		return false
	}
	gateway := s.gatewayServiceForModelSupport()
	if gateway == nil {
		return true
	}
	groupID := target.GroupID
	return !gateway.needsUpstreamChannelRestrictionCheck(ctx, &groupID) ||
		!gateway.isUpstreamModelRestrictedByChannel(ctx, target.GroupID, account, target.Model)
}

func samePlatform(groupPlatform, accountPlatform string) bool {
	groupPlatform = strings.TrimSpace(strings.ToLower(groupPlatform))
	accountPlatform = strings.TrimSpace(strings.ToLower(accountPlatform))
	return groupPlatform == "" || accountPlatform == "" || groupPlatform == accountPlatform
}

func collectSelfCheckLatest(
	model string,
	accountIDs []int64,
	latestByModel map[string]map[int64]*ModelSelfCheckHistory,
) []*ModelSelfCheckHistory {
	out := make([]*ModelSelfCheckHistory, 0, len(accountIDs))
	modelLatest := latestByModel[model]
	for _, accountID := range accountIDs {
		if latest := modelLatest[accountID]; latest != nil {
			out = append(out, latest)
		}
	}
	return out
}

func filterFreshSelfCheckLatest(rows []*ModelSelfCheckHistory, now time.Time) []*ModelSelfCheckHistory {
	fresh := make([]*ModelSelfCheckHistory, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.CheckedAt.After(now) || now.Sub(row.CheckedAt) <= modelSelfCheckFreshWindow {
			fresh = append(fresh, row)
		}
	}
	return fresh
}

func aggregateSelfCheckStatus(latest []*ModelSelfCheckHistory, expectedAccounts int) string {
	if expectedAccounts == 0 {
		return MonitorStatusFailed
	}
	if len(latest) == 0 {
		return UserModelStatusUnknown
	}
	usable := 0
	degraded := 0
	bad := 0
	for _, row := range latest {
		switch row.Status {
		case MonitorStatusOperational:
			usable++
		case MonitorStatusDegraded:
			usable++
			degraded++
		case MonitorStatusFailed, MonitorStatusError:
			bad++
		default:
			bad++
		}
	}
	if usable == 0 {
		return MonitorStatusFailed
	}
	if bad > 0 || degraded > 0 || len(latest) < expectedAccounts {
		return MonitorStatusDegraded
	}
	return MonitorStatusOperational
}

func messageCodeForModelStatus(status string) string {
	switch status {
	case MonitorStatusOperational:
		return userModelMessageNormal
	case MonitorStatusDegraded:
		return userModelMessagePartial
	case MonitorStatusFailed, MonitorStatusError:
		return userModelMessageUnavailable
	default:
		return userModelMessageNoData
	}
}

func bestSelfCheckLatency(rows []*ModelSelfCheckHistory) *int {
	var best *int
	for _, row := range rows {
		if row.LatencyMs == nil {
			continue
		}
		if row.Status != MonitorStatusOperational && row.Status != MonitorStatusDegraded {
			continue
		}
		if best == nil || *row.LatencyMs < *best {
			v := *row.LatencyMs
			best = &v
		}
	}
	return best
}

func latestSelfCheckCheckedAt(rows []*ModelSelfCheckHistory) *time.Time {
	var latestAt *time.Time
	for _, row := range rows {
		checkedAt := row.CheckedAt.UTC()
		if latestAt == nil || checkedAt.After(*latestAt) {
			v := checkedAt
			latestAt = &v
		}
	}
	return latestAt
}

func aggregateSelfCheckAvailability(
	model string,
	accountIDs []int64,
	historyByModel map[string]map[int64][]ModelSelfCheckHistory,
	now time.Time,
	windowDays int,
) modelSelfCheckAvailabilityAggregate {
	since := now.AddDate(0, 0, -windowDays)
	var totalChecks int
	var usableChecks int
	var degradedChecks int
	var latencyChecks int
	var latencySum int
	modelHistory := historyByModel[model]
	for _, accountID := range accountIDs {
		for _, row := range modelHistory[accountID] {
			if row.CheckedAt.Before(since) {
				continue
			}
			totalChecks++
			switch row.Status {
			case MonitorStatusOperational:
				usableChecks++
				if row.LatencyMs != nil {
					latencyChecks++
					latencySum += *row.LatencyMs
				}
			case MonitorStatusDegraded:
				usableChecks++
				degradedChecks++
				if row.LatencyMs != nil {
					latencyChecks++
					latencySum += *row.LatencyMs
				}
			}
		}
	}
	var availability *float64
	if totalChecks > 0 {
		v := float64(usableChecks) * 100 / float64(totalChecks)
		availability = &v
	}
	var degradedRatio *float64
	if totalChecks > 0 {
		v := float64(degradedChecks) * 100 / float64(totalChecks)
		degradedRatio = &v
	}
	var avgLatency *int
	if latencyChecks > 0 {
		v := latencySum / latencyChecks
		avgLatency = &v
	}
	return modelSelfCheckAvailabilityAggregate{
		Availability:  availability,
		AvgLatencyMs:  avgLatency,
		DegradedRatio: degradedRatio,
	}
}

func (s *ModelSelfCheckService) loadTimeline(ctx context.Context, model string, accountIDs []int64) ([]UserModelTimelinePoint, error) {
	if len(accountIDs) == 0 {
		return []UserModelTimelinePoint{}, nil
	}
	rows, err := s.repo.ListRecentHistories(ctx, model, accountIDs, monitorTimelineMaxPoints)
	if err != nil {
		return nil, fmt.Errorf("list model self check timeline: %w", err)
	}
	points := make([]UserModelTimelinePoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, UserModelTimelinePoint{
			Status:    row.Status,
			LatencyMs: row.LatencyMs,
			CheckedAt: row.CheckedAt,
		})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].CheckedAt.After(points[j].CheckedAt)
	})
	if len(points) > monitorTimelineMaxPoints {
		points = points[:monitorTimelineMaxPoints]
	}
	return points, nil
}

func sortSelfCheckTargets(targets []ModelSelfCheckTarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].GroupName != targets[j].GroupName {
			return targets[i].GroupName < targets[j].GroupName
		}
		if targets[i].GroupID != targets[j].GroupID {
			return targets[i].GroupID < targets[j].GroupID
		}
		return targets[i].Model < targets[j].Model
	})
}

func uniqueSelfCheckGroupIDs(targets []ModelSelfCheckTarget) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(targets))
	for _, target := range targets {
		if _, ok := seen[target.GroupID]; ok {
			continue
		}
		seen[target.GroupID] = struct{}{}
		out = append(out, target.GroupID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func uniqueSelfCheckModels(targets []ModelSelfCheckTarget) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		model := strings.TrimSpace(target.Model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}
