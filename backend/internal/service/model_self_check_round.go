package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	modelSelfCheckProbeRoundStatusChecking = "checking"

	modelSelfCheckProbeStepOutcomeSkipped      = "skipped"
	modelSelfCheckProbeStepOutcomePending      = "pending"
	modelSelfCheckProbeStepOutcomeIncomplete   = "incomplete"
	modelSelfCheckProbeStepOutcomeSucceeded    = "succeeded"
	modelSelfCheckProbeStepOutcomeFailed       = "failed"
	modelSelfCheckProbeStepOutcomeNotAttempted = "not_attempted"

	modelSelfCheckProbeReasonOK             = "ok"
	modelSelfCheckProbeReasonUnsupported    = "unsupported_model"
	modelSelfCheckProbeReasonIneligible     = "account_ineligible"
	modelSelfCheckProbeReasonDisabled       = "disabled"
	modelSelfCheckProbeReasonNotSchedulable = "not_schedulable"
	modelSelfCheckProbeReasonGroupRemoved   = "group_removed"
	modelSelfCheckProbeReasonExpired        = "expired"
	modelSelfCheckProbeReasonOverloaded     = "overloaded"
	modelSelfCheckProbeReasonRateLimited    = "rate_limited"
	modelSelfCheckProbeReasonTempUnsched    = "temporarily_unschedulable"
	modelSelfCheckProbeReasonQuotaExceeded  = "quota_exceeded"
	modelSelfCheckProbeReasonModelCooldown  = "model_cooldown"
	modelSelfCheckProbeReasonParent         = "parent_unavailable"
	modelSelfCheckProbeReasonPlatform       = "platform_mismatch"
	modelSelfCheckProbeReasonChannel        = "channel_restricted"
	modelSelfCheckProbeReasonNoEligible     = "no_eligible_account"
	modelSelfCheckProbeReasonAllFailed      = "all_probe_failed"
	modelSelfCheckProbeReasonFallbackOK     = "fallback_succeeded"
	modelSelfCheckProbeReasonPriorSuccess   = "prior_success"
	modelSelfCheckProbeReasonRoundDeadline  = "round_deadline"
	modelSelfCheckProbeReasonIncomplete     = "round_incomplete"

	modelSelfCheckProbeAttemptTimeout = 30 * time.Second
	modelSelfCheckProbeRoundTimeout   = 90 * time.Second
)

type modelSelfCheckRoundCandidate struct {
	accountID  int64
	account    *Account
	eligible   bool
	reasonCode string
	order      int
	last       *ModelSelfCheckHistory
}

func (s *ModelSelfCheckService) roundRepo() ModelSelfCheckRoundRepository {
	if s == nil || s.repo == nil {
		return nil
	}
	repo, ok := s.repo.(ModelSelfCheckRoundRepository)
	if !ok {
		return nil
	}
	return repo
}

