package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberAdmissionModeStub service.EnterpriseMemberModelAdmissionMode

func (s enterpriseMemberAdmissionModeStub) GetEnterpriseMemberModelAdmissionMode(context.Context) service.EnterpriseMemberModelAdmissionMode {
	return service.EnterpriseMemberModelAdmissionMode(s)
}

type enterpriseMemberAdmissionResolverStub struct {
	runtime service.EnterpriseMemberModelAdmissionRuntime
}

func (s enterpriseMemberAdmissionResolverStub) GetEnterpriseMemberModelAdmissionMode(context.Context) service.EnterpriseMemberModelAdmissionMode {
	return s.runtime.Mode
}

func (s enterpriseMemberAdmissionResolverStub) ResolveEnterpriseMemberModelAdmissionMode(_ context.Context, input service.EnterpriseMemberModelAdmissionRolloutInput) service.EnterpriseMemberModelAdmissionRuntime {
	runtime := s.runtime
	if runtime.Mode != service.EnterpriseMemberModelAdmissionEnforcePublished {
		return runtime
	}
	rollout := service.EvaluateEnterpriseMemberModelAdmissionRollout(runtime.Rollout.Policy, input)
	runtime.Rollout = rollout
	if !rollout.Matched || !rollout.Valid || rollout.AutoStopped {
		runtime.Mode = service.EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "rollout_shadow"
	}
	return runtime
}

type enterpriseMemberRoutePlannerFunc func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error)

func (f enterpriseMemberRoutePlannerFunc) Plan(ctx context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
	return f(ctx, input)
}

func TestResolveEnterpriseMemberGroupDefaultAdmissionSkipsModelAwarePlanner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "minimax"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		panic("default admission must not enter the model-aware planner")
	})
	settings := service.NewSettingService(nil, &config.Config{})
	router := enterpriseMemberModelRoutingTestRouterWithSettings(t, key, planner, settings, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), *requestKey.GroupID)
		_, hasTrace := GetEnterpriseMemberRouteShadowTrace(c)
		require.False(t, hasTrace)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"minimax-m3"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestResolveEnterpriseMemberGroupInvalidAdmissionSkipsModelAwarePlanner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	invalidMode := service.EnterpriseMemberModelAdmissionMode("not-a-mode")
	tests := []struct {
		name     string
		settings service.EnterpriseMemberModelAdmissionSettingReader
	}{
		{name: "reader", settings: enterpriseMemberAdmissionModeStub(invalidMode)},
		{name: "resolver", settings: enterpriseMemberAdmissionResolverStub{runtime: service.EnterpriseMemberModelAdmissionRuntime{Mode: invalidMode}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := enterpriseMemberModelRoutingTestKey(
				enterpriseMemberModelRoutingTestGroup(11, "mimo"),
				enterpriseMemberModelRoutingTestGroup(12, "minimax"),
			)
			planner := enterpriseMemberRoutePlannerFunc(func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
				panic("invalid admission mode must not enter the model-aware planner")
			})
			router := enterpriseMemberModelRoutingTestRouterWithSettings(t, key, planner, tc.settings, func(c *gin.Context) {
				requestKey, ok := GetAPIKeyFromContext(c)
				require.True(t, ok)
				require.Equal(t, int64(11), *requestKey.GroupID)
				_, hasTrace := GetEnterpriseMemberRouteShadowTrace(c)
				require.False(t, hasTrace)
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"minimax-m3"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}

func TestResolveEnterpriseMemberGroupShadowPublishedKeepsLegacyExecutionAndRecordsDiff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "minimax"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		require.Equal(t, "minimax-m3", input.Model)
		require.Equal(t, "/v1/responses", input.Endpoint)
		require.JSONEq(t, `{"model":"minimax-m3","input":"hello"}`, string(input.Body))
		return &service.EnterpriseMemberRoutePlan{
			Model: input.Model,
			Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID: 12,
				Reason:  service.EnterpriseMemberRouteReasonEligible,
			}},
			Rejected: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID: 11,
				Reason:  service.EnterpriseMemberRouteReasonModelUnpublished,
			}},
		}, nil
	})

	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionShadowPublished, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), *requestKey.GroupID, "shadow mode must preserve legacy execution order")
		trace, ok := GetEnterpriseMemberRouteShadowTrace(c)
		require.True(t, ok)
		require.Equal(t, []int64{11, 12}, trace.LegacyGroupIDs)
		require.Equal(t, []int64{12}, trace.PlannedGroupIDs)
		require.Equal(t, []EnterpriseMemberRouteShadowRejection{{
			GroupID: 11,
			Reason:  service.EnterpriseMemberRouteReasonModelUnpublished,
		}}, trace.Rejected)
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, service.EnterpriseMemberModelAdmissionShadowPublished, active.RoutePlanMode)
		require.False(t, active.ModelPlanApplied, "shadow mode must not authorize new replay behavior")
		evidence, ok := service.UsageRoutingShadowEvidenceFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, service.EnterpriseMemberModelAdmissionShadowPublished, evidence.Mode)
		require.Equal(t, []int64{11, 12}, evidence.LegacyGroupIDs)
		require.Equal(t, []int64{12}, evidence.PlannedGroupIDs)
		require.Equal(t, []service.UsageRoutingShadowRejection{{
			GroupID: 11,
			Reason:  service.EnterpriseMemberRouteReasonModelUnpublished,
		}}, evidence.Rejected)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"minimax-m3","input":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Nil(t, key.GroupID, "cached API key must remain immutable")
}

