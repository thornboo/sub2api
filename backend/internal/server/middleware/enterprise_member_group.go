package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	enterpriseMemberGroupPlanKey        = "enterprise_member_group_plan"
	enterpriseMemberRouteShadowTraceKey = "enterprise_member_route_shadow_trace"
	enterpriseMemberRouteTraceModelMax  = 256
)

type enterpriseMemberGroupCandidate struct {
	group                  service.Group
	subscription           *service.UserSubscription
	memberIndex            int
	routePlanSnapshotAgeMs *int64
}

type enterpriseMemberGroupPlan struct {
	apiKey           *service.APIKey
	legacyCandidates []enterpriseMemberGroupCandidate
	candidates       []enterpriseMemberGroupCandidate
	current          int
	planner          service.EnterpriseMemberRoutePlanningService
	admissionMode    service.EnterpriseMemberModelAdmissionMode
	routePlanSource  service.EnterpriseMemberRoutePlanSource
	modelPlanApplied bool
}

// EnterpriseMemberRouteShadowTrace is request-scoped administrator evidence.
// It contains the bounded public model identifier, group IDs and closed reason
// codes; request bodies, keys, credentials, account IDs and mapped upstream
// models are deliberately absent.
type EnterpriseMemberRouteShadowTrace struct {
	Mode             service.EnterpriseMemberModelAdmissionMode
	PlanSource       service.EnterpriseMemberRoutePlanSource
	Model            string
	LegacyGroupIDs   []int64
	PlannedGroupIDs  []int64
	Rejected         []EnterpriseMemberRouteShadowRejection
	EvaluationError  bool
	PlannerLatencyMs int64
}

type EnterpriseMemberRouteShadowRejection struct {
	GroupID int64
	Reason  service.EnterpriseMemberRouteReasonCode
}

// ResolveEnterpriseMemberGroup builds the ordered, request-local candidate
// plan and activates its first group. It never mutates the cached API key.
func ResolveEnterpriseMemberGroup(
	subscriptionService *service.SubscriptionService,
	planner service.EnterpriseMemberRoutePlanningService,
	admissionSettings service.EnterpriseMemberModelAdmissionSettingReader,
	cfg *config.Config,
	writeError GatewayErrorWriter,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.MemberID == nil {
			c.Next()
			return
		}
		if _, message, valid := validateEnterpriseMemberAPIKey(apiKey); !valid {
			writeError(c, http.StatusForbidden, message)
			c.Abort()
			return
		}

		requestedModel, requestBody, err := extractEnterpriseMemberRouteRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Unable to parse request model")
			c.Abort()
			return
		}
		if requestedModel != "" {
			ctx := context.WithValue(c.Request.Context(), ctxkey.Model, requestedModel)
			c.Request = c.Request.WithContext(ctx)
		}

		plan := &enterpriseMemberGroupPlan{
			apiKey:        apiKey,
			current:       -1,
			planner:       planner,
			admissionMode: enterpriseMemberModelAdmissionMode(c.Request.Context(), admissionSettings, apiKey),
		}
		for i := range apiKey.Member.Groups {
			candidate := &apiKey.Member.Groups[i]
			var selectedSubscription *service.UserSubscription
			if !enterpriseMemberGroupEligible(c, apiKey.User, candidate) {
				continue
			}
			if cfg == nil || cfg.RunMode != config.RunModeSimple {
				if candidate.IsSubscriptionType() {
					if subscriptionService == nil {
						continue
					}
					subscription, subErr := subscriptionService.GetActiveSubscription(c.Request.Context(), apiKey.UserID, candidate.ID)
					if subErr != nil {
						continue
					}
					needsMaintenance, validateErr := subscriptionService.ValidateAndCheckLimits(subscription, candidate)
					if needsMaintenance {
						subscription, subErr = subscriptionService.EnsureWindowMaintenance(c.Request.Context(), subscription)
						if subErr != nil {
							continue
						}
						_, validateErr = subscriptionService.ValidateAndCheckLimits(subscription, candidate)
					}
					if validateErr != nil {
						continue
					}
					selectedSubscription = subscription
				} else if apiKey.User.Balance <= 0 {
					continue
				}
			}
			plan.legacyCandidates = append(plan.legacyCandidates, enterpriseMemberGroupCandidate{
				group:        *candidate,
				subscription: selectedSubscription,
				memberIndex:  i,
			})
			selectedSubscription = nil
		}
		if len(plan.legacyCandidates) == 0 {
			service.RecordEnterpriseMemberRoutingPlan(0)
			writeError(c, http.StatusForbidden, "No authorized enterprise member group can serve this endpoint or model")
			c.Abort()
			return
		}
		plan.candidates = append([]enterpriseMemberGroupCandidate(nil), plan.legacyCandidates...)
		if requestedModel != "" &&
			service.SupportsEnterpriseMemberRoutePlanning(c.Request.URL.Path) &&
			plan.admissionMode != service.EnterpriseMemberModelAdmissionLegacyOrderOnly {
			routePlan, planErr := evaluateEnterpriseMemberRoutePlan(c, plan, requestedModel, requestBody)
			if plan.admissionMode == service.EnterpriseMemberModelAdmissionEnforcePublished {
				if planErr != nil {
					writeEnterpriseMemberRoutePlanError(c, writeError, http.StatusServiceUnavailable, "ROUTING_ELIGIBILITY_UNAVAILABLE", "Model routing eligibility is temporarily unavailable")
					return
				}
				plan.candidates = selectEnterpriseMemberRouteCandidates(plan.legacyCandidates, routePlan.Candidates)
				if len(plan.candidates) == 0 {
					status, code, message := aggregateEnterpriseMemberRoutePlanFailure(routePlan)
					writeEnterpriseMemberRoutePlanError(c, writeError, status, code, message)
					return
				}
				plan.modelPlanApplied = true
			}
		}
		service.RecordEnterpriseMemberRoutingPlan(len(plan.candidates))

		c.Set(enterpriseMemberGroupPlanKey, plan)
		activateEnterpriseMemberGroupCandidate(c, plan, 0, requestedModel)
		c.Next()
	}
}

