package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberRoutePlannerProductionShapeCases(t *testing.T) {
	groups := []*Group{
		enterpriseMemberRoutePlannerTestGroup(11, "deepseek", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(12, "minimax", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(13, "glm", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(14, "mimo", PlatformOpenAI),
	}
	tests := []struct {
		name           string
		model          string
		endpoint       string
		body           []byte
		publications   map[int64]EnterpriseMemberRoutePublication
		projections    map[int64]*ModelDeliveryGroupProjection
		wantCandidates []int64
		wantDelivery   bool
	}{
		{
			name:           "unpublished image model does not activate first group",
			model:          "gpt-image-1.5",
			endpoint:       "/v1/responses",
			publications:   map[int64]EnterpriseMemberRoutePublication{},
			projections:    map[int64]*ModelDeliveryGroupProjection{},
			wantCandidates: nil,
			wantDelivery:   false,
		},
		{
			name:     "glm model keeps only published glm group",
			model:    "glm-5.2",
			endpoint: "/v1/responses",
			publications: map[int64]EnterpriseMemberRoutePublication{
				13: enterpriseMemberRoutePlannerTestPublication(13, "glm-5.2"),
			},
			projections: map[int64]*ModelDeliveryGroupProjection{
				13: enterpriseMemberRoutePlannerTestProjection(13, "glm-5.2", ModelProtocolOpenAIResponses, true, nil),
			},
			wantCandidates: []int64{13},
			wantDelivery:   true,
		},
		{
			name:     "minimax model keeps only published minimax group",
			model:    "minimax-m3",
			endpoint: "/v1/responses",
			publications: map[int64]EnterpriseMemberRoutePublication{
				12: enterpriseMemberRoutePlannerTestPublication(12, "minimax-m3"),
			},
			projections: map[int64]*ModelDeliveryGroupProjection{
				12: enterpriseMemberRoutePlannerTestProjection(12, "minimax-m3", ModelProtocolOpenAIResponses, true, nil),
			},
			wantCandidates: []int64{12},
			wantDelivery:   true,
		},
		{
			name:     "openai compatible model keeps only published compatible group",
			model:    "gpt-5.6-terra",
			endpoint: "/v1/chat/completions",
			publications: map[int64]EnterpriseMemberRoutePublication{
				14: enterpriseMemberRoutePlannerTestPublication(14, "gpt-5.6-terra"),
			},
			projections: map[int64]*ModelDeliveryGroupProjection{
				14: enterpriseMemberRoutePlannerTestProjection(14, "gpt-5.6-terra", ModelProtocolOpenAIChat, true, nil),
			},
			wantCandidates: []int64{14},
			wantDelivery:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publication := &enterpriseMemberRoutePlannerPublicationFake{items: tt.publications}
			delivery := &enterpriseMemberRoutePlannerDeliveryFake{projections: tt.projections}
			planner := NewEnterpriseMemberRoutePlanner(publication, delivery)

			plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
				AuthorizedGroups: groups,
				Model:            tt.model,
				Endpoint:         tt.endpoint,
				Body:             tt.body,
			})

			require.NoError(t, err)
			require.Equal(t, []int64{11, 12, 13, 14}, publication.calls[0].groupIDs)
			require.Equal(t, tt.wantDelivery, delivery.calls > 0)
			require.Equal(t, tt.wantCandidates, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
			for _, candidate := range plan.Candidates {
				require.NotEqual(t, int64(11), candidate.GroupID, "first authorized group must not win without publication")
			}
		})
	}
}

func TestEnterpriseMemberRoutePlannerUsesRealCompatibilityProjectionWhenNativeRoutingIsDisabled(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(10, "openai", PlatformOpenAI)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		10: enterpriseMemberRoutePlannerTestPublication(10, "published-model"),
	}}
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups:     []Group{{ID: 999, Name: "unrelated", Platform: PlatformOpenAI, Status: StatusActive}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"published-model": "upstream-model"},
		},
	}}}
	planner := NewEnterpriseMemberRoutePlanner(
		publication,
		NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{}),
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "published-model",
		Endpoint:         "/v1/chat/completions",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{10}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Zero(t, groupRepo.listCalls, "enterprise planner must consume authorized group snapshots")
}