func (s *ModelSelfCheckService) RunProbeRound(ctx context.Context, task ModelSelfCheckProbeTask) (err error) {
	if s == nil {
		return fmt.Errorf("run model self check probe round: nil service")
	}
	if s.accountRepo == nil {
		return fmt.Errorf("run model self check probe round: account repository is not configured")
	}
	if s.probeExecutor == nil {
		return fmt.Errorf("run model self check probe round: probe executor is not configured")
	}
	roundRepo := s.roundRepo()
	if roundRepo == nil {
		return s.RunProbe(ctx, task)
	}
	model := strings.TrimSpace(task.Model)
	if model == "" || task.GroupID <= 0 {
		return fmt.Errorf("run model self check probe round: invalid task")
	}
	target, data, err := s.loadSingleTargetStatusData(ctx, task.GroupID, model)
	if err != nil {
		return err
	}
	if data == nil {
		return ErrChannelMonitorNotFound
	}
	candidates := s.modelSelfCheckCandidates(ctx, target, data)
	eligible := eligibleModelSelfCheckCandidates(candidates)
	startedAt := s.now().UTC()
	round := &ModelSelfCheckProbeRound{
		GroupID:    target.GroupID,
		Model:      target.Model,
		Status:     modelSelfCheckProbeRoundStatusChecking,
		ReasonCode: modelSelfCheckProbeReasonIncomplete,
		StartedAt:  startedAt,
		Steps:      stepsFromRoundCandidates(candidates),
	}
	if len(eligible) == 0 {
		finishedAt := startedAt
		duration := 0
		round.Status = MonitorStatusFailed
		round.ReasonCode = modelSelfCheckProbeReasonNoEligible
		round.FinishedAt = &finishedAt
		round.DurationMs = &duration
		if err := roundRepo.CreateProbeRound(ctx, round); errors.Is(err, ErrModelSelfCheckRoundInProgress) {
			return nil
		} else if err != nil {
			return err
		}
		return nil
	}
	if err := roundRepo.CreateProbeRound(ctx, round); err != nil {
		if errors.Is(err, ErrModelSelfCheckRoundInProgress) {
			return nil
		}
		return fmt.Errorf("create model self check probe round: %w", err)
	}
	defer func() {
		if round.ID == 0 || round.FinishedAt != nil {
			return
		}
		finishedAt := s.now().UTC()
		duration := int(finishedAt.Sub(startedAt).Milliseconds())
		if duration < 0 {
			duration = 0
		}
		round.FinishedAt = &finishedAt
		round.DurationMs = &duration
		markInterruptedProbeSteps(round.Steps)
		if err != nil {
			round.Status = UserModelStatusUnknown
			round.ReasonCode = modelSelfCheckProbeReasonIncomplete
			round.WinnerAccountID = nil
		} else if round.Status == modelSelfCheckProbeRoundStatusChecking {
			round.Status = UserModelStatusUnknown
			round.ReasonCode = modelSelfCheckProbeReasonIncomplete
		}
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer finishCancel()
		if updateErr := roundRepo.UpdateProbeRound(finishCtx, round); updateErr != nil {
			wrapped := fmt.Errorf("finish model self check probe round: %w", updateErr)
			if err != nil {
				err = errors.Join(err, wrapped)
			} else {
				err = wrapped
			}
		}
	}()

	roundCtx, cancel := context.WithTimeout(ctx, modelSelfCheckProbeRoundTimeout)
	defer cancel()
	roundCtx = context.WithValue(roundCtx, modelSelfCheckSessionKey{}, "self-check-"+uuid.NewString())
	var lastResult *ModelSelfCheckProbeResult
	actualFailures := 0
	actualAttempts := 0
	transientFailures := 0
	incompleteAttempts := 0
	for _, candidate := range eligible {
		if roundCtx.Err() != nil {
			markRemainingStepsNotAttempted(round.Steps, candidate.order, modelSelfCheckProbeReasonRoundDeadline)
			break
		}
		step := roundStepByOrder(round.Steps, candidate.order)
		if step == nil {
			continue
		}
		step.Outcome = modelSelfCheckProbeStepOutcomePending
		step.ReasonCode = modelSelfCheckProbeReasonIncomplete
		_ = s.updateProbeRoundProgress(roundCtx, roundRepo, round)
		// Read current membership and eligibility after persistence, immediately
		// before attempting the account, since configuration may change meanwhile.
		fresh, reason, freshErr := s.freshEligibleRoundAccount(roundCtx, target, candidate.account.ID)
		// Persistence is preparation, not an upstream attempt. Do not turn a
		// deadline reached while writing progress into failed probe evidence.
		if roundCtx.Err() != nil {
			step.Outcome = modelSelfCheckProbeStepOutcomeNotAttempted
			markRemainingStepsNotAttempted(round.Steps, candidate.order, modelSelfCheckProbeReasonRoundDeadline)
			break
		}
		if freshErr != nil {
			return freshErr
		}
		if fresh == nil {
			step.Outcome = modelSelfCheckProbeStepOutcomeSkipped
			step.ReasonCode = reason
			continue
		}
		attemptCtx, attemptCancel := context.WithTimeout(roundCtx, modelSelfCheckProbeAttemptTimeout)
		attemptStarted := s.now().UTC()
		step.StartedAt = &attemptStarted
		result := s.probeExecutor.Probe(attemptCtx, fresh, target.Model)
		attemptCancel()
		attemptFinished := s.now().UTC()
		step.FinishedAt = &attemptFinished
		step.LatencyMs = result.LatencyMs
		step.HTTPStatus = result.HTTPStatus
		step.ErrorCode = strings.TrimSpace(result.ErrorCode)
		step.RetryCount = result.RetryCount
		step.InitialHTTPStatus = result.InitialHTTPStatus
		if step.ErrorCode == "" && result.Status != MonitorStatusOperational {
			step.ErrorCode = strings.TrimSpace(result.Status)
		}
		actualAttempts++
		lastResult = &result
		if result.Status == MonitorStatusOperational || result.Recovered {
			step.Outcome = modelSelfCheckProbeStepOutcomeSucceeded
			step.ReasonCode = modelSelfCheckProbeReasonOK
			winnerID := fresh.ID
			round.WinnerAccountID = &winnerID
			if result.Recovered {
				round.Status = MonitorStatusDegraded
				round.ReasonCode = "retry_succeeded"
				step.ReasonCode = "retry_succeeded"
			} else if actualFailures == 0 {
				round.Status = MonitorStatusOperational
				round.ReasonCode = modelSelfCheckProbeReasonOK
			} else {
				round.Status = MonitorStatusDegraded
				round.ReasonCode = modelSelfCheckProbeReasonFallbackOK
			}
			markRemainingStepsNotAttempted(round.Steps, candidate.order+1, modelSelfCheckProbeReasonPriorSuccess)
		} else {
			actualFailures++
			if result.Status == UserModelStatusUnknown {
				incompleteAttempts++
			}
			if result.Transient || result.ErrorCode == modelSelfCheckErrorRateLimit {
				transientFailures++
			}
			step.Outcome = modelSelfCheckProbeStepOutcomeFailed
			if result.Status == UserModelStatusUnknown {
				step.Outcome = modelSelfCheckProbeStepOutcomeIncomplete
			}
			step.ReasonCode = resultReasonCode(result)
		}
		history := &ModelSelfCheckHistory{
			Model:        target.Model,
			AccountID:    fresh.ID,
			Platform:     fresh.Platform,
			Status:       result.Status,
			LatencyMs:    result.LatencyMs,
			HTTPStatus:   result.HTTPStatus,
			ErrorCode:    result.ErrorCode,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			CheckedAt:    attemptFinished,
		}
		historyCtx, historyCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		recordErr := s.RecordHistory(historyCtx, history)
		historyCancel()
		if recordErr != nil {
			return recordErr
		}
		if result.Status == MonitorStatusOperational || result.Recovered {
			_ = s.updateProbeRoundProgress(roundCtx, roundRepo, round)
			break
		}
		_ = s.updateProbeRoundProgress(roundCtx, roundRepo, round)
	}
	finishedAt := s.now().UTC()
	duration := int(finishedAt.Sub(startedAt).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	round.FinishedAt = &finishedAt
	round.DurationMs = &duration
	if round.Status == modelSelfCheckProbeRoundStatusChecking {
		if roundCtx.Err() != nil {
			round.Status = UserModelStatusUnknown
			round.ReasonCode = modelSelfCheckProbeReasonRoundDeadline
		} else {
			round.Status = MonitorStatusFailed
			round.ReasonCode = modelSelfCheckProbeReasonAllFailed
			if actualAttempts == 0 {
				round.ReasonCode = modelSelfCheckProbeReasonNoEligible
			} else if incompleteAttempts > 0 {
				round.Status = UserModelStatusUnknown
				round.ReasonCode = modelSelfCheckProbeReasonIncomplete
			} else if lastResult != nil && lastResult.Status == MonitorStatusDegraded {
				round.Status = MonitorStatusDegraded
				round.ReasonCode = modelSelfCheckErrorRateLimit
			} else if transientFailures == actualAttempts {
				round.ReasonCode = "transient_probe_failed"
				previous := latestRoundForTarget(target, data.latestRounds, startedAt)
				if previous == nil || previous.FinishedAt == nil || previous.ReasonCode != "transient_probe_failed" {
					round.Status = MonitorStatusDegraded
				}
			}
		}
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	if err := roundRepo.UpdateProbeRound(finishCtx, round); err != nil {
		return fmt.Errorf("finish model self check probe round: %w", err)
	}
	return nil
}

func (s *ModelSelfCheckService) GetAdminProbeChain(ctx context.Context, groupID int64, model string) (*ModelSelfCheckChainView, error) {
	model = strings.TrimSpace(model)
	if groupID <= 0 || model == "" {
		return nil, ErrChannelMonitorNotFound
	}
	target, data, err := s.loadSingleTargetStatusData(ctx, groupID, model)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	candidates := s.modelSelfCheckCandidates(ctx, target, data)
	rounds := data.latestRounds
	var latest *ModelSelfCheckProbeRound
	if round, ok := rounds[modelSelfCheckTargetKey{target.GroupID, target.Model}]; ok {
		latest = adminProbeRoundView(round, data.now)
	}
	view := &ModelSelfCheckChainView{
		GroupID:               target.GroupID,
		GroupName:             target.GroupName,
		Model:                 target.Model,
		UpdatedAt:             data.now,
		AttemptTimeoutSeconds: int(modelSelfCheckProbeAttemptTimeout / time.Second),
		RoundTimeoutSeconds:   int(modelSelfCheckProbeRoundTimeout / time.Second),
		Candidates:            chainCandidatesFromRoundCandidates(candidates),
		LatestRound:           latest,
	}
	return view, nil
}

func (s *ModelSelfCheckService) CleanupProbeRoundsWithRetention(ctx context.Context, retentionDays int) (int64, error) {
	roundRepo := s.roundRepo()
	if roundRepo == nil {
		return 0, nil
	}
	retentionDays = clampModelSelfCheckSnapshotRetentionDays(retentionDays)
	if retentionDays == 0 {
		return 0, nil
	}
	before := s.now().UTC().AddDate(0, 0, -retentionDays)
	deleted, err := roundRepo.DeleteProbeRoundsBefore(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup model self check probe rounds: %w", err)
	}
	return deleted, nil
}

func (s *ModelSelfCheckService) updateProbeRoundProgress(ctx context.Context, roundRepo ModelSelfCheckRoundRepository, round *ModelSelfCheckProbeRound) error {
	if roundRepo == nil || round == nil || round.ID == 0 || round.FinishedAt != nil {
		return nil
	}
	updateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return roundRepo.UpdateProbeRound(updateCtx, round)
}

func (s *ModelSelfCheckService) freshEligibleRoundAccount(ctx context.Context, target ModelSelfCheckTarget, accountID int64) (*Account, string, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, modelSelfCheckProbeReasonIneligible, nil
		}
		return nil, "", fmt.Errorf("run model self check probe round: load account %d: %w", accountID, err)
	}
	if account != nil && !slices.Contains(account.GroupIDs, target.GroupID) {
		return nil, modelSelfCheckProbeReasonGroupRemoved, nil
	}
	parentLookup, err := s.selfCheckParentLookup(ctx, account)
	if err != nil {
		return nil, "", err
	}
	if reason := s.selfCheckCandidateReason(ctx, target, account, parentLookup); reason != "" {
		return nil, reason, nil
	}
	return account, modelSelfCheckProbeReasonOK, nil
}