func activateEnterpriseMemberGroupCandidate(c *gin.Context, plan *enterpriseMemberGroupPlan, candidateIndex int, requestedModel string) {
	candidate := &plan.candidates[candidateIndex]
	plan.current = candidateIndex
	service.RecordEnterpriseMemberRoutingActivation(candidateIndex > 0)
	requestKey := *plan.apiKey
	requestGroup := candidate.group
	requestMember := *plan.apiKey.Member
	requestMember.Groups = make([]service.Group, 0, len(plan.candidates))
	requestMember.GroupIDs = make([]int64, 0, len(plan.candidates))
	for i := range plan.candidates {
		group := plan.candidates[i].group
		requestMember.Groups = append(requestMember.Groups, group)
		requestMember.GroupIDs = append(requestMember.GroupIDs, group.ID)
	}
	requestKey.Member = &requestMember
	requestKey.GroupID = &requestGroup.ID
	requestKey.Group = &requestGroup
	c.Set(string(ContextKeyAPIKey), &requestKey)
	SetOpsFallbackAPIKey(c, &requestKey)
	c.Set(string(ContextKeySubscription), candidate.subscription)
	setGroupContext(c, &requestGroup)

	logicalRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
	active := &service.ActiveGroupContext{
		LogicalRequestID:       logicalRequestID,
		AttemptID:              fmt.Sprintf("%s:g%d:a%d", logicalRequestID, requestGroup.ID, candidateIndex+1),
		MemberID:               plan.apiKey.Member.ID,
		MemberVersion:          plan.apiKey.Member.Version,
		GroupID:                requestGroup.ID,
		Platform:               requestGroup.Platform,
		RateMultiplier:         requestGroup.RateMultiplier,
		SubscriptionType:       requestGroup.SubscriptionType,
		Endpoint:               c.Request.URL.Path,
		RequestedModel:         requestedModel,
		MappedModel:            requestedModel,
		CandidateIndex:         candidate.memberIndex,
		AttemptNumber:          candidateIndex + 1,
		RoutePlanMode:          plan.admissionMode,
		RoutePlanSource:        plan.routePlanSource,
		RoutePlanSnapshotAgeMs: candidate.routePlanSnapshotAgeMs,
		ModelPlanApplied:       plan.modelPlanApplied,
	}
	ctx := service.WithoutCompositeRouteDecision(c.Request.Context())
	ctx = context.WithValue(ctx, ctxkey.ActiveGroup, active)
	c.Request = c.Request.WithContext(ctx)
}

