package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	OpsRoutingAttemptsKey = "ops_routing_attempts"

	OpsRoutingAttemptStagePlannedCandidate = "planned_candidate"
	OpsRoutingAttemptStagePrunedCandidate  = "pruned_candidate"
	OpsRoutingAttemptStageActualAttempt    = "actual_attempt"

	OpsRoutingAttemptOutcomePlanned  = "planned"
	OpsRoutingAttemptOutcomePruned   = "pruned"
	OpsRoutingAttemptOutcomeSelected = "selected"
)

const opsRoutingAttemptsMax = 48

const UsageShadowDiffLegacySuccessNewPruned = "legacy_success_new_pruned"

type usageRoutingShadowEvidenceContextKey struct{}

// UsageRoutingShadowEvidence is request-scoped, low-sensitivity evidence used
// only to annotate successful administrator usage logs during shadow rollout.
// It intentionally carries group IDs only in memory; persisted usage metadata
// stores bounded counts and closed reason codes instead of topology.
type UsageRoutingShadowEvidence struct {
	Mode             EnterpriseMemberModelAdmissionMode
	PlanSource       EnterpriseMemberRoutePlanSource
	Model            string
	LegacyGroupIDs   []int64
	PlannedGroupIDs  []int64
	Rejected         []UsageRoutingShadowRejection
	EvaluationError  bool
	PlannerLatencyMs int64
}

type UsageRoutingShadowRejection struct {
	GroupID int64
	Reason  EnterpriseMemberRouteReasonCode
}

// OpsRoutingAttemptEvidence is a bounded, non-sensitive routing evidence row.
// It intentionally stores group routing facts separately from model ownership
// facts so a final failed group cannot be misread as the model's publisher.
type OpsRoutingAttemptEvidence struct {
	Stage             string `json:"stage"`
	Outcome           string `json:"outcome,omitempty"`
	GroupID           int64  `json:"group_id,omitempty"`
	ModelOwnerGroupID int64  `json:"model_owner_group_id,omitempty"`
	AttemptNumber     int    `json:"attempt_number,omitempty"`
	CandidateIndex    int    `json:"candidate_index,omitempty"`
	Platform          string `json:"platform,omitempty"`
	RequestedModel    string `json:"requested_model,omitempty"`
	MappedModel       string `json:"mapped_model,omitempty"`
	Reason            string `json:"reason,omitempty"`
	SafeToReplay      *bool  `json:"safe_to_replay,omitempty"`
	ResponseCommitted *bool  `json:"response_committed,omitempty"`
	OutcomeUnknown    *bool  `json:"outcome_unknown,omitempty"`
	UnsafeReason      string `json:"unsafe_reason,omitempty"`
}

func WithUsageRoutingShadowEvidence(ctx context.Context, evidence UsageRoutingShadowEvidence) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	copyEvidence := evidence
	copyEvidence.LegacyGroupIDs = append([]int64(nil), evidence.LegacyGroupIDs...)
	copyEvidence.PlannedGroupIDs = append([]int64(nil), evidence.PlannedGroupIDs...)
	copyEvidence.Rejected = append([]UsageRoutingShadowRejection(nil), evidence.Rejected...)
	return context.WithValue(ctx, usageRoutingShadowEvidenceContextKey{}, &copyEvidence)
}

func UsageRoutingShadowEvidenceFromContext(ctx context.Context) (UsageRoutingShadowEvidence, bool) {
	if ctx == nil {
		return UsageRoutingShadowEvidence{}, false
	}
	value, ok := ctx.Value(usageRoutingShadowEvidenceContextKey{}).(*UsageRoutingShadowEvidence)
	if !ok || value == nil {
		return UsageRoutingShadowEvidence{}, false
	}
	copyEvidence := *value
	copyEvidence.LegacyGroupIDs = append([]int64(nil), value.LegacyGroupIDs...)
	copyEvidence.PlannedGroupIDs = append([]int64(nil), value.PlannedGroupIDs...)
	copyEvidence.Rejected = append([]UsageRoutingShadowRejection(nil), value.Rejected...)
	return copyEvidence, true
}

func ApplyUsageRoutingPlanEvidence(ctx context.Context, usage *UsageLog) {
	if usage == nil {
		return
	}
	active, ok := ActiveGroupFromContext(ctx)
	if !ok || active == nil || !active.ModelPlanApplied {
		return
	}
	source := strings.TrimSpace(string(active.RoutePlanSource))
	if source == "" {
		return
	}
	usage.RoutePlanSource = source
	usage.RoutePlanSnapshotAgeMs = active.RoutePlanSnapshotAgeMs
}