func (s *ModelSelfCheckService) selfCheckCandidateReason(ctx context.Context, target ModelSelfCheckTarget, account *Account, parentLookup func(int64) *Account) string {
	if account == nil {
		return modelSelfCheckProbeReasonIneligible
	}
	if !samePlatform(target.GroupPlatform, account.Platform) {
		return modelSelfCheckProbeReasonPlatform
	}
	if isAccountEligibleForSelfCheck(ctx, account, target.Model, parentLookup) {
		if !s.isModelSupportedBySelfCheckAccount(ctx, account, target.Model) {
			return modelSelfCheckProbeReasonUnsupported
		}
		if !s.accountPassesChannelRestriction(ctx, target, account) {
			return modelSelfCheckProbeReasonChannel
		}
		return ""
	}
	if reason := selfCheckAccountEligibilityReason(ctx, account, target.Model, parentLookup); reason != "" {
		return reason
	}
	if !s.isModelSupportedBySelfCheckAccount(ctx, account, target.Model) {
		return modelSelfCheckProbeReasonUnsupported
	}
	if !s.accountPassesChannelRestriction(ctx, target, account) {
		return modelSelfCheckProbeReasonChannel
	}
	return ""
}

func selfCheckAccountEligibilityReason(ctx context.Context, account *Account, model string, parentLookup func(int64) *Account) string {
	if account == nil {
		return modelSelfCheckProbeReasonIneligible
	}
	now := time.Now()
	if !account.IsActive() {
		return modelSelfCheckProbeReasonDisabled
	}
	if !account.Schedulable {
		return modelSelfCheckProbeReasonNotSchedulable
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return modelSelfCheckProbeReasonExpired
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return modelSelfCheckProbeReasonOverloaded
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return modelSelfCheckProbeReasonRateLimited
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return modelSelfCheckProbeReasonTempUnsched
	}
	if !parentHealthyForShadow(account, parentLookup) {
		return modelSelfCheckProbeReasonParent
	}
	if account.IsAPIKeyOrBedrock() && account.IsQuotaExceeded() {
		return modelSelfCheckProbeReasonQuotaExceeded
	}
	if account.isModelRateLimitedWithContext(ctx, model) {
		return modelSelfCheckProbeReasonModelCooldown
	}
	if !account.IsSchedulableForModelWithContext(ctx, model) {
		return modelSelfCheckProbeReasonIneligible
	}
	return ""
}