// GetEnterpriseMemberCandidateGroups returns defensive copies of the ordered,
// request-authorized groups for member-aware discovery endpoints.
func GetEnterpriseMemberCandidateGroups(c *gin.Context) []service.Group {
	plan, ok := enterpriseMemberGroupPlanFromContext(c)
	if !ok {
		return nil
	}
	groups := make([]service.Group, 0, len(plan.candidates))
	for i := range plan.candidates {
		groups = append(groups, plan.candidates[i].group)
	}
	return groups
}

func enterpriseMemberGroupEligible(c *gin.Context, user *service.User, group *service.Group) bool {
	if user == nil || group == nil || !group.IsActive() || !service.IsGroupContextValid(group) {
		return false
	}
	if group.ClaudeCodeOnly && (c == nil || c.Request == nil || !service.IsClaudeCodeClient(c.Request.Context())) {
		return false
	}
	if !group.IsSubscriptionType() && !user.CanBindGroup(group.ID, group.IsExclusive) {
		return false
	}
	if forced, ok := GetForcePlatformFromContext(c); ok && forced != "" && group.Platform != forced {
		return false
	}
	requestPath := c.Request.URL.Path
	switch {
	case enterpriseMemberLiveEndpoint(c.Request.Method, requestPath):
		if !group.AllowLive || (group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformComposite) {
			return false
		}
	case strings.Contains(requestPath, "/backend-api/codex/") || (strings.HasSuffix(requestPath, "/models") && c.Query("client_version") != ""):
		if group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformComposite {
			return false
		}
	case strings.Contains(requestPath, "/v1beta/"):
		if group.Platform != service.PlatformGemini && group.Platform != service.PlatformAntigravity && group.Platform != service.PlatformComposite {
			return false
		}
	case strings.HasSuffix(requestPath, "/embeddings"):
		if group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformComposite {
			return false
		}
	case strings.HasSuffix(requestPath, "/alpha/search"):
		if group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformComposite {
			return false
		}
	case strings.Contains(requestPath, "/videos/"):
		if group.Platform != service.PlatformComposite && (group.Platform != service.PlatformGrok || !service.GroupAllowsImageGeneration(group)) {
			return false
		}
	case strings.Contains(requestPath, "/images/batches"):
		if group.Platform != service.PlatformComposite && (group.Platform != service.PlatformGemini || !group.AllowImageGeneration || !group.AllowBatchImageGeneration) {
			return false
		}
	case strings.Contains(requestPath, "/images/"):
		if group.Platform != service.PlatformComposite && ((group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformGrok) || !service.GroupAllowsImageGeneration(group)) {
			return false
		}
	case strings.HasSuffix(requestPath, "/messages"):
		if group.Platform == service.PlatformOpenAI && !group.AllowMessagesDispatch {
			return false
		}
	case c.Request.Method == http.MethodGet && strings.HasSuffix(requestPath, "/responses"):
		if group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformGrok && group.Platform != service.PlatformComposite {
			return false
		}
	}
	return true
}

func enterpriseMemberLiveEndpoint(method, requestPath string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	requestPath = strings.ToLower(strings.TrimSpace(requestPath))
	if method == http.MethodPost && (strings.HasSuffix(requestPath, "/live") || strings.HasSuffix(requestPath, "/realtime/calls")) {
		return true
	}
	if method == http.MethodGet && strings.Contains(requestPath, "/live/") {
		return true
	}
	if method == http.MethodGet && strings.HasPrefix(requestPath, "/backend-api/codex/") {
		return !strings.HasSuffix(requestPath, "/responses") && !strings.HasSuffix(requestPath, "/models")
	}
	return false
}

// ActivateEnterpriseMemberGroupForModel re-plans model-aware candidates for
// model-late protocols such as Responses WebSocket, then activates the first
// effective candidate before an upstream connection is opened.
func ActivateEnterpriseMemberGroupForModel(c *gin.Context, model string) bool {
	return ActivateEnterpriseMemberGroupForRequest(c, model, nil)
}

type EnterpriseMemberGroupActivationFailure string

const (
	EnterpriseMemberGroupActivationFailureNone                   EnterpriseMemberGroupActivationFailure = ""
	EnterpriseMemberGroupActivationFailureNoEligibleGroup        EnterpriseMemberGroupActivationFailure = "no_eligible_group"
	EnterpriseMemberGroupActivationFailureEligibilityUnavailable EnterpriseMemberGroupActivationFailure = "eligibility_unavailable"
)