func ApplyUsageRoutingShadowSuccessEvidence(ctx context.Context, usage *UsageLog) {
	if usage == nil || usage.EffectiveRequestType() == RequestTypeCyberBlocked {
		return
	}
	active, ok := ActiveGroupFromContext(ctx)
	if !ok || active == nil || active.MemberID == 0 {
		return
	}
	evidence, ok := UsageRoutingShadowEvidenceFromContext(ctx)
	if !ok || evidence.Mode != EnterpriseMemberModelAdmissionShadowPublished || evidence.EvaluationError {
		return
	}
	groupPruned := usageRoutingShadowGroupPruned(active.GroupID, evidence.PlannedGroupIDs)
	groupKept := active.GroupID > 0 && !groupPruned
	if !groupKept && !groupPruned {
		return
	}
	if usage.ScheduleMeta == nil {
		usage.ScheduleMeta = &UsageScheduleMeta{}
	}
	usage.ScheduleMeta.ShadowPlanEvaluated = true
	usage.ScheduleMeta.ShadowGroupKept = groupKept
	usage.ScheduleMeta.ShadowEvaluationError = evidence.EvaluationError
	usage.ScheduleMeta.ShadowPlannerLatencyMs = evidence.PlannerLatencyMs
	usage.ScheduleMeta.ShadowPlanSource = strings.TrimSpace(string(evidence.PlanSource))
	usage.ScheduleMeta.ShadowLegacyGroups = len(evidence.LegacyGroupIDs)
	usage.ScheduleMeta.ShadowPlannedGroups = len(evidence.PlannedGroupIDs)
	usage.ScheduleMeta.ShadowPrunedGroups = usageRoutingShadowPrunedGroupCount(evidence.LegacyGroupIDs, evidence.PlannedGroupIDs)
	if groupPruned {
		reasonCodes := usageRoutingShadowReasonCodesForGroup(active.GroupID, evidence.Rejected)
		if len(reasonCodes) == 0 {
			return
		}
		usage.ScheduleMeta.ShadowDiffType = UsageShadowDiffLegacySuccessNewPruned
		usage.ScheduleMeta.ShadowReasonCodes = reasonCodes
	}
}

func usageRoutingShadowGroupPruned(groupID int64, planned []int64) bool {
	if groupID <= 0 || len(planned) == 0 {
		return false
	}
	for _, plannedGroupID := range planned {
		if plannedGroupID == groupID {
			return false
		}
	}
	return true
}

func usageRoutingShadowPrunedGroupCount(legacy, planned []int64) int {
	if len(legacy) == 0 {
		return 0
	}
	plannedSet := make(map[int64]struct{}, len(planned))
	for _, groupID := range planned {
		if groupID > 0 {
			plannedSet[groupID] = struct{}{}
		}
	}
	pruned := 0
	for _, groupID := range legacy {
		if groupID <= 0 {
			continue
		}
		if _, ok := plannedSet[groupID]; !ok {
			pruned++
		}
	}
	return pruned
}

func usageRoutingShadowReasonCodesForGroup(groupID int64, rejected []UsageRoutingShadowRejection) []string {
	if groupID <= 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rejected))
	reasons := make([]string, 0, len(rejected))
	for _, item := range rejected {
		if item.GroupID != groupID {
			continue
		}
		reason := strings.TrimSpace(string(item.Reason))
		if !validEnterpriseMemberRouteReasonCode(item.Reason) || reason == string(EnterpriseMemberRouteReasonEligible) {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		reasons = append(reasons, reason)
	}
	return reasons
}

func validEnterpriseMemberRouteReasonCode(reason EnterpriseMemberRouteReasonCode) bool {
	switch reason {
	case EnterpriseMemberRouteReasonEligible,
		EnterpriseMemberRouteReasonModelUnpublished,
		EnterpriseMemberRouteReasonModelUnsupported,
		EnterpriseMemberRouteReasonEndpointCapability,
		EnterpriseMemberRouteReasonNoPersistentPool,
		EnterpriseMemberRouteReasonEvaluationFailed:
		return true
	default:
		return false
	}
}

func SanitizeOpsRoutingAttemptsForQueue(entry *OpsInsertErrorLogInput) error {
	return sanitizeOpsRoutingAttempts(entry)
}