func (s *ModelSelfCheckService) loadSingleTargetStatusData(ctx context.Context, groupID int64, model string) (ModelSelfCheckTarget, *modelSelfCheckStatusData, error) {
	data, err := s.loadStatusDataWithHistory(ctx, false, map[int64]struct{}{groupID: {}})
	if err != nil {
		return ModelSelfCheckTarget{}, nil, err
	}
	target, ok := findSelfCheckTarget(data.targets, groupID, model)
	if !ok {
		return ModelSelfCheckTarget{}, nil, nil
	}
	data.targets = []ModelSelfCheckTarget{target}
	return target, data, nil
}

func (s *ModelSelfCheckService) modelSelfCheckCandidates(ctx context.Context, target ModelSelfCheckTarget, data *modelSelfCheckStatusData) []modelSelfCheckRoundCandidate {
	if data == nil || !s.targetAllowsSelfCheckModel(ctx, target) {
		return []modelSelfCheckRoundCandidate{}
	}
	rows := data.accountsByGroup[target.GroupID]
	out := make([]modelSelfCheckRoundCandidate, 0, len(rows))
	seen := map[int64]struct{}{}
	for _, row := range rows {
		if row.AccountID <= 0 {
			continue
		}
		if _, ok := seen[row.AccountID]; ok {
			continue
		}
		seen[row.AccountID] = struct{}{}
		account := data.accountsByID[row.AccountID]
		if account == nil {
			out = append(out, modelSelfCheckRoundCandidate{
				accountID:  row.AccountID,
				reasonCode: modelSelfCheckProbeReasonIneligible,
			})
			continue
		}
		reason := s.selfCheckCandidateReason(ctx, target, account, func(id int64) *Account { return data.accountsByID[id] })
		candidate := modelSelfCheckRoundCandidate{
			accountID:  account.ID,
			account:    account,
			eligible:   reason == "",
			reasonCode: reason,
			last:       latestHistoryForAccount(target.Model, account.ID, data.latestByModel),
		}
		if candidate.reasonCode == "" {
			candidate.reasonCode = modelSelfCheckProbeReasonOK
		}
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i].account, out[j].account
		if left == nil && right == nil {
			return out[i].reasonCode < out[j].reasonCode
		}
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return left.ID < right.ID
	})
	order := 1
	for i := range out {
		if out[i].eligible {
			out[i].order = order
			order++
		}
	}
	return out
}