type EnterpriseMemberGroupActivationResult struct {
	Activated bool
	Failure   EnterpriseMemberGroupActivationFailure
}

// ActivateEnterpriseMemberGroupForRequest is the model-late equivalent of the
// HTTP middleware planner. body is used only for bounded intent detection and
// is never retained in the route plan or shadow trace.
func ActivateEnterpriseMemberGroupForRequest(c *gin.Context, model string, body []byte) bool {
	return ActivateEnterpriseMemberGroupForRequestResult(c, model, body).Activated
}

// ActivateEnterpriseMemberGroupForRequestResult preserves the failure class
// needed by model-late protocols. In particular, eligibility dependency errors
// must not be misreported as a client model/policy violation after a WebSocket
// upgrade.
func ActivateEnterpriseMemberGroupForRequestResult(c *gin.Context, model string, body []byte) EnterpriseMemberGroupActivationResult {
	plan, ok := enterpriseMemberGroupPlanFromContext(c)
	if !ok {
		return EnterpriseMemberGroupActivationResult{Activated: true}
	}
	model = strings.TrimSpace(model)
	plan.current = -1
	plan.modelPlanApplied = false
	plan.candidates = append([]enterpriseMemberGroupCandidate(nil), plan.legacyCandidates...)
	if model != "" &&
		service.SupportsEnterpriseMemberRoutePlanning(c.Request.URL.Path) &&
		plan.admissionMode != service.EnterpriseMemberModelAdmissionLegacyOrderOnly {
		routePlan, err := evaluateEnterpriseMemberRoutePlan(c, plan, model, body)
		if plan.admissionMode == service.EnterpriseMemberModelAdmissionEnforcePublished {
			if err != nil {
				return EnterpriseMemberGroupActivationResult{Failure: EnterpriseMemberGroupActivationFailureEligibilityUnavailable}
			}
			plan.candidates = selectEnterpriseMemberRouteCandidates(plan.legacyCandidates, routePlan.Candidates)
			if len(plan.candidates) == 0 {
				return EnterpriseMemberGroupActivationResult{Failure: EnterpriseMemberGroupActivationFailureNoEligibleGroup}
			}
			plan.modelPlanApplied = true
		}
	}
	if len(plan.candidates) == 0 {
		return EnterpriseMemberGroupActivationResult{Failure: EnterpriseMemberGroupActivationFailureNoEligibleGroup}
	}
	activateEnterpriseMemberGroupCandidate(c, plan, 0, model)
	ctx := context.WithValue(c.Request.Context(), ctxkey.Model, model)
	c.Request = c.Request.WithContext(ctx)
	return EnterpriseMemberGroupActivationResult{Activated: true}
}

// ActivateNextEnterpriseMemberGroupForModel advances a WebSocket request to
// the next authorized snapshot before an upstream connection is opened. The
// scheduler, not the display-only models list, decides model support.
func ActivateNextEnterpriseMemberGroupForModel(c *gin.Context, model string) bool {
	plan, ok := enterpriseMemberGroupPlanFromContext(c)
	if !ok {
		return false
	}
	model = strings.TrimSpace(model)
	next := plan.current + 1
	if next >= len(plan.candidates) {
		return false
	}
	activateEnterpriseMemberGroupCandidate(c, plan, next, model)
	ctx := context.WithValue(c.Request.Context(), ctxkey.Model, model)
	c.Request = c.Request.WithContext(ctx)
	return true
}

// ActivateEnterpriseMemberGroupByID restores a previously persisted async-task
// group, but only when that group is still present in the request's currently
// authorized candidate plan. Revoked groups therefore fail closed.
func ActivateEnterpriseMemberGroupByID(c *gin.Context, groupID int64) bool {
	plan, ok := enterpriseMemberGroupPlanFromContext(c)
	if !ok || groupID <= 0 {
		return false
	}
	requestedModel, _ := c.Request.Context().Value(ctxkey.Model).(string)
	for i := range plan.candidates {
		if plan.candidates[i].group.ID != groupID {
			continue
		}
		activateEnterpriseMemberGroupCandidate(c, plan, i, requestedModel)
		return true
	}
	return false
}