func TestEnterpriseMemberRoutePlannerRestoresOnlyAuthorizedLKGAtCurrentDurableRevision(t *testing.T) {
	groups := []*Group{
		enterpriseMemberRoutePlannerTestGroup(10, "first", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(20, "second", PlatformOpenAI),
	}
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		10: enterpriseMemberRoutePlannerTestPublication(10, "published-model"),
		20: enterpriseMemberRoutePlannerTestPublication(20, "published-model"),
	}}
	delivery := &enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
		10: enterpriseMemberRoutePlannerTestProjection(10, "published-model", ModelProtocolOpenAIResponses, true, nil),
		20: enterpriseMemberRoutePlannerTestProjection(20, "published-model", ModelProtocolOpenAIResponses, true, nil),
	}}
	revisionRepo := enterpriseMemberPlannerRevisionRepo(10, 20)
	runtime := NewRoutingEligibilityRuntimeWithIntervals(revisionRepo, nil, 0, 0, time.Minute)
	require.NoError(t, runtime.Reconcile(context.Background()))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)
	planner.SetRoutingEligibilityRuntime(runtime)

	live, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "published-model",
		Endpoint:         "/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePlanSourceLive, live.Source)
	require.Equal(t, []int64{10, 20}, enterpriseMemberRoutePlannerDecisionIDs(live.Candidates))
	require.Equal(t, 2, runtime.SnapshotStore().Len())

	delivery.err = errors.New("capability repository unavailable")
	restored, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups[1:],
		Model:            "published-model",
		Endpoint:         "/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePlanSourceLastKnownGood, restored.Source)
	require.Equal(t, []int64{20}, enterpriseMemberRoutePlannerDecisionIDs(restored.Candidates))

	// Once this process observes a committed generation, the event immediately
	// invalidates matching snapshots. Duplicate/older events cannot revive them.
	require.True(t, runtime.ApplyEvent(RoutingEligibilityEvent{
		Scope:    RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0},
		Revision: 200,
	}))
	failed, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "published-model",
		Endpoint:         "/v1/responses",
	})
	require.ErrorContains(t, err, "capability repository unavailable")
	require.Equal(t, EnterpriseMemberRoutePlanSourceLive, failed.Source)
	require.Empty(t, failed.Candidates)
	require.Zero(t, runtime.SnapshotStore().Len())
}

func TestEnterpriseMemberRoutePlannerUsesReadyMirrorWhenProjectionDatabaseFails(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(30, "only", PlatformOpenAI)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		30: enterpriseMemberRoutePlannerTestPublication(30, "published-model"),
	}}
	delivery := &enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
		30: enterpriseMemberRoutePlannerTestProjection(30, "published-model", ModelProtocolOpenAIResponses, true, nil),
	}}
	revisionRepo := enterpriseMemberPlannerRevisionRepo(30)
	runtime := NewRoutingEligibilityRuntimeWithIntervals(revisionRepo, nil, 0, 0, time.Minute)
	require.NoError(t, runtime.Reconcile(context.Background()))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)
	planner.SetRoutingEligibilityRuntime(runtime)

	_, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group}, Model: "published-model", Endpoint: "/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, 1, runtime.SnapshotStore().Len())

	delivery.err = errors.New("projection unavailable")
	revisionRepo.err = errors.New("revision database unavailable")
	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group}, Model: "published-model", Endpoint: "/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePlanSourceLastKnownGood, plan.Source)
	require.Equal(t, []int64{30}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
}

func TestEnterpriseMemberRoutePlannerCarriesLKGSnapshotAgeOnRestoredCandidate(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(31, "only", PlatformOpenAI)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		31: enterpriseMemberRoutePlannerTestPublication(31, "published-model"),
	}}
	delivery := &enterpriseMemberRoutePlannerDeliveryFake{err: errors.New("projection unavailable")}
	revisionRepo := enterpriseMemberPlannerRevisionRepo(31)
	runtime := NewRoutingEligibilityRuntimeWithIntervals(revisionRepo, nil, 0, 0, time.Minute)
	require.NoError(t, runtime.Reconcile(context.Background()))
	version := NewRoutingEligibilityVersion([]RoutingEligibilityScopeRevision{
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 100},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0}, Revision: 101},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 0}, Revision: 102},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeComposite, ID: 0}, Revision: 103},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 31}, Revision: 110},
	})
	require.True(t, runtime.SnapshotStore().Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key: EnterpriseMemberRouteSnapshotKey{
			PublicModel: "published-model", Endpoint: "/v1/responses", InboundProtocol: ModelProtocolOpenAIResponses,
			Intent: EnterpriseMemberRouteIntentText, GroupID: 31, Eligibility: version,
		},
		Candidates: []EnterpriseMemberRouteSnapshotCandidate{{
			GroupID: 31, ReasonCode: string(EnterpriseMemberRouteReasonEligible), PublicModel: "published-model", UpstreamModel: "upstream-model",
			InboundProtocol: ModelProtocolOpenAIResponses, UpstreamProtocol: ModelProtocolOpenAIResponses, DeliveryMode: ModelDeliveryModeNative,
		}},
		CreatedAt: time.Now().Add(-2 * time.Second),
	}))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)
	planner.SetRoutingEligibilityRuntime(runtime)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group}, Model: "published-model", Endpoint: "/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePlanSourceLastKnownGood, plan.Source)
	require.Len(t, plan.Candidates, 1)
	require.NotNil(t, plan.Candidates[0].RoutePlanSnapshotAgeMs)
	require.GreaterOrEqual(t, *plan.Candidates[0].RoutePlanSnapshotAgeMs, int64(2000))
	require.Less(t, *plan.Candidates[0].RoutePlanSnapshotAgeMs, int64(time.Minute/time.Millisecond))
}