func AppendOpsRoutingAttempts(c *gin.Context, attempts ...*OpsRoutingAttemptEvidence) {
	if c == nil || len(attempts) == 0 {
		return
	}
	existing := OpsRoutingAttemptsFromContext(c)
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		if item := sanitizeOpsRoutingAttemptEvidence(*attempt); item != nil {
			existing = upsertOpsRoutingAttempt(existing, item)
		}
	}
	if len(existing) > opsRoutingAttemptsMax {
		existing = existing[len(existing)-opsRoutingAttemptsMax:]
	}
	c.Set(OpsRoutingAttemptsKey, existing)
}

func AppendCurrentOpsRoutingAttempt(c *gin.Context, result GroupAttemptResult) {
	if c == nil {
		return
	}
	var actual *OpsRoutingAttemptEvidence
	if c.Request != nil {
		if active, ok := ActiveGroupFromContext(c.Request.Context()); ok && active != nil {
			actual = &OpsRoutingAttemptEvidence{
				Stage:          OpsRoutingAttemptStageActualAttempt,
				Outcome:        OpsRoutingAttemptOutcomeSelected,
				GroupID:        active.GroupID,
				AttemptNumber:  active.AttemptNumber,
				CandidateIndex: active.CandidateIndex,
				Platform:       active.Platform,
				RequestedModel: active.RequestedModel,
				MappedModel:    active.MappedModel,
			}
		}
	}
	if actual == nil && result.GroupID > 0 {
		actual = &OpsRoutingAttemptEvidence{
			Stage:   OpsRoutingAttemptStageActualAttempt,
			Outcome: OpsRoutingAttemptOutcomeSelected,
			GroupID: result.GroupID,
		}
	}
	if actual == nil {
		return
	}
	if result.Valid() {
		actual.Outcome = string(result.Outcome)
		actual.Reason = string(result.Reason)
		actual.SafeToReplay = opsRoutingEvidenceBoolPtr(result.SafeToReplay)
		actual.ResponseCommitted = opsRoutingEvidenceBoolPtr(result.ResponseCommitted)
		actual.OutcomeUnknown = opsRoutingEvidenceBoolPtr(result.OutcomeUnknown)
		actual.UnsafeReason = string(result.UnsafeReason)
		if actual.GroupID == 0 {
			actual.GroupID = result.GroupID
		}
		if actual.AttemptNumber == 0 {
			actual.AttemptNumber = result.AttemptNumber
		}
	}
	AppendOpsRoutingAttempts(c, actual)
}

func OpsRoutingAttemptsFromContext(c *gin.Context) []*OpsRoutingAttemptEvidence {
	if c == nil {
		return nil
	}
	value, ok := c.Get(OpsRoutingAttemptsKey)
	if !ok {
		return nil
	}
	out := opsRoutingAttemptsFromValue(value)
	if len(out) > opsRoutingAttemptsMax {
		out = out[len(out)-opsRoutingAttemptsMax:]
	}
	return out
}

func opsRoutingAttemptsFromValue(value any) []*OpsRoutingAttemptEvidence {
	var out []*OpsRoutingAttemptEvidence
	switch attempts := value.(type) {
	case []*OpsRoutingAttemptEvidence:
		out = make([]*OpsRoutingAttemptEvidence, 0, len(attempts))
		for _, attempt := range attempts {
			if attempt == nil {
				continue
			}
			if item := sanitizeOpsRoutingAttemptEvidence(*attempt); item != nil {
				out = append(out, item)
			}
		}
	case []OpsRoutingAttemptEvidence:
		out = make([]*OpsRoutingAttemptEvidence, 0, len(attempts))
		for _, attempt := range attempts {
			if item := sanitizeOpsRoutingAttemptEvidence(attempt); item != nil {
				out = append(out, item)
			}
		}
	case []string:
		out = make([]*OpsRoutingAttemptEvidence, 0, len(attempts))
		for _, raw := range attempts {
			if item := legacyOpsRoutingAttempt(raw); item != nil {
				out = append(out, item)
			}
		}
	}
	return out
}

func upsertOpsRoutingAttempt(existing []*OpsRoutingAttemptEvidence, item *OpsRoutingAttemptEvidence) []*OpsRoutingAttemptEvidence {
	if item == nil {
		return existing
	}
	for i, current := range existing {
		if sameOpsRoutingAttempt(current, item) {
			existing[i] = item
			return existing
		}
	}
	return append(existing, item)
}