func extractEnterpriseMemberRouteRequest(c *gin.Context) (string, []byte, error) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "", nil, nil
	}
	requestPath := c.Request.URL.Path
	if strings.Contains(requestPath, "/v1beta/models/") {
		segment := path.Base(requestPath)
		if idx := strings.IndexByte(segment, ':'); idx >= 0 {
			segment = segment[:idx]
		}
		return strings.TrimSpace(segment), nil, nil
	}
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete || c.Request.Body == nil {
		return "", nil, nil
	}
	contentType := c.GetHeader("Content-Type")
	normalizedContentType := strings.ToLower(contentType)
	if normalizedContentType != "" && !strings.Contains(normalizedContentType, "application/json") && !strings.Contains(normalizedContentType, "multipart/form-data") {
		return "", nil, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", body, nil
	}
	if enterpriseMemberLiveEndpoint(c.Request.Method, requestPath) {
		model, extractErr := service.ExtractLiveCallRequestModel(contentType, body)
		return model, body, extractErr
	}
	model, extractErr := service.ExtractEnterpriseMemberBudgetRequestModel(contentType, body)
	return model, body, extractErr
}

func enterpriseMemberModelAdmissionMode(ctx context.Context, settings service.EnterpriseMemberModelAdmissionSettingReader, apiKey *service.APIKey) service.EnterpriseMemberModelAdmissionMode {
	if settings == nil {
		return service.EnterpriseMemberModelAdmissionLegacyOrderOnly
	}
	if resolver, ok := settings.(service.EnterpriseMemberModelAdmissionRequestResolver); ok {
		input := service.EnterpriseMemberModelAdmissionRolloutInput{}
		if apiKey != nil {
			input.APIKeyID = apiKey.ID
			if apiKey.Member != nil {
				input.EnterpriseUserID = apiKey.Member.EnterpriseUserID
				input.MemberID = apiKey.Member.ID
			}
			if input.EnterpriseUserID == 0 {
				input.EnterpriseUserID = apiKey.UserID
			}
			if apiKey.MemberID != nil && input.MemberID == 0 {
				input.MemberID = *apiKey.MemberID
			}
		}
		switch mode := resolver.ResolveEnterpriseMemberModelAdmissionMode(ctx, input).Mode; mode {
		case service.EnterpriseMemberModelAdmissionLegacyOrderOnly,
			service.EnterpriseMemberModelAdmissionShadowPublished,
			service.EnterpriseMemberModelAdmissionEnforcePublished:
			return mode
		default:
			return service.EnterpriseMemberModelAdmissionShadowPublished
		}
	}
	switch mode := settings.GetEnterpriseMemberModelAdmissionMode(ctx); mode {
	case service.EnterpriseMemberModelAdmissionLegacyOrderOnly,
		service.EnterpriseMemberModelAdmissionShadowPublished,
		service.EnterpriseMemberModelAdmissionEnforcePublished:
		return mode
	default:
		return service.EnterpriseMemberModelAdmissionShadowPublished
	}
}