func TestEnterpriseMemberRoutePlannerRejectsLKGBeforeStartupRevisionReconciliation(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(40, "only", PlatformOpenAI)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		40: enterpriseMemberRoutePlannerTestPublication(40, "published-model"),
	}}
	delivery := &enterpriseMemberRoutePlannerDeliveryFake{err: errors.New("projection unavailable")}
	runtime := NewRoutingEligibilityRuntimeWithIntervals(enterpriseMemberPlannerRevisionRepo(40), nil, 0, 0, time.Minute)
	version := NewRoutingEligibilityVersion([]RoutingEligibilityScopeRevision{
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 100},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0}, Revision: 101},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 0}, Revision: 102},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeComposite, ID: 0}, Revision: 103},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 40}, Revision: 110},
	})
	require.True(t, runtime.SnapshotStore().Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key: EnterpriseMemberRouteSnapshotKey{
			PublicModel: "published-model", Endpoint: "/v1/responses", InboundProtocol: ModelProtocolOpenAIResponses,
			Intent: EnterpriseMemberRouteIntentText, GroupID: 40, Eligibility: version,
		},
		Candidates: []EnterpriseMemberRouteSnapshotCandidate{{
			GroupID: 40, ReasonCode: string(EnterpriseMemberRouteReasonEligible), PublicModel: "published-model", UpstreamModel: "upstream-model",
			InboundProtocol: ModelProtocolOpenAIResponses, UpstreamProtocol: ModelProtocolOpenAIResponses, DeliveryMode: ModelDeliveryModeNative,
		}},
	}))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)
	planner.SetRoutingEligibilityRuntime(runtime)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group}, Model: "published-model", Endpoint: "/v1/responses",
	})
	require.ErrorIs(t, err, ErrRoutingEligibilityRevisionUnavailable)
	require.Empty(t, plan.Candidates)
}

func enterpriseMemberPlannerRevisionRepo(groupIDs ...int64) *routingEligibilityRuntimeRepo {
	items := []RoutingEligibilityScopeRevision{
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 100},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0}, Revision: 101},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 0}, Revision: 102},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeComposite, ID: 0}, Revision: 103},
	}
	for index, groupID := range groupIDs {
		items = append(items, RoutingEligibilityScopeRevision{
			Scope:    RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: groupID},
			Revision: uint64(110 + index),
		})
	}
	return newRoutingEligibilityRuntimeRepo(items...)
}

func TestEnterpriseMemberRoutePlannerUsesRealGrokCompatibilityProjection(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(20, "grok", PlatformGrok)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		20: enterpriseMemberRoutePlannerTestPublication(20, "grok-route-model"),
	}}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{92}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 92, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{20},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-route-model": "grok-upstream-model"},
		},
	}}}
	planner := NewEnterpriseMemberRoutePlanner(
		publication,
		NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{}),
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "grok-route-model",
		Endpoint:         "/v1/responses",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{20}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
}

func TestEnterpriseMemberRoutePlannerTreatsMissingCompositeEvaluatorAsDependencyFailure(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(30, "composite", PlatformComposite)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		30: enterpriseMemberRoutePlannerTestPublication(30, "composite-model"),
	}}
	planner := NewEnterpriseMemberRoutePlanner(
		publication,
		NewModelDeliveryService(
			&modelProtocolCatalogAccountRepoStub{},
			&modelProtocolCatalogGroupRepoStub{},
			nil,
			nil,
			&config.Config{},
		),
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "composite-model",
		Endpoint:         "/v1/responses",
	})

	require.Error(t, err)
	require.Empty(t, plan.Candidates)
	require.Equal(t, EnterpriseMemberRouteReasonEvaluationFailed, plan.Rejected[0].Reason)
}

func TestEnterpriseMemberRoutePlannerUsesSideEffectFreeCompositePreview(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(30, "composite", PlatformComposite)
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		30: enterpriseMemberRoutePlannerTestPublication(30, "composite-alias"),
	}}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{301}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5-upstream"}},
	}}}
	delivery := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
	delivery.SetCompositeRoutePreviewer(NewCompositeRouteResolver(compositeRouteRepoStub{
		routes: []CompositeModelRoute{{
			ID: 1, GroupID: 30, PublicModel: "composite-alias", MatchType: CompositeRouteMatchExact,
			TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointResponses, Enabled: true,
		}},
	}))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "composite-alias",
		Endpoint:         "/v1/responses",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{30}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Equal(t, "gpt-5-upstream", plan.Candidates[0].Delivery.Routes[0].UpstreamModel)
}

