package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const enterpriseMemberGroupPlanKey = "enterprise_member_group_plan"

type enterpriseMemberGroupCandidate struct {
	group        service.Group
	subscription *service.UserSubscription
	memberIndex  int
}

type enterpriseMemberGroupPlan struct {
	apiKey     *service.APIKey
	candidates []enterpriseMemberGroupCandidate
	current    int
}

// ResolveEnterpriseMemberGroup builds the ordered, request-local candidate
// plan and activates its first group. It never mutates the cached API key.
func ResolveEnterpriseMemberGroup(subscriptionService *service.SubscriptionService, cfg *config.Config, writeError GatewayErrorWriter) gin.HandlerFunc {
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

		requestedModel, err := extractEnterpriseMemberRequestedModel(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Unable to parse request model")
			c.Abort()
			return
		}
		if requestedModel != "" {
			ctx := context.WithValue(c.Request.Context(), ctxkey.Model, requestedModel)
			c.Request = c.Request.WithContext(ctx)
		}

		plan := &enterpriseMemberGroupPlan{apiKey: apiKey, current: -1}
		for i := range apiKey.Member.Groups {
			candidate := &apiKey.Member.Groups[i]
			var selectedSubscription *service.UserSubscription
			if !enterpriseMemberGroupEligible(c, apiKey.User, candidate, requestedModel) {
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
			plan.candidates = append(plan.candidates, enterpriseMemberGroupCandidate{
				group:        *candidate,
				subscription: selectedSubscription,
				memberIndex:  i,
			})
			selectedSubscription = nil
		}
		if len(plan.candidates) == 0 {
			service.RecordEnterpriseMemberRoutingPlan(0)
			writeError(c, http.StatusForbidden, "No authorized enterprise member group can serve this endpoint or model")
			c.Abort()
			return
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
		LogicalRequestID: logicalRequestID,
		AttemptID:        fmt.Sprintf("%s:g%d:a%d", logicalRequestID, requestGroup.ID, candidateIndex+1),
		MemberID:         plan.apiKey.Member.ID,
		MemberVersion:    plan.apiKey.Member.Version,
		GroupID:          requestGroup.ID,
		Platform:         requestGroup.Platform,
		RateMultiplier:   requestGroup.RateMultiplier,
		SubscriptionType: requestGroup.SubscriptionType,
		Endpoint:         c.Request.URL.Path,
		RequestedModel:   requestedModel,
		MappedModel:      requestedModel,
		CandidateIndex:   candidate.memberIndex,
		AttemptNumber:    candidateIndex + 1,
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

func enterpriseMemberGroupEligible(c *gin.Context, user *service.User, group *service.Group, _ string) bool {
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

// ActivateEnterpriseMemberGroupForModel selects the first already-authorized
// candidate. WebSocket ingress calls this after reading the first
// response.create frame but before any upstream connection is opened. Actual
// model schedulability is decided by the account scheduler; models_list_config
// only controls the optional /v1/models response and is not a routing policy.
func ActivateEnterpriseMemberGroupForModel(c *gin.Context, model string) bool {
	plan, ok := enterpriseMemberGroupPlanFromContext(c)
	if !ok {
		return true
	}
	model = strings.TrimSpace(model)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, model)
	ctx := context.WithValue(c.Request.Context(), ctxkey.Model, model)
	c.Request = c.Request.WithContext(ctx)
	return true
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

func extractEnterpriseMemberRequestedModel(c *gin.Context) (string, error) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "", nil
	}
	requestPath := c.Request.URL.Path
	if strings.Contains(requestPath, "/v1beta/models/") {
		segment := path.Base(requestPath)
		if idx := strings.IndexByte(segment, ':'); idx >= 0 {
			segment = segment[:idx]
		}
		return strings.TrimSpace(segment), nil
	}
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete || c.Request.Body == nil {
		return "", nil
	}
	contentType := c.GetHeader("Content-Type")
	normalizedContentType := strings.ToLower(contentType)
	if normalizedContentType != "" && !strings.Contains(normalizedContentType, "application/json") && !strings.Contains(normalizedContentType, "multipart/form-data") {
		return "", nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	return service.ExtractEnterpriseMemberBudgetRequestModel(contentType, body)
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
			status := http.StatusBadRequest
			if service.IsEnterpriseMemberBudgetExceeded(err) {
				status = http.StatusTooManyRequests
			}
			writeEnterpriseMemberBudgetErrorDetails(c, err)
			writeError(c, status, enterpriseMemberBudgetClientMessage(err))
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