func evaluateEnterpriseMemberRoutePlan(c *gin.Context, plan *enterpriseMemberGroupPlan, model string, body []byte) (*service.EnterpriseMemberRoutePlan, error) {
	legacyGroups := make([]*service.Group, 0, len(plan.legacyCandidates))
	legacyGroupIDs := make([]int64, 0, len(plan.legacyCandidates))
	for i := range plan.legacyCandidates {
		legacyGroups = append(legacyGroups, &plan.legacyCandidates[i].group)
		legacyGroupIDs = append(legacyGroupIDs, plan.legacyCandidates[i].group.ID)
	}
	trace := &EnterpriseMemberRouteShadowTrace{
		Mode:           plan.admissionMode,
		Model:          sanitizeEnterpriseMemberRouteTraceModel(model),
		LegacyGroupIDs: append([]int64(nil), legacyGroupIDs...),
	}
	if plan.planner == nil {
		trace.EvaluationError = true
		setEnterpriseMemberRouteShadowTrace(c, trace)
		err := fmt.Errorf("enterprise member route planner is not configured")
		service.RecordEnterpriseMemberRoutePlanning(plan.admissionMode, legacyGroupIDs, nil, err)
		return nil, err
	}
	plannerStart := time.Now()
	routePlan, err := plan.planner.Plan(c.Request.Context(), service.EnterpriseMemberRouteInput{
		AuthorizedGroups: legacyGroups,
		Model:            model,
		Endpoint:         c.Request.URL.Path,
		Body:             body,
	})
	trace.PlannerLatencyMs = time.Since(plannerStart).Milliseconds()
	if routePlan == nil && err == nil {
		err = fmt.Errorf("enterprise member route planner returned no plan")
	}
	if routePlan != nil {
		plan.routePlanSource = routePlan.Source
		trace.PlanSource = routePlan.Source
		trace.PlannedGroupIDs = enterpriseMemberRouteDecisionGroupIDs(routePlan.Candidates)
		trace.Rejected = make([]EnterpriseMemberRouteShadowRejection, 0, len(routePlan.Rejected))
		for _, rejected := range routePlan.Rejected {
			trace.Rejected = append(trace.Rejected, EnterpriseMemberRouteShadowRejection{GroupID: rejected.GroupID, Reason: rejected.Reason})
		}
	}
	trace.EvaluationError = err != nil
	setEnterpriseMemberRouteShadowTrace(c, trace)
	service.RecordEnterpriseMemberRoutePlanning(plan.admissionMode, legacyGroupIDs, trace.PlannedGroupIDs, err)
	return routePlan, err
}

func setEnterpriseMemberRouteShadowTrace(c *gin.Context, trace *EnterpriseMemberRouteShadowTrace) {
	if c == nil || trace == nil {
		return
	}
	c.Set(enterpriseMemberRouteShadowTraceKey, trace)
	evidence := service.UsageRoutingShadowEvidence{
		Mode:             trace.Mode,
		PlanSource:       trace.PlanSource,
		Model:            trace.Model,
		LegacyGroupIDs:   append([]int64(nil), trace.LegacyGroupIDs...),
		PlannedGroupIDs:  append([]int64(nil), trace.PlannedGroupIDs...),
		EvaluationError:  trace.EvaluationError,
		PlannerLatencyMs: trace.PlannerLatencyMs,
		Rejected:         make([]service.UsageRoutingShadowRejection, 0, len(trace.Rejected)),
	}
	for _, rejected := range trace.Rejected {
		evidence.Rejected = append(evidence.Rejected, service.UsageRoutingShadowRejection{
			GroupID: rejected.GroupID,
			Reason:  rejected.Reason,
		})
	}
	c.Request = c.Request.WithContext(service.WithUsageRoutingShadowEvidence(c.Request.Context(), evidence))
}

func sanitizeEnterpriseMemberRouteTraceModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}

	var sanitized strings.Builder
	sanitized.Grow(min(len(model), enterpriseMemberRouteTraceModelMax))
	runeCount := 0
	for _, value := range model {
		if unicode.IsControl(value) {
			continue
		}
		if runeCount >= enterpriseMemberRouteTraceModelMax {
			break
		}
		_, _ = sanitized.WriteRune(value)
		runeCount++
	}
	return strings.TrimSpace(sanitized.String())
}

func selectEnterpriseMemberRouteCandidates(legacy []enterpriseMemberGroupCandidate, planned []service.EnterpriseMemberRouteCandidateDecision) []enterpriseMemberGroupCandidate {
	legacyByGroupID := make(map[int64]enterpriseMemberGroupCandidate, len(legacy))
	for _, candidate := range legacy {
		legacyByGroupID[candidate.group.ID] = candidate
	}
	selected := make([]enterpriseMemberGroupCandidate, 0, len(planned))
	for _, decision := range planned {
		candidate, ok := legacyByGroupID[decision.GroupID]
		if !ok {
			continue
		}
		candidate.routePlanSnapshotAgeMs = decision.RoutePlanSnapshotAgeMs
		selected = append(selected, candidate)
		delete(legacyByGroupID, decision.GroupID)
	}
	return selected
}

func enterpriseMemberRouteDecisionGroupIDs(decisions []service.EnterpriseMemberRouteCandidateDecision) []int64 {
	if len(decisions) == 0 {
		return nil
	}
	groupIDs := make([]int64, 0, len(decisions))
	for _, decision := range decisions {
		groupIDs = append(groupIDs, decision.GroupID)
	}
	return groupIDs
}