func TestEnterpriseMemberRoutePlannerPreservesOrderAndNeverAddsUnauthorizedGroups(t *testing.T) {
	groups := []*Group{
		enterpriseMemberRoutePlannerTestGroup(30, "first", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(20, "second", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(10, "third", PlatformOpenAI),
	}
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		99: enterpriseMemberRoutePlannerTestPublication(99, "gpt-test"),
		10: enterpriseMemberRoutePlannerTestPublication(10, "gpt-test"),
		30: enterpriseMemberRoutePlannerTestPublication(30, "gpt-test"),
	}}
	delivery := &enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
		99: enterpriseMemberRoutePlannerTestProjection(99, "gpt-test", ModelProtocolOpenAIChat, true, nil),
		10: enterpriseMemberRoutePlannerTestProjection(10, "gpt-test", ModelProtocolOpenAIChat, true, nil),
		30: enterpriseMemberRoutePlannerTestProjection(30, "gpt-test", ModelProtocolOpenAIChat, true, nil),
	}}
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "gpt-test",
		Protocol:         ModelProtocolOpenAIChat,
	})

	require.NoError(t, err)
	require.Equal(t, []int64{30, 10}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Equal(t, []int64{30, 20, 10}, publication.calls[0].groupIDs)
	require.Equal(t, []int64{30, 10}, delivery.lastGroupIDs)
}

func TestEnterpriseMemberRoutePlannerDoesNotMutateAuthorizedGroupInput(t *testing.T) {
	groups := []*Group{
		enterpriseMemberRoutePlannerTestGroup(1, "one", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(2, "two", PlatformOpenAI),
	}
	originalNames := []string{groups[0].Name, groups[1].Name}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			2: enterpriseMemberRoutePlannerTestPublication(2, "gpt-test"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
			2: enterpriseMemberRoutePlannerTestProjection(2, "gpt-test", ModelProtocolOpenAIResponses, true, nil),
		}},
	)

	_, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "gpt-test",
		Protocol:         ModelProtocolOpenAIResponses,
	})

	require.NoError(t, err)
	require.Equal(t, originalNames[0], groups[0].Name)
	require.Equal(t, originalNames[1], groups[1].Name)
	require.Equal(t, int64(1), groups[0].ID)
	require.Equal(t, int64(2), groups[1].ID)
}

func TestEnterpriseMemberRoutePlannerInfersProtocolAcrossRegisteredRouteAliases(t *testing.T) {
	tests := []struct {
		endpoint string
		want     ModelProtocol
	}{
		{endpoint: "/v1/chat/completions", want: ModelProtocolOpenAIChat},
		{endpoint: "/openai/v1/chat/completions", want: ModelProtocolOpenAIChat},
		{endpoint: "/v1/responses", want: ModelProtocolOpenAIResponses},
		{endpoint: "/responses", want: ModelProtocolOpenAIResponses},
		{endpoint: "/backend-api/codex/responses", want: ModelProtocolOpenAIResponses},
		{endpoint: "/v1/messages", want: ModelProtocolAnthropicMessages},
		{endpoint: "/anthropic/v1/messages/count_tokens", want: ModelProtocolAnthropicMessages},
		{endpoint: "/v1/embeddings", want: ModelProtocolOpenAIEmbeddings},
		{endpoint: "/v1/images/generations", want: ModelProtocolOpenAIImages},
		{endpoint: "/v1/images/edits", want: ModelProtocolOpenAIImages},
		{endpoint: "/v1/live", want: ModelProtocolOpenAILive},
		{endpoint: "/backend-api/codex/realtime/calls", want: ModelProtocolOpenAILive},
		{endpoint: "/v1/images/batches", want: ModelProtocolBatchImages},
		{endpoint: "/v1/videos/generations", want: ModelProtocolGrokVideo},
		{endpoint: "/v1/videos/edits", want: ModelProtocolGrokVideo},
		{endpoint: "/v1/videos/extensions", want: ModelProtocolGrokVideo},
		{endpoint: "/v1beta/models/gemini-2.5-pro:generateContent", want: ModelProtocolGeminiNative},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeEnterpriseMemberRouteProtocol("", tt.endpoint))
			require.True(t, SupportsEnterpriseMemberRoutePlanning(tt.endpoint))
		})
	}
}

func TestEnterpriseMemberRoutePlannerDistinguishesStableFailureReasons(t *testing.T) {
	groups := []*Group{
		enterpriseMemberRoutePlannerTestGroup(1, "unsupported", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(2, "endpoint", PlatformOpenAI),
		enterpriseMemberRoutePlannerTestGroup(3, "pool", PlatformOpenAI),
	}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-test"),
			2: enterpriseMemberRoutePlannerTestPublication(2, "gpt-test"),
			3: enterpriseMemberRoutePlannerTestPublication(3, "gpt-test"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
			1: enterpriseMemberRoutePlannerTestProjection(1, "gpt-test", ModelProtocolOpenAIChat, false, []ModelDeliveryReasonCode{ModelDeliveryReasonModelUnsupported}),
			2: enterpriseMemberRoutePlannerTestProjection(2, "gpt-test", ModelProtocolOpenAIChat, false, []ModelDeliveryReasonCode{ModelDeliveryReasonCapabilityUnsupported}),
			3: enterpriseMemberRoutePlannerTestProjection(3, "gpt-test", ModelProtocolOpenAIChat, false, []ModelDeliveryReasonCode{ModelDeliveryReasonNoStableRoute}),
		}},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "gpt-test",
		Protocol:         ModelProtocolOpenAIChat,
	})

	require.NoError(t, err)
	require.Empty(t, plan.Candidates)
	require.Equal(t, map[int64]EnterpriseMemberRouteReasonCode{
		1: EnterpriseMemberRouteReasonModelUnsupported,
		2: EnterpriseMemberRouteReasonEndpointCapability,
		3: EnterpriseMemberRouteReasonNoPersistentPool,
	}, enterpriseMemberRoutePlannerReasons(plan.Rejected))
}