func TestSanitizeEnterpriseMemberRouteTraceModelBoundsAndRemovesControlCharacters(t *testing.T) {
	raw := " \nmodel\x00\t" + strings.Repeat("界", enterpriseMemberRouteTraceModelMax+20) + "\r "
	sanitized := sanitizeEnterpriseMemberRouteTraceModel(raw)

	require.NotContains(t, sanitized, "\x00")
	require.NotContains(t, sanitized, "\n")
	require.NotContains(t, sanitized, "\t")
	require.NotContains(t, sanitized, "\r")
	require.LessOrEqual(t, len([]rune(sanitized)), enterpriseMemberRouteTraceModelMax)
	require.True(t, strings.HasPrefix(sanitized, "model"))
}

func TestResolveEnterpriseMemberGroupEnforcePublishedActivatesOnlyPlannedCandidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "glm"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		return &service.EnterpriseMemberRoutePlan{
			Model: input.Model,
			Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID: 12,
				Reason:  service.EnterpriseMemberRouteReasonEligible,
			}},
		}, nil
	})

	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(12), *requestKey.GroupID)
		require.Len(t, requestKey.Member.Groups, 1)
		require.Equal(t, int64(12), requestKey.Member.Groups[0].ID)
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, 1, active.CandidateIndex, "candidate index must refer to original member binding order")
		require.Equal(t, service.EnterpriseMemberModelAdmissionEnforcePublished, active.RoutePlanMode)
		require.True(t, active.ModelPlanApplied)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"glm-5.2"}`, string(body))
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"glm-5.2"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Nil(t, key.GroupID)
}

func TestResolveEnterpriseMemberGroupRequestRolloutMissKeepsShadowExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "glm"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		return &service.EnterpriseMemberRoutePlan{
			Model: input.Model,
			Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID: 12,
				Reason:  service.EnterpriseMemberRouteReasonEligible,
			}},
		}, nil
	})
	settings := enterpriseMemberAdmissionResolverStub{runtime: service.EnterpriseMemberModelAdmissionRuntime{
		Mode:   service.EnterpriseMemberModelAdmissionEnforcePublished,
		Source: "settings",
		Rollout: service.EnterpriseMemberModelAdmissionRolloutState{Policy: service.EnterpriseMemberModelAdmissionRolloutPolicy{
			Percentage: 0,
		}},
	}}

	router := enterpriseMemberModelRoutingTestRouterWithSettings(t, key, planner, settings, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), *requestKey.GroupID, "non-matching rollout requests must remain shadow")
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, service.EnterpriseMemberModelAdmissionShadowPublished, active.RoutePlanMode)
		require.False(t, active.ModelPlanApplied)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"glm-5.2"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestResolveEnterpriseMemberGroupRequestRolloutAllowlistEnforces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "glm"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		return &service.EnterpriseMemberRoutePlan{
			Model: input.Model,
			Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID: 12,
				Reason:  service.EnterpriseMemberRouteReasonEligible,
			}},
		}, nil
	})
	settings := enterpriseMemberAdmissionResolverStub{runtime: service.EnterpriseMemberModelAdmissionRuntime{
		Mode:   service.EnterpriseMemberModelAdmissionEnforcePublished,
		Source: "settings",
		Rollout: service.EnterpriseMemberModelAdmissionRolloutState{Policy: service.EnterpriseMemberModelAdmissionRolloutPolicy{
			MemberIDs: []int64{8},
		}},
	}}

	router := enterpriseMemberModelRoutingTestRouterWithSettings(t, key, planner, settings, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(12), *requestKey.GroupID)
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, service.EnterpriseMemberModelAdmissionEnforcePublished, active.RoutePlanMode)
		require.True(t, active.ModelPlanApplied)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"glm-5.2"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestResolveEnterpriseMemberGroupEnforcePublishedRejectsUnpublishedModelBeforeActivation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "deepseek"),
		enterpriseMemberModelRoutingTestGroup(12, "mimo"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		rejected := make([]service.EnterpriseMemberRouteCandidateDecision, 0, len(input.AuthorizedGroups))
		for _, group := range input.AuthorizedGroups {
			rejected = append(rejected, service.EnterpriseMemberRouteCandidateDecision{
				GroupID: group.ID,
				Reason:  service.EnterpriseMemberRouteReasonModelUnpublished,
			})
		}
		return &service.EnterpriseMemberRoutePlan{Model: input.Model, Rejected: rejected}, nil
	})
	activated := false
	activeAtError := true
	errorWriter := func(c *gin.Context, status int, message string) {
		_, activeAtError = service.ActiveGroupFromContext(c.Request.Context())
		AnthropicErrorWriter(c, status, message)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Next()
	})
	router.Use(ResolveEnterpriseMemberGroup(nil, planner, enterpriseMemberAdmissionModeStub(service.EnterpriseMemberModelAdmissionEnforcePublished), &config.Config{RunMode: config.RunModeSimple}, errorWriter))
	router.POST("/v1/responses", func(c *gin.Context) {
		activated = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-image-1.5","tools":[{"type":"image_generation"}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
	require.Equal(t, "MODEL_NOT_FOUND", response.Header().Get(gatewayErrorCodeHeader))
	require.False(t, activated)
	require.False(t, activeAtError)
	require.Nil(t, key.GroupID)
	require.NotContains(t, response.Body.String(), "deepseek")
	require.NotContains(t, response.Body.String(), "mimo")
	require.NotContains(t, strings.ToLower(response.Body.String()), "this group")
}

func TestResolveEnterpriseMemberGroupEnforcePublishedFailsClosedOnProjectionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(enterpriseMemberModelRoutingTestGroup(11, "openai"))
	planner := enterpriseMemberRoutePlannerFunc(func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		return &service.EnterpriseMemberRoutePlan{Rejected: []service.EnterpriseMemberRouteCandidateDecision{{
			GroupID: 11,
			Reason:  service.EnterpriseMemberRouteReasonEvaluationFailed,
		}}}, errors.New("projection unavailable")
	})
	called := false
	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.6-terra"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Equal(t, "ROUTING_ELIGIBILITY_UNAVAILABLE", response.Header().Get(gatewayErrorCodeHeader))
	require.False(t, called)
	require.Nil(t, key.GroupID)
}

func TestResolveEnterpriseMemberGroupOrdinaryKeyBypassesPlanner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(11)
	key := &service.APIKey{ID: 17, UserID: 3, GroupID: &groupID, Group: enterpriseMemberModelRoutingTestGroup(11, "openai")}
	planner := enterpriseMemberRoutePlannerFunc(func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		panic("ordinary keys must bypass enterprise member planning")
	})
	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestResolveEnterpriseMemberGroupEnforceDoesNotPlanModelessLookupEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := enterpriseMemberModelRoutingTestGroup(11, "grok-video")
	group.Platform = service.PlatformGrok
	key := enterpriseMemberModelRoutingTestKey(group)
	planner := enterpriseMemberRoutePlannerFunc(func(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		panic("modeless lookup endpoints must remain on their guarded legacy path")
	})
	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), *requestKey.GroupID)
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/videos/request-123", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestActivateEnterpriseMemberGroupForRequestReplansWebSocketFirstFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
		enterpriseMemberModelRoutingTestGroup(12, "openai"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, input service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		require.Equal(t, "gpt-5.6-terra", input.Model)
		require.JSONEq(t, `{"type":"response.create","model":"gpt-5.6-terra"}`, string(input.Body))
		return &service.EnterpriseMemberRoutePlan{Model: input.Model, Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
			GroupID: 12,
			Reason:  service.EnterpriseMemberRouteReasonEligible,
		}}}, nil
	})
	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		firstFrame := []byte(`{"type":"response.create","model":"gpt-5.6-terra"}`)
		require.True(t, ActivateEnterpriseMemberGroupForRequest(c, "gpt-5.6-terra", firstFrame))
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(12), *requestKey.GroupID)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestActivateEnterpriseMemberGroupForRequestClassifiesPlannerDependencyFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := enterpriseMemberModelRoutingTestKey(
		enterpriseMemberModelRoutingTestGroup(11, "mimo"),
	)
	planner := enterpriseMemberRoutePlannerFunc(func(_ context.Context, _ service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
		return nil, errors.New("capability repository unavailable")
	})
	router := enterpriseMemberModelRoutingTestRouter(t, key, planner, service.EnterpriseMemberModelAdmissionEnforcePublished, func(c *gin.Context) {
		result := ActivateEnterpriseMemberGroupForRequestResult(c, "gpt-5.6-terra", []byte(`{"type":"response.create","model":"gpt-5.6-terra"}`))
		require.False(t, result.Activated)
		require.Equal(t, EnterpriseMemberGroupActivationFailureEligibilityUnavailable, result.Failure)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func enterpriseMemberModelRoutingTestRouter(
	t *testing.T,
	key *service.APIKey,
	planner service.EnterpriseMemberRoutePlanningService,
	mode service.EnterpriseMemberModelAdmissionMode,
	next gin.HandlerFunc,
) *gin.Engine {
	return enterpriseMemberModelRoutingTestRouterWithSettings(t, key, planner, enterpriseMemberAdmissionModeStub(mode), next)
}

func enterpriseMemberModelRoutingTestRouterWithSettings(
	t *testing.T,
	key *service.APIKey,
	planner service.EnterpriseMemberRoutePlanningService,
	settings service.EnterpriseMemberModelAdmissionSettingReader,
	next gin.HandlerFunc,
) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Next()
	})
	router.Use(ResolveEnterpriseMemberGroup(
		nil,
		planner,
		settings,
		&config.Config{RunMode: config.RunModeSimple},
		AnthropicErrorWriter,
	))
	router.Any("/v1/responses", next)
	router.Any("/v1/chat/completions", next)
	router.Any("/v1/videos/:request_id", next)
	return router
}

func enterpriseMemberModelRoutingTestKey(groups ...*service.Group) *service.APIKey {
	memberID := int64(8)
	memberGroups := make([]service.Group, 0, len(groups))
	for _, group := range groups {
		memberGroups = append(memberGroups, *group)
	}
	return &service.APIKey{
		ID:       17,
		UserID:   3,
		MemberID: &memberID,
		User: &service.User{
			ID:          3,
			Role:        service.RoleUser,
			AccountType: service.UserAccountTypeEnterprise,
			Status:      service.StatusActive,
			Balance:     10,
		},
		Member: &service.EnterpriseMember{
			ID:               memberID,
			EnterpriseUserID: 3,
			Status:           service.EnterpriseMemberStatusActive,
			Version:          4,
			Groups:           memberGroups,
		},
	}
}

func enterpriseMemberModelRoutingTestGroup(id int64, name string) *service.Group {
	return &service.Group{
		ID:                    id,
		Name:                  name,
		Platform:              service.PlatformOpenAI,
		Status:                service.StatusActive,
		Hydrated:              true,
		AllowMessagesDispatch: true,
		AllowImageGeneration:  true,
	}
}