func (s *ModelSelfCheckService) accountPassesChannelRestriction(ctx context.Context, target ModelSelfCheckTarget, account *Account) bool {
	gateway := s.gatewayServiceForModelSupport()
	if gateway == nil {
		return true
	}
	groupID := target.GroupID
	return !gateway.needsUpstreamChannelRestrictionCheck(ctx, &groupID) ||
		!gateway.isUpstreamModelRestrictedByChannel(ctx, target.GroupID, account, target.Model)
}

func eligibleModelSelfCheckCandidates(candidates []modelSelfCheckRoundCandidate) []modelSelfCheckRoundCandidate {
	out := make([]modelSelfCheckRoundCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.eligible && candidate.account != nil {
			out = append(out, candidate)
		}
	}
	return out
}

func stepsFromRoundCandidates(candidates []modelSelfCheckRoundCandidate) []ModelSelfCheckProbeStep {
	steps := make([]ModelSelfCheckProbeStep, 0, len(candidates))
	for _, candidate := range candidates {
		step := ModelSelfCheckProbeStep{
			Outcome:    modelSelfCheckProbeStepOutcomeSkipped,
			ReasonCode: candidate.reasonCode,
		}
		if candidate.account != nil {
			step.AccountID = candidate.account.ID
			step.AccountName = candidate.account.Name
			step.Priority = candidate.account.Priority
			step.Platform = candidate.account.Platform
		} else {
			step.AccountID = candidate.accountID
		}
		if candidate.eligible {
			step.Order = candidate.order
			step.Outcome = modelSelfCheckProbeStepOutcomeNotAttempted
			step.ReasonCode = modelSelfCheckProbeReasonIncomplete
		}
		steps = append(steps, step)
	}
	return steps
}