func TestEnterpriseMemberRoutePlannerRequiresOneConcreteRouteForRequestedProtocol(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(1, "mixed", PlatformOpenAI)
	projection := enterpriseMemberRoutePlannerTestProjection(1, "gpt-test", ModelProtocolOpenAIChat, true, nil)
	projection.Endpoints[ModelProtocolOpenAIResponses] = ModelDeliveryModeNative
	projection.Decisions[ModelProtocolOpenAIResponses] = ModelDeliveryDecision{
		Eligible:        false,
		PublicModel:     "gpt-test",
		InboundProtocol: ModelProtocolOpenAIResponses,
		ReasonCodes:     []ModelDeliveryReasonCode{ModelDeliveryReasonCapabilityUnsupported},
	}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-test"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{1: projection}},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "gpt-test",
		Protocol:         ModelProtocolOpenAIResponses,
	})

	require.NoError(t, err)
	require.Empty(t, plan.Candidates)
	require.Equal(t, EnterpriseMemberRouteReasonEndpointCapability, plan.Rejected[0].Reason)
}

func TestEnterpriseMemberRoutePlannerExplicitImageIntentRequiresGroupPermission(t *testing.T) {
	allowed := enterpriseMemberRoutePlannerTestGroup(1, "images", PlatformOpenAI)
	allowed.AllowImageGeneration = true
	blocked := enterpriseMemberRoutePlannerTestGroup(2, "text", PlatformOpenAI)
	blocked.AllowImageGeneration = false
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-5.5"),
			2: enterpriseMemberRoutePlannerTestPublication(2, "gpt-5.5"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
			1: enterpriseMemberRoutePlannerTestProjection(1, "gpt-5.5", ModelProtocolOpenAIResponses, true, nil),
			2: enterpriseMemberRoutePlannerTestProjection(2, "gpt-5.5", ModelProtocolOpenAIResponses, true, nil),
		}},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{blocked, allowed},
		Model:            "gpt-5.5",
		Endpoint:         "/v1/responses",
		Body:             []byte(`{"model":"gpt-5.5","tools":[{"type":"image_generation"}]}`),
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Equal(t, EnterpriseMemberRouteReasonEndpointCapability, enterpriseMemberRoutePlannerReasons(plan.Rejected)[2])
}

func TestEnterpriseMemberRoutePlannerResponsesTextDoesNotRequireImagePermission(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(1, "text-only", PlatformOpenAI)
	group.AllowImageGeneration = false
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-5.5"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
			1: enterpriseMemberRoutePlannerTestProjection(1, "gpt-5.5", ModelProtocolOpenAIResponses, true, nil),
		}},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "gpt-5.5",
		Endpoint:         "/v1/responses",
		Body:             []byte(`{"model":"gpt-5.5","input":"summarize this text"}`),
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Empty(t, plan.Rejected)
}

func TestEnterpriseMemberRoutePlannerPassiveImageNamespaceDoesNotRequireImagePermission(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(1, "text-only", PlatformOpenAI)
	group.AllowImageGeneration = false
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-5.5"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{projections: map[int64]*ModelDeliveryGroupProjection{
			1: enterpriseMemberRoutePlannerTestProjection(1, "gpt-5.5", ModelProtocolOpenAIResponses, true, nil),
		}},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "gpt-5.5",
		Endpoint:         "/v1/responses",
		Body:             []byte(`{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto","input":"write code"}`),
	})

	require.NoError(t, err)
	require.Equal(t, []int64{1}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Empty(t, plan.Rejected)
}

func TestEnterpriseMemberRoutePlannerMessagesRequiresDispatchCapability(t *testing.T) {
	blocked := enterpriseMemberRoutePlannerTestGroup(1, "openai-text-only", PlatformOpenAI)
	blocked.AllowMessagesDispatch = false
	blocked.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{SonnetMappedModel: "glm-5.2"}
	allowed := enterpriseMemberRoutePlannerTestGroup(2, "openai-messages", PlatformOpenAI)
	allowed.AllowMessagesDispatch = true
	allowed.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{SonnetMappedModel: "glm-5.2"}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{81, 82}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{
		{
			ID: 81, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{1},
			Credentials: map[string]any{"model_mapping": map[string]any{"glm-5.2": "glm-5.2-upstream"}},
		},
		{
			ID: 82, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{2},
			Credentials: map[string]any{"model_mapping": map[string]any{"glm-5.2": "glm-5.2-upstream"}},
		},
	}}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "claude-sonnet-4-5"),
			2: enterpriseMemberRoutePlannerTestPublication(2, "claude-sonnet-4-5"),
		}},
		NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{}),
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{blocked, allowed},
		Model:            "claude-sonnet-4-5",
		Endpoint:         "/v1/messages",
	})

	require.NoError(t, err)
	require.Equal(t, []int64{2}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Equal(t, EnterpriseMemberRouteReasonEndpointCapability, enterpriseMemberRoutePlannerReasons(plan.Rejected)[1])
}