func sameOpsRoutingAttempt(a, b *OpsRoutingAttemptEvidence) bool {
	if a == nil || b == nil || a.Stage != b.Stage || a.GroupID <= 0 || a.GroupID != b.GroupID {
		return false
	}
	if a.AttemptNumber > 0 && b.AttemptNumber > 0 && a.AttemptNumber != b.AttemptNumber {
		return false
	}
	return true
}

func sanitizeOpsRoutingAttempts(entry *OpsInsertErrorLogInput) error {
	if entry == nil || len(entry.RoutingAttempts) == 0 {
		return nil
	}

	attempts := entry.RoutingAttempts
	if len(attempts) > opsRoutingAttemptsMax {
		attempts = attempts[len(attempts)-opsRoutingAttemptsMax:]
	}

	out := make([]*OpsRoutingAttemptEvidence, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		if item := sanitizeOpsRoutingAttemptEvidence(*attempt); item != nil {
			out = append(out, item)
		}
	}

	entry.RoutingAttemptsJSON = marshalOpsRoutingAttempts(out)
	entry.RoutingAttempts = nil
	return nil
}

func sanitizeOpsRoutingAttemptEvidence(attempt OpsRoutingAttemptEvidence) *OpsRoutingAttemptEvidence {
	attempt.Stage = opsRoutingEvidenceStage(attempt.Stage)
	if attempt.Stage == "" {
		return nil
	}
	attempt.Outcome = truncateString(opsRoutingEvidenceFirstToken(attempt.Outcome), 64)
	attempt.Platform = truncateString(opsRoutingEvidenceFirstToken(attempt.Platform), 32)
	attempt.RequestedModel = truncateString(opsRoutingEvidenceModel(attempt.RequestedModel), 256)
	attempt.MappedModel = truncateString(opsRoutingEvidenceModel(attempt.MappedModel), 256)
	attempt.Reason = truncateString(opsRoutingEvidenceFirstToken(attempt.Reason), 128)
	attempt.UnsafeReason = truncateString(opsRoutingEvidenceFirstToken(attempt.UnsafeReason), 128)
	if attempt.GroupID < 0 {
		attempt.GroupID = 0
	}
	if attempt.ModelOwnerGroupID < 0 {
		attempt.ModelOwnerGroupID = 0
	}
	if attempt.AttemptNumber < 0 {
		attempt.AttemptNumber = 0
	}
	if attempt.CandidateIndex < 0 {
		attempt.CandidateIndex = 0
	}
	itemCopy := attempt
	return &itemCopy
}

func legacyOpsRoutingAttempt(raw string) *OpsRoutingAttemptEvidence {
	raw = strings.TrimSpace(raw)
	stage, tail, ok := strings.Cut(raw, ":")
	if !ok {
		return nil
	}
	item := OpsRoutingAttemptEvidence{Stage: stage}
	if strings.HasPrefix(tail, "g") {
		for _, r := range tail[1:] {
			if r < '0' || r > '9' {
				break
			}
			item.GroupID = item.GroupID*10 + int64(r-'0')
		}
	}
	return sanitizeOpsRoutingAttemptEvidence(item)
}

func opsRoutingEvidenceStage(stage string) string {
	switch strings.TrimSpace(stage) {
	case OpsRoutingAttemptStagePlannedCandidate:
		return OpsRoutingAttemptStagePlannedCandidate
	case OpsRoutingAttemptStagePrunedCandidate:
		return OpsRoutingAttemptStagePrunedCandidate
	case OpsRoutingAttemptStageActualAttempt:
		return OpsRoutingAttemptStageActualAttempt
	default:
		return ""
	}
}

func opsRoutingEvidenceToken(value string) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == ' ' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			_, _ = b.WriteRune(r)
		} else if r < 0x20 || r == 0x7f {
			_, _ = b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

func opsRoutingEvidenceFirstToken(value string) string {
	fields := strings.Fields(opsRoutingEvidenceToken(value))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func opsRoutingEvidenceModel(value string) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			continue
		}
		_, _ = b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func opsRoutingEvidenceBoolPtr(value bool) *bool {
	return &value
}

func marshalOpsRoutingAttempts(attempts []*OpsRoutingAttemptEvidence) *string {
	if len(attempts) == 0 {
		return nil
	}
	raw, err := json.Marshal(attempts)
	if err != nil || len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}

func ParseOpsRoutingAttempts(raw string) ([]*OpsRoutingAttemptEvidence, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []*OpsRoutingAttemptEvidence{}, nil
	}
	var out []*OpsRoutingAttemptEvidence
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