func chainCandidatesFromRoundCandidates(candidates []modelSelfCheckRoundCandidate) []ModelSelfCheckChainCandidate {
	out := make([]ModelSelfCheckChainCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		row := ModelSelfCheckChainCandidate{
			Order:      candidate.order,
			Eligible:   candidate.eligible,
			ReasonCode: candidate.reasonCode,
		}
		if candidate.account != nil {
			row.AccountID = candidate.account.ID
			row.AccountName = candidate.account.Name
			row.Priority = candidate.account.Priority
			row.Platform = candidate.account.Platform
		} else {
			row.AccountID = candidate.accountID
		}
		if candidate.last != nil {
			checkedAt := candidate.last.CheckedAt.UTC()
			row.LastCheckedAt = &checkedAt
			row.LastStatus = candidate.last.Status
		}
		out = append(out, row)
	}
	return out
}

func markRemainingStepsNotAttempted(steps []ModelSelfCheckProbeStep, startingOrder int, reason string) {
	for i := range steps {
		if steps[i].Order < startingOrder || steps[i].Outcome != modelSelfCheckProbeStepOutcomeNotAttempted {
			continue
		}
		steps[i].ReasonCode = reason
	}
}

func roundStepByOrder(steps []ModelSelfCheckProbeStep, order int) *ModelSelfCheckProbeStep {
	for i := range steps {
		if steps[i].Order == order {
			return &steps[i]
		}
	}
	return nil
}

func resultReasonCode(result ModelSelfCheckProbeResult) string {
	if strings.TrimSpace(result.ErrorCode) != "" {
		return strings.TrimSpace(result.ErrorCode)
	}
	switch result.Status {
	case MonitorStatusDegraded:
		return modelSelfCheckErrorRateLimit
	case MonitorStatusFailed, MonitorStatusError:
		return modelSelfCheckProbeReasonAllFailed
	default:
		return strings.TrimSpace(result.Status)
	}
}

func latestHistoryForAccount(model string, accountID int64, latestByModel map[string]map[int64]*ModelSelfCheckHistory) *ModelSelfCheckHistory {
	if latestByModel == nil {
		return nil
	}
	if row := latestByModel[model][accountID]; row != nil {
		return row
	}
	return nil
}

func latestRoundForTarget(target ModelSelfCheckTarget, rounds map[modelSelfCheckTargetKey]ModelSelfCheckProbeRound, now time.Time) *ModelSelfCheckProbeRound {
	if rounds == nil {
		return nil
	}
	round, ok := rounds[modelSelfCheckTargetKey{target.GroupID, target.Model}]
	if !ok {
		return nil
	}
	if round.FinishedAt == nil {
		return nil
	}
	if round.FinishedAt.After(now) {
		return nil
	}
	if now.Sub(*round.FinishedAt) <= modelSelfCheckFreshWindow {
		return &round
	}
	return nil
}

func adminProbeRoundView(round ModelSelfCheckProbeRound, now time.Time) *ModelSelfCheckProbeRound {
	cp := cloneModelSelfCheckProbeRound(&round)
	if cp.FinishedAt == nil {
		cp.Status = modelSelfCheckProbeRoundStatusChecking
		cp.ReasonCode = modelSelfCheckProbeReasonIncomplete
		if cp.StartedAt.After(now) || now.Sub(cp.StartedAt) > modelSelfCheckProbeRoundTimeout {
			cp.Status = UserModelStatusUnknown
			markInterruptedProbeSteps(cp.Steps)
		}
	}
	return &cp
}

func markInterruptedProbeSteps(steps []ModelSelfCheckProbeStep) {
	for i := range steps {
		if steps[i].Outcome == modelSelfCheckProbeStepOutcomePending {
			steps[i].Outcome = modelSelfCheckProbeStepOutcomeIncomplete
			steps[i].ReasonCode = modelSelfCheckProbeReasonIncomplete
		}
	}
}

func cloneModelSelfCheckProbeRound(round *ModelSelfCheckProbeRound) ModelSelfCheckProbeRound {
	if round == nil {
		return ModelSelfCheckProbeRound{}
	}
	cp := *round
	cp.Steps = append([]ModelSelfCheckProbeStep(nil), round.Steps...)
	return cp
}