func TestEnterpriseMemberRoutePlannerNonTextEndpointFamiliesUseStableEvaluator(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     ModelProtocol
	}{
		{name: "openai images", endpoint: "/v1/images/generations", want: ModelProtocolOpenAIImages},
		{name: "embeddings", endpoint: "/v1/embeddings", want: ModelProtocolOpenAIEmbeddings},
		{name: "live", endpoint: "/v1/live", want: ModelProtocolOpenAILive},
		{name: "batch image", endpoint: "/v1/images/batches", want: ModelProtocolBatchImages},
		{name: "video", endpoint: "/v1/videos/generations", want: ModelProtocolGrokVideo},
		{name: "gemini native", endpoint: "/v1beta/models/gemini-2.5-pro:generateContent", want: ModelProtocolGeminiNative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, SupportsEnterpriseMemberRoutePlanning(tt.endpoint))
			require.Equal(t, tt.want, normalizeEnterpriseMemberRouteProtocol("", tt.endpoint))
		})
	}
}

func TestEnterpriseMemberRoutePlannerNonTextRealProjectionKeepsOnlyCapableGroups(t *testing.T) {
	openAIImages := enterpriseMemberRoutePlannerTestGroup(1, "openai-images", PlatformOpenAI)
	openAIImages.AllowImageGeneration = true
	openAIImagesDisabled := enterpriseMemberRoutePlannerTestGroup(2, "openai-images-disabled", PlatformOpenAI)
	openAIImagesDisabled.AllowImageGeneration = false
	embedding := enterpriseMemberRoutePlannerTestGroup(3, "openai-embeddings", PlatformOpenAI)
	live := enterpriseMemberRoutePlannerTestGroup(4, "openai-live", PlatformOpenAI)
	live.AllowLive = true
	liveDisabled := enterpriseMemberRoutePlannerTestGroup(5, "openai-live-disabled", PlatformOpenAI)
	liveDisabled.AllowLive = false
	batch := enterpriseMemberRoutePlannerTestGroup(6, "gemini-batch", PlatformGemini)
	batch.AllowImageGeneration = true
	batch.AllowBatchImageGeneration = true
	video := enterpriseMemberRoutePlannerTestGroup(7, "grok-video", PlatformGrok)
	geminiNative := enterpriseMemberRoutePlannerTestGroup(8, "gemini-native", PlatformGemini)

	groups := []*Group{openAIImages, openAIImagesDisabled, embedding, live, liveDisabled, batch, video, geminiNative}
	publications := make(map[int64]EnterpriseMemberRoutePublication, len(groups))
	for _, group := range groups {
		publications[group.ID] = enterpriseMemberRoutePlannerTestPublication(group.ID, "published-model")
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{
		{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{1}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{2}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 103, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{3}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 104, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{4}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 105, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{5}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 106, Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{6}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 107, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{7}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
		{ID: 108, Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{8}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "published-model"}}},
	}}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{101, 102, 103, 104, 105, 106, 107, 108}}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: publications},
		NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{}),
	)

	tests := []struct {
		name       string
		endpoint   string
		groups     []*Group
		want       []int64
		rejectID   int64
		rejectCode EnterpriseMemberRouteReasonCode
	}{
		{name: "openai images require group switch", endpoint: "/v1/images/generations", groups: []*Group{openAIImages, openAIImagesDisabled}, want: []int64{1}, rejectID: 2, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
		{name: "embeddings require openai endpoint capability", endpoint: "/v1/embeddings", groups: []*Group{embedding, video}, want: []int64{3}, rejectID: 7, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
		{name: "live requires oauth and group switch", endpoint: "/v1/live", groups: []*Group{live, liveDisabled, embedding}, want: []int64{4}, rejectID: 5, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
		{name: "batch images require gemini batch switch", endpoint: "/v1/images/batches", groups: []*Group{batch, openAIImages}, want: []int64{6}, rejectID: 1, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
		{name: "grok video requires grok media capability", endpoint: "/v1/videos/generations", groups: []*Group{video, openAIImages}, want: []int64{7}, rejectID: 1, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
		{name: "gemini native requires gemini platform", endpoint: "/v1beta/models/published-model:generateContent", groups: []*Group{geminiNative, openAIImages}, want: []int64{8}, rejectID: 1, rejectCode: EnterpriseMemberRouteReasonEndpointCapability},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
				AuthorizedGroups: tt.groups,
				Model:            "published-model",
				Endpoint:         tt.endpoint,
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
			require.Equal(t, tt.rejectCode, enterpriseMemberRoutePlannerReasons(plan.Rejected)[tt.rejectID])
		})
	}
}

func TestEnterpriseMemberRoutePlannerCompositeNonTextRequiresExplicitEndpointRoute(t *testing.T) {
	group := enterpriseMemberRoutePlannerTestGroup(30, "composite", PlatformComposite)
	group.AllowImageGeneration = true
	group.AllowLive = true
	publication := &enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
		30: enterpriseMemberRoutePlannerTestPublication(30, "image-alias"),
	}}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{301}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-image-upstream": "gpt-image-upstream"}},
	}}}
	delivery := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
	delivery.SetCompositeRoutePreviewer(NewCompositeRouteResolver(compositeRouteRepoStub{
		routes: []CompositeModelRoute{{
			ID: 1, GroupID: 30, PublicModel: "image-alias", MatchType: CompositeRouteMatchExact,
			TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-image-upstream", Endpoint: CompositeRouteEndpointImages, Enabled: true,
		}},
	}))
	planner := NewEnterpriseMemberRoutePlanner(publication, delivery)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "image-alias",
		Endpoint:         "/v1/images/generations",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{30}, enterpriseMemberRoutePlannerDecisionIDs(plan.Candidates))
	require.Equal(t, "gpt-image-upstream", plan.Candidates[0].Delivery.Routes[0].UpstreamModel)

	plan, err = planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: []*Group{group},
		Model:            "image-alias",
		Endpoint:         "/v1/embeddings",
	})
	require.NoError(t, err)
	require.Empty(t, plan.Candidates)
	require.Equal(t, EnterpriseMemberRouteReasonEndpointCapability, plan.Rejected[0].Reason)
}