func aggregateEnterpriseMemberRoutePlanFailure(plan *service.EnterpriseMemberRoutePlan) (int, string, string) {
	var hasNoPersistentPool, hasEndpointCapability bool
	if plan != nil {
		for _, rejected := range plan.Rejected {
			switch rejected.Reason {
			case service.EnterpriseMemberRouteReasonNoPersistentPool:
				hasNoPersistentPool = true
			case service.EnterpriseMemberRouteReasonEndpointCapability:
				hasEndpointCapability = true
			}
		}
	}
	if hasNoPersistentPool {
		return http.StatusServiceUnavailable, "NO_AVAILABLE_ACCOUNTS", "No eligible account pool is currently available"
	}
	if hasEndpointCapability {
		return http.StatusForbidden, "MODEL_ENDPOINT_NOT_ALLOWED", "The requested model is not allowed for this endpoint"
	}
	return http.StatusNotFound, "MODEL_NOT_FOUND", "The requested model is not available"
}

func writeEnterpriseMemberRoutePlanError(c *gin.Context, writeError GatewayErrorWriter, status int, code, message string) {
	if c == nil {
		return
	}
	if code != "" {
		c.Header(gatewayErrorCodeHeader, code)
	}
	service.RecordEnterpriseMemberRoutingPlan(0)
	writeError(c, status, message)
	c.Abort()
}

// GetEnterpriseMemberRouteShadowTrace returns a defensive copy for Ops and
// tests. This trace is never included in owner/member/key self-service DTOs.
func GetEnterpriseMemberRouteShadowTrace(c *gin.Context) (EnterpriseMemberRouteShadowTrace, bool) {
	if c == nil {
		return EnterpriseMemberRouteShadowTrace{}, false
	}
	value, exists := c.Get(enterpriseMemberRouteShadowTraceKey)
	if !exists {
		return EnterpriseMemberRouteShadowTrace{}, false
	}
	trace, ok := value.(*EnterpriseMemberRouteShadowTrace)
	if !ok || trace == nil {
		return EnterpriseMemberRouteShadowTrace{}, false
	}
	copyTrace := *trace
	copyTrace.LegacyGroupIDs = append([]int64(nil), trace.LegacyGroupIDs...)
	copyTrace.PlannedGroupIDs = append([]int64(nil), trace.PlannedGroupIDs...)
	copyTrace.Rejected = append([]EnterpriseMemberRouteShadowRejection(nil), trace.Rejected...)
	return copyTrace, true
}

// EnforceEnterpriseMemberBudget authorizes member spending before the request
// reaches an upstream handler. Synchronous requests create a zero-amount receipt
// after checking settled usage; asynchronous image/video tasks keep a positive
// hold. Definitive failures release the receipt, unknown outcomes become
// ambiguous, and successful requests are settled by unified billing.
func EnforceEnterpriseMemberBudget(budgetService *service.EnterpriseMemberBudgetService, cfg *config.Config, writeError GatewayErrorWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || budgetService == nil || cfg == nil || cfg.RunMode == config.RunModeSimple || !enterpriseMemberBudgetRequired(apiKey) {
			c.Next()
			return
		}
		// Responses WebSocket is a multi-turn protocol, so the handler owns one
		// durable zero-amount receipt per response.create turn instead of creating
		// a connection-wide receipt here.
		if isWebSocketUpgrade(c.Request) {
			c.Next()
			return
		}
		var body []byte
		if c.Request.Body != nil && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodDelete {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				writeError(c, http.StatusBadRequest, "Unable to read request for member budget authorization")
				c.Abort()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		requestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		model, _ := c.Request.Context().Value(ctxkey.Model).(string)
		reservation, err := budgetService.Reserve(c.Request.Context(), service.EnterpriseMemberBudgetEstimateInput{
			RequestID: requestID, APIKey: apiKey, RequestedModel: model, Method: c.Request.Method, Endpoint: c.Request.URL.Path, ContentType: c.GetHeader("Content-Type"), Body: body,
		})
		if err != nil {
			status, message := enterpriseMemberBudgetErrorResponse(c, err)
			writeEnterpriseMemberBudgetErrorDetails(c, err)
			writeError(c, status, message)
			c.Abort()
			return
		}
		if reservation == nil {
			c.Next()
			return
		}
		ctx := context.WithValue(c.Request.Context(), ctxkey.MemberBudgetReservation, reservation)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if owned, _ := c.Request.Context().Value(ctxkey.MemberBudgetAsyncTaskOwned).(bool); owned {
			// The async task handler owns the receipt from task creation onward.
			// Its release/ambiguous operations include both request ID and task ID;
			// falling back here would bypass that durable task fence.
			return
		}
		if service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c) {
			reconcileCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reason := service.EnterpriseMemberBudgetOutcomeAmbiguousReason(c)
			if reason == "" {
				reason = "upstream_outcome_unknown"
			}
			_ = budgetService.MarkAmbiguous(reconcileCtx, apiKey.ID, requestID, reason)
			return
		}
		if c.IsAborted() || c.Writer.Status() >= http.StatusBadRequest {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = budgetService.Release(releaseCtx, apiKey.ID, requestID)
		}
	}
}