func (s *ModelSelfCheckService) currentStatusFromLatestRound(target ModelSelfCheckTarget, data *modelSelfCheckStatusData, accountIDs []int64) (string, string, *int, *time.Time) {
	if len(accountIDs) == 0 {
		return MonitorStatusFailed, modelSelfCheckSnapshotReasonNoAvailableAccount, nil, nil
	}
	rawRound, hasRawRound := data.latestRounds[modelSelfCheckTargetKey{target.GroupID, target.Model}]
	if hasRawRound {
		if rawRound.FinishedAt == nil {
			checkedAt := rawRound.StartedAt.UTC()
			if rawRound.StartedAt.After(data.now) || data.now.Sub(rawRound.StartedAt) > modelSelfCheckProbeRoundTimeout {
				return UserModelStatusUnknown, modelSelfCheckProbeReasonIncomplete, nil, &checkedAt
			}
			return UserModelStatusUnknown, "checking", nil, &checkedAt
		}
		if rawRound.FinishedAt.After(data.now) {
			return UserModelStatusUnknown, modelSelfCheckSnapshotReasonNoFreshProbe, nil, nil
		}
		if data.now.Sub(*rawRound.FinishedAt) > modelSelfCheckFreshWindow {
			checkedAt := rawRound.FinishedAt.UTC()
			return UserModelStatusUnknown, "stale_probe", nil, &checkedAt
		}
	}
	round := latestRoundForTarget(target, data.latestRounds, data.now)
	if round == nil || !roundWinnerCurrentlyEligible(round, accountIDs) {
		return UserModelStatusUnknown, modelSelfCheckSnapshotReasonNoFreshProbe, nil, nil
	}
	checkedAt := round.StartedAt.UTC()
	if round.FinishedAt != nil {
		checkedAt = round.FinishedAt.UTC()
	}
	status := round.Status
	reasonCode := round.ReasonCode
	if status == modelSelfCheckProbeRoundStatusChecking {
		status = UserModelStatusUnknown
		reasonCode = modelSelfCheckProbeReasonIncomplete
	}
	return status, userSafeModelStatusReasonCode(status, reasonCode), winningRoundLatency(round), &checkedAt
}

func roundWinnerCurrentlyEligible(round *ModelSelfCheckProbeRound, accountIDs []int64) bool {
	if round == nil {
		return false
	}
	if round.WinnerAccountID == nil {
		return true
	}
	for _, accountID := range accountIDs {
		if accountID == *round.WinnerAccountID {
			return true
		}
	}
	return false
}

func snapshotFromLatestRound(target ModelSelfCheckTarget, round *ModelSelfCheckProbeRound, eligibleAccounts int) *ModelSelfCheckStatusSnapshot {
	if round == nil {
		return nil
	}
	checkedAt := round.StartedAt
	if round.FinishedAt != nil {
		checkedAt = *round.FinishedAt
	}
	snapshot := &ModelSelfCheckStatusSnapshot{
		GroupID:              target.GroupID,
		Model:                target.Model,
		Status:               round.Status,
		ReasonCode:           round.ReasonCode,
		EligibleAccountCount: eligibleAccounts,
		CheckedAt:            checkedAt,
		LatencyMs:            winningRoundLatency(round),
	}
	if snapshot.Status == modelSelfCheckProbeRoundStatusChecking {
		snapshot.Status = UserModelStatusUnknown
		snapshot.ReasonCode = modelSelfCheckProbeReasonIncomplete
	}
	for _, step := range round.Steps {
		if step.Order <= 0 || (step.StartedAt == nil && step.FinishedAt == nil) {
			continue
		}
		snapshot.CheckedAccountCount++
		switch step.Outcome {
		case modelSelfCheckProbeStepOutcomeSucceeded:
			snapshot.OperationalAccountCount++
		case modelSelfCheckProbeStepOutcomeFailed:
			snapshot.FailedAccountCount++
		}
	}
	return snapshot
}

func winningRoundLatency(round *ModelSelfCheckProbeRound) *int {
	if round == nil {
		return nil
	}
	for _, step := range round.Steps {
		if step.Outcome == modelSelfCheckProbeStepOutcomeSucceeded && step.LatencyMs != nil {
			value := *step.LatencyMs
			return &value
		}
	}
	return nil
}