func TestEnterpriseMemberRoutePlannerDependencyErrorsAreEvaluationFailures(t *testing.T) {
	wantErr := errors.New("projection failed")
	groups := []*Group{enterpriseMemberRoutePlannerTestGroup(1, "one", PlatformOpenAI)}
	planner := NewEnterpriseMemberRoutePlanner(
		&enterpriseMemberRoutePlannerPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			1: enterpriseMemberRoutePlannerTestPublication(1, "gpt-test"),
		}},
		&enterpriseMemberRoutePlannerDeliveryFake{err: wantErr},
	)

	plan, err := planner.Plan(context.Background(), EnterpriseMemberRouteInput{
		AuthorizedGroups: groups,
		Model:            "gpt-test",
		Protocol:         ModelProtocolOpenAIChat,
	})

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, EnterpriseMemberRouteReasonEvaluationFailed, plan.Rejected[0].Reason)
}

func TestEnterpriseMemberChannelPublicationResolverUsesExactSourcesOnly(t *testing.T) {
	cache := populateChannelCache([]Channel{
		{
			ID:       100,
			Status:   StatusActive,
			GroupIDs: []int64{1, 2, 3, 4, 5},
			ModelMapping: map[string]map[string]string{
				PlatformOpenAI: {
					"alias-model": "upstream-model",
					"wild-*":      "wild-upstream",
				},
			},
			ModelPricing: []ChannelModelPricing{
				{Platform: PlatformOpenAI, Models: []string{"priced-model"}},
				{Platform: PlatformOpenAI, Models: []string{"wild-card-only-*"}},
				{Platform: PlatformOpenAI, Models: []string{"wild-exact"}},
			},
		},
	}, map[int64]string{
		1: PlatformOpenAI,
		2: PlatformOpenAI,
		3: PlatformOpenAI,
		4: PlatformOpenAI,
		5: PlatformOpenAI,
	})
	cache.loadedAt = time.Now()
	channel := &ChannelService{}
	channel.cache.Store(cache)
	resolver := NewEnterpriseMemberChannelPublicationResolver(channel)

	alias, err := resolver.ResolveEnterpriseMemberRoutePublications(context.Background(), []int64{1}, "alias-model")
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePublicationExactMapping, alias[1].Source)
	require.Equal(t, "upstream-model", alias[1].ChannelMappedModel)

	priced, err := resolver.ResolveEnterpriseMemberRoutePublications(context.Background(), []int64{2}, "priced-model")
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePublicationExactPricing, priced[2].Source)

	wildcardPricingOnly, err := resolver.ResolveEnterpriseMemberRoutePublications(context.Background(), []int64{3}, "wild-card-only-arbitrary")
	require.NoError(t, err)
	require.Empty(t, wildcardPricingOnly)

	wildcardMappingPriced, err := resolver.ResolveEnterpriseMemberRoutePublications(context.Background(), []int64{4}, "wild-exact")
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberRoutePublicationWildcardMappingPricedExpansion, wildcardMappingPriced[4].Source)
	require.Equal(t, "wild-upstream", wildcardMappingPriced[4].ChannelMappedModel)

	unpublished, err := resolver.ResolveEnterpriseMemberRoutePublications(context.Background(), []int64{5}, "models-list-only")
	require.NoError(t, err)
	require.Empty(t, unpublished)
}