func writeEnterpriseMemberBudgetErrorDetails(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	appErr := infraerrors.FromError(err)
	if appErr == nil {
		return
	}
	if reason := strings.TrimSpace(appErr.Reason); reason != "" {
		c.Header(gatewayErrorCodeHeader, reason)
	}
	for key, header := range gatewayBudgetMetadataHeaders {
		if value := strings.TrimSpace(appErr.Metadata[key]); value != "" {
			c.Header(header, value)
		}
	}
}

// enterpriseMemberBudgetErrorResponse attributes a reservation failure to the
// party that caused it. Only errors carrying a domain reason are the caller's
// fault; anything else is an infrastructure failure that must be logged and
// reported as a platform error instead of a bare "internal error" 400.
func enterpriseMemberBudgetErrorResponse(c *gin.Context, err error) (int, string) {
	if service.IsEnterpriseMemberBudgetExceeded(err) {
		return http.StatusTooManyRequests, enterpriseMemberBudgetClientMessage(err)
	}
	if !isClassifiedEnterpriseMemberBudgetError(err) {
		logEnterpriseMemberBudgetInternalError(c, err)
		return http.StatusInternalServerError, "Member budget authorization is temporarily unavailable"
	}
	return http.StatusBadRequest, enterpriseMemberBudgetClientMessage(err)
}

// isClassifiedEnterpriseMemberBudgetError reports whether the reservation layer
// attached a domain reason. infraerrors.FromError synthesizes an empty-reason
// wrapper for anything it does not recognize, so an empty reason is exactly the
// unclassified case.
func isClassifiedEnterpriseMemberBudgetError(err error) bool {
	appErr := infraerrors.FromError(err)
	return appErr != nil && strings.TrimSpace(appErr.Reason) != ""
}

// logEnterpriseMemberBudgetInternalError preserves the original error, which
// infraerrors.FromError would otherwise collapse into its unknown-error
// fallback and drop before it reaches any log sink.
func logEnterpriseMemberBudgetInternalError(c *gin.Context, err error) {
	attrs := []any{"error", err}
	if c != nil && c.Request != nil && c.Request.URL != nil {
		attrs = append(attrs, "endpoint", c.Request.URL.Path, "method", c.Request.Method)
	}
	slog.Error("enterprise_member_budget_reserve_internal_error", attrs...)
}

func enterpriseMemberBudgetClientMessage(err error) string {
	appErr := infraerrors.FromError(err)
	if appErr == nil {
		return "Member budget authorization failed"
	}
	if appErr.Reason != service.ErrEnterpriseMemberAsyncBudgetUnavailable.Reason {
		return appErr.Message
	}
	metadata := appErr.Metadata
	return fmt.Sprintf(
		"Asynchronous task budget is unavailable for the %s limit: limit US$%s, settled usage US$%s, active task holds US$%s, requested task hold US$%s. Wait for an active task to finish, lower the task cost, or ask the enterprise administrator to increase the limit.",
		strings.TrimSpace(metadata["limit_window"]),
		strings.TrimSpace(metadata["limit_usd"]),
		strings.TrimSpace(metadata["settled_used_usd"]),
		strings.TrimSpace(metadata["active_task_holds_usd"]),
		strings.TrimSpace(metadata["requested_task_hold_usd"]),
	)
}

func enterpriseMemberBudgetRequired(apiKey *service.APIKey) bool {
	return apiKey != nil && apiKey.MemberID != nil && apiKey.Member != nil
}

func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}