type enterpriseMemberRoutePlannerPublicationCall struct {
	groupIDs []int64
	model    string
}

type enterpriseMemberRoutePlannerPublicationFake struct {
	items map[int64]EnterpriseMemberRoutePublication
	err   error
	calls []enterpriseMemberRoutePlannerPublicationCall
}

func (f *enterpriseMemberRoutePlannerPublicationFake) ResolveEnterpriseMemberRoutePublications(_ context.Context, groupIDs []int64, model string) (map[int64]EnterpriseMemberRoutePublication, error) {
	f.calls = append(f.calls, enterpriseMemberRoutePlannerPublicationCall{
		groupIDs: append([]int64(nil), groupIDs...),
		model:    model,
	})
	if f.err != nil {
		return nil, f.err
	}
	result := make(map[int64]EnterpriseMemberRoutePublication, len(f.items))
	for groupID, item := range f.items {
		result[groupID] = item
	}
	return result, nil
}

type enterpriseMemberRoutePlannerDeliveryFake struct {
	projections  map[int64]*ModelDeliveryGroupProjection
	err          error
	calls        int
	lastGroupIDs []int64
	lastModel    string
	lastProtocol ModelProtocol
}

func (f *enterpriseMemberRoutePlannerDeliveryFake) ResolveForEnterpriseMemberRoute(_ context.Context, groups []*Group, model string, protocol ModelProtocol) (*ModelDeliveryProjection, error) {
	f.calls++
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	f.lastGroupIDs = append([]int64(nil), groupIDs...)
	f.lastModel = model
	f.lastProtocol = protocol
	if f.err != nil {
		return nil, f.err
	}
	result := &ModelDeliveryProjection{Models: make(map[string]map[int64]*ModelDeliveryGroupProjection)}
	result.Models[model] = make(map[int64]*ModelDeliveryGroupProjection)
	for _, groupID := range groupIDs {
		if projection := f.projections[groupID]; projection != nil {
			result.Models[model][groupID] = projection
		}
	}
	return result, nil
}

func enterpriseMemberRoutePlannerTestGroup(id int64, name, platform string) *Group {
	return &Group{
		ID:       id,
		Name:     name,
		Platform: platform,
		Status:   StatusActive,
		Hydrated: true,
	}
}

func enterpriseMemberRoutePlannerTestPublication(groupID int64, model string) EnterpriseMemberRoutePublication {
	return EnterpriseMemberRoutePublication{
		GroupID:            groupID,
		ChannelID:          groupID + 1000,
		PublicModel:        model,
		ChannelMappedModel: model,
		Source:             EnterpriseMemberRoutePublicationExactPricing,
	}
}

func enterpriseMemberRoutePlannerTestProjection(groupID int64, model string, protocol ModelProtocol, eligible bool, reasons []ModelDeliveryReasonCode) *ModelDeliveryGroupProjection {
	projection := &ModelDeliveryGroupProjection{
		PublicModel: model,
		GroupID:     groupID,
		Platform:    PlatformOpenAI,
		Endpoints:   make(map[ModelProtocol]ModelDeliveryMode),
		Decisions:   make(map[ModelProtocol]ModelDeliveryDecision),
	}
	decision := ModelDeliveryDecision{
		Eligible:           eligible,
		PublicModel:        model,
		ChannelMappedModel: model,
		UpstreamModel:      model + "-upstream",
		InboundProtocol:    protocol,
		UpstreamProtocol:   protocol,
		Mode:               ModelDeliveryModeNative,
		ReasonCodes:        reasons,
	}
	projection.Decisions[protocol] = decision
	if eligible {
		projection.Endpoints[protocol] = ModelDeliveryModeNative
		projection.Routes = []ModelDeliveryRoute{{
			PublicModel:        model,
			GroupID:            groupID,
			ChannelMappedModel: model,
			UpstreamModel:      model + "-upstream",
			Decisions:          map[ModelProtocol]ModelDeliveryDecision{protocol: decision},
			Endpoints:          []ModelDeliveryEndpoint{{Protocol: protocol, Mode: ModelDeliveryModeNative}},
		}}
	}
	return projection
}

func enterpriseMemberRoutePlannerDecisionIDs(decisions []EnterpriseMemberRouteCandidateDecision) []int64 {
	if len(decisions) == 0 {
		return nil
	}
	result := make([]int64, 0, len(decisions))
	for _, decision := range decisions {
		result = append(result, decision.GroupID)
	}
	return result
}

func enterpriseMemberRoutePlannerReasons(decisions []EnterpriseMemberRouteCandidateDecision) map[int64]EnterpriseMemberRouteReasonCode {
	result := make(map[int64]EnterpriseMemberRouteReasonCode, len(decisions))
	for _, decision := range decisions {
		result[decision.GroupID] = decision.Reason
	}
	return result
}
