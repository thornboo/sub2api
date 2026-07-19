package service

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type enterpriseMemberBudgetRateRepoStub struct {
	UserGroupRateRepository
	rate *float64
	err  error
}

type enterpriseMemberBudgetAccountRepoStub struct {
	accounts []Account
	err      error
}

func (s *enterpriseMemberBudgetAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return append([]Account(nil), s.accounts...), s.err
}

type enterpriseMemberBudgetReceiptRepoSpy struct {
	EnterpriseMemberBudgetRepository
	requestID              string
	memberID               int64
	groupID                *int64
	payloadHash            string
	amount                 float64
	receiptKind            string
	expiresAt              time.Time
	asyncTaskRequestID     string
	asyncTaskID            string
	asyncTaskExpiresAt     time.Time
	asyncTaskExecuting     bool
	resolvedReceiptID      int64
	resolvedInput          EnterpriseMemberAmbiguousReceiptResolution
	resolvedActorUserID    int64
	resolveAmbiguousCalled bool
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) AttachAsyncTask(_ context.Context, requestID, taskID string, expiresAt time.Time) error {
	s.asyncTaskRequestID = requestID
	s.asyncTaskID = taskID
	s.asyncTaskExpiresAt = expiresAt
	return nil
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) MarkAsyncTaskExecuting(_ context.Context, requestID, taskID string) error {
	s.asyncTaskRequestID = requestID
	s.asyncTaskID = taskID
	s.asyncTaskExecuting = true
	return nil
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) Reserve(_ context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error) {
	return s.reserve(requestID, memberID, groupID, payloadHash, amount, EnterpriseMemberReceiptKindLegacy, expiresAt)
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) ReserveWithKind(_ context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, receiptKind string, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error) {
	return s.reserve(requestID, memberID, groupID, payloadHash, amount, receiptKind, expiresAt)
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) reserve(requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, receiptKind string, expiresAt time.Time) (*EnterpriseMemberBudgetReservation, error) {
	s.requestID = requestID
	s.memberID = memberID
	s.groupID = groupID
	s.payloadHash = payloadHash
	s.amount = amount
	s.receiptKind = receiptKind
	s.expiresAt = expiresAt
	return &EnterpriseMemberBudgetReservation{
		ID:          91,
		RequestID:   requestID,
		MemberID:    memberID,
		GroupID:     groupID,
		PayloadHash: payloadHash,
		ReservedUSD: amount,
		Status:      "reserved",
		ReceiptKind: receiptKind,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *enterpriseMemberBudgetReceiptRepoSpy) ResolveAmbiguousReceipt(_ context.Context, receiptID int64, input EnterpriseMemberAmbiguousReceiptResolution, actorUserID int64) (*EnterpriseMemberAmbiguousReceipt, error) {
	s.resolveAmbiguousCalled = true
	s.resolvedReceiptID = receiptID
	s.resolvedInput = input
	s.resolvedActorUserID = actorUserID
	return &EnterpriseMemberAmbiguousReceipt{ID: receiptID, OutcomeReason: "manual_release"}, nil
}

func TestEnterpriseMemberBudgetManualResolutionOnlyAllowsProvenRelease(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	_, err := budgetService.ResolveAmbiguousReceipt(context.Background(), 91, EnterpriseMemberAmbiguousReceiptResolution{
		Decision:                  "settle",
		ExpectedReconcileAttempts: 2,
		Reason:                    "provider charged the request",
	}, 7)
	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetConflict)
	require.False(t, repo.resolveAmbiguousCalled, "manual settlement must not bypass the unified billing transaction")

	resolved, err := budgetService.ResolveAmbiguousReceipt(context.Background(), 91, EnterpriseMemberAmbiguousReceiptResolution{
		Decision:                  " RELEASE ",
		ExpectedReconcileAttempts: 2,
		Reason:                    " upstream confirmed no task exists ",
	}, 7)
	require.NoError(t, err)
	require.Equal(t, int64(91), resolved.ID)
	require.True(t, repo.resolveAmbiguousCalled)
	require.Equal(t, int64(91), repo.resolvedReceiptID)
	require.Equal(t, EnterpriseMemberReceiptDecisionRelease, repo.resolvedInput.Decision)
	require.Equal(t, "upstream confirmed no task exists", repo.resolvedInput.Reason)
	require.Equal(t, int64(7), repo.resolvedActorUserID)
}

func TestEnterpriseMemberBudgetReserveCreatesDurableReceiptForUnlimitedMember(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)
	memberID := int64(44)
	groupID := int64(9)
	body := []byte(`{"model":"gpt-test"}`)

	receipt, err := budgetService.Reserve(context.Background(), EnterpriseMemberBudgetEstimateInput{
		RequestID:   "request-uuid",
		APIKey:      &APIKey{ID: 17, MemberID: &memberID, GroupID: &groupID, Member: &EnterpriseMember{ID: memberID}},
		Endpoint:    "/v1/responses",
		ContentType: "application/json",
		Body:        body,
	})

	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, "17:client:request-uuid", repo.requestID)
	require.Equal(t, memberID, repo.memberID)
	require.Equal(t, &groupID, repo.groupID)
	require.Equal(t, HashUsageRequestPayload(body), repo.payloadHash)
	require.Zero(t, repo.amount)
	require.Equal(t, EnterpriseMemberReceiptKindSync, repo.receiptKind)
}

func TestEnterpriseMemberBudgetReserveCreatesZeroAmountReceiptForLimitedSynchronousRequest(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)
	memberID := int64(44)
	groupID := int64(9)
	body := []byte(`{"model":"expensive-mapped-model","input":"hello"}`)

	receipt, err := budgetService.Reserve(context.Background(), EnterpriseMemberBudgetEstimateInput{
		RequestID: "request-with-remaining-budget",
		APIKey: &APIKey{
			ID:       17,
			MemberID: &memberID,
			GroupID:  &groupID,
			Member: &EnterpriseMember{
				ID:              memberID,
				MonthlyLimitUSD: 300,
				Groups:          []Group{{ID: groupID, RateMultiplier: 99}},
			},
		},
		RequestedModel: "expensive-mapped-model",
		Method:         "POST",
		Endpoint:       "/v1/responses",
		ContentType:    "application/json",
		Body:           body,
	})

	require.NoError(t, err, "synchronous authorization must not depend on a theoretical pricing upper bound")
	require.NotNil(t, receipt)
	require.Zero(t, repo.amount)
	require.Zero(t, receipt.ReservedUSD)
	require.Equal(t, EnterpriseMemberReceiptKindSync, repo.receiptKind)
	require.Equal(t, HashUsageRequestPayload(body), repo.payloadHash)
}

func TestEnterpriseMemberBudgetReserveKeepsPositiveHoldForAsynchronousVideo(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{}}
	budgetService := NewEnterpriseMemberBudgetService(repo, NewModelPricingResolver(nil, NewBillingService(nil, pricingService)), nil)
	memberID := int64(44)
	groupID := int64(9)

	receipt, err := budgetService.Reserve(context.Background(), EnterpriseMemberBudgetEstimateInput{
		RequestID: "async-video-request",
		APIKey: &APIKey{
			ID:       17,
			MemberID: &memberID,
			GroupID:  &groupID,
			Member: &EnterpriseMember{
				ID:              memberID,
				MonthlyLimitUSD: 300,
				Groups:          []Group{{ID: groupID, RateMultiplier: 1}},
			},
		},
		RequestedModel: "grok-imagine-video",
		Method:         "POST",
		Endpoint:       "/v1/videos/generations",
		ContentType:    "application/json",
		Body:           []byte(`{"model":"grok-imagine-video","duration":8}`),
	})

	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Positive(t, repo.amount, "asynchronous work that continues after the HTTP response must keep a real hold")
	require.Equal(t, repo.amount, receipt.ReservedUSD)
	require.Equal(t, EnterpriseMemberReceiptKindAsyncVideo, repo.receiptKind)
}

func TestEnterpriseMemberBudgetReserveKeepsPositiveHoldForAsynchronousImage(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{}}
	budgetService := NewEnterpriseMemberBudgetService(repo, NewModelPricingResolver(nil, NewBillingService(nil, pricingService)), nil)
	memberID := int64(44)
	groupID := int64(9)
	imagePrice := 0.04

	receipt, err := budgetService.Reserve(context.Background(), EnterpriseMemberBudgetEstimateInput{
		RequestID: "async-image-request",
		APIKey: &APIKey{
			ID:       17,
			MemberID: &memberID,
			GroupID:  &groupID,
			Member: &EnterpriseMember{
				ID:              memberID,
				MonthlyLimitUSD: 300,
				Groups: []Group{{
					ID: groupID, RateMultiplier: 1,
					ImagePrice1K: &imagePrice, ImagePrice2K: &imagePrice, ImagePrice4K: &imagePrice,
				}},
			},
		},
		RequestedModel: "gpt-image-2",
		Method:         "POST",
		Endpoint:       "/v1/images/generations/async",
		ContentType:    "application/json",
		Body:           []byte(`{"model":"gpt-image-2","prompt":"cat","n":1}`),
	})

	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Positive(t, repo.amount, "an image task that runs after the 202 response must keep a real hold")
	require.Equal(t, repo.amount, receipt.ReservedUSD)
	require.Equal(t, EnterpriseMemberReceiptKindAsyncImage, repo.receiptKind)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), repo.expiresAt, time.Second)
}

func TestEnterpriseMemberBudgetAsyncImageTaskLinkUsesShortQueuedFence(t *testing.T) {
	repo := &enterpriseMemberBudgetReceiptRepoSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	require.NoError(t, budgetService.AttachImageTask(context.Background(), "17:client:request-1", "imgtask_1"))
	require.Equal(t, "17:client:request-1", repo.asyncTaskRequestID)
	require.Equal(t, "imgtask_1", repo.asyncTaskID)
	require.WithinDuration(t, time.Now().Add(defaultImageTaskDispatchTimeout+defaultImageTaskRecoveryGrace), repo.asyncTaskExpiresAt, time.Second)

	require.NoError(t, budgetService.MarkImageTaskExecuting(context.Background(), "17:client:request-1", "imgtask_1"))
	require.True(t, repo.asyncTaskExecuting)
}

func TestEnterpriseMemberBudgetAmountHoldEndpointClassification(t *testing.T) {
	for _, endpoint := range []string{
		"/v1/images/generations/async",
		"/images/generations/async",
		"/v1/images/edits/async",
		"/images/edits/async",
		"/v1/videos/generations",
		"/v1/videos/edits",
		"/v1/videos/extensions",
	} {
		require.True(t, enterpriseMemberBudgetRequiresAmountHold("POST", endpoint), endpoint)
	}
	for _, endpoint := range []string{
		"/v1/responses",
		"/v1/images/generations",
		"/v1/images/edits",
		"/v1/images/tasks/imgtask-1",
	} {
		require.False(t, enterpriseMemberBudgetRequiresAmountHold("POST", endpoint), endpoint)
	}
	require.False(t, enterpriseMemberBudgetRequiresAmountHold("GET", "/v1/images/generations/async"))
}

func (s *enterpriseMemberBudgetRateRepoStub) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	return s.rate, s.err
}

func TestEnterpriseMemberBudgetRequestShapeParsesMultipartImageEditFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("n", "3"))
	file, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("not-a-real-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	shape, err := parseEnterpriseMemberBudgetRequestShape(writer.FormDataContentType(), body.Bytes())
	require.NoError(t, err)
	require.Equal(t, "gpt-image-1", shape.Model)
	require.Equal(t, 3, shape.N)
}

func TestEnterpriseMemberBudgetFailsClosedWhenUserRateCannotBeResolved(t *testing.T) {
	service := &EnterpriseMemberBudgetService{
		pricingResolver:   &ModelPricingResolver{billingService: NewBillingService(nil, nil)},
		userGroupRateRepo: &enterpriseMemberBudgetRateRepoStub{err: errors.New("db unavailable")},
	}
	memberID := int64(44)
	_, err := service.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey:         &APIKey{UserID: 7, MemberID: &memberID, Member: &EnterpriseMember{Groups: []Group{{ID: 9, RateMultiplier: 1}}}},
		RequestedModel: "gpt-5",
		Endpoint:       "/v1/responses",
		ContentType:    "application/json",
		Body:           []byte(`{"max_output_tokens":1}`),
	})
	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetUnbounded)
}

func TestEnterpriseMemberBudgetFailsClosedWhenAnyCandidateGroupCannotBePriced(t *testing.T) {
	channelService := &ChannelService{}
	channelCache := newEmptyChannelCache()
	channelCache.loadedAt = time.Now()
	channelService.cache.Store(channelCache)
	budgetService := &EnterpriseMemberBudgetService{
		pricingResolver: NewModelPricingResolver(channelService, NewBillingService(nil, nil)),
	}
	memberID := int64(44)
	_, err := budgetService.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey: &APIKey{
			UserID:   7,
			MemberID: &memberID,
			Member: &EnterpriseMember{Groups: []Group{
				{ID: 9, RateMultiplier: 1, DefaultMappedModel: "gpt-5"},
				{ID: 10, RateMultiplier: 1},
			}},
		},
		RequestedModel: "model-without-pricing",
		Endpoint:       "/v1/responses",
		ContentType:    "application/json",
		Body:           []byte(`{"max_output_tokens":1}`),
	})
	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetUnbounded)
}

func TestEnterpriseMemberBudgetModelCandidatesIncludeConfiguredMappings(t *testing.T) {
	models := enterpriseMemberBudgetModelCandidates("requested", &Group{
		DefaultMappedModel: "default-mapped",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:    "opus-mapped",
			ExactModelMappings: map[string]string{"input": "exact-mapped"},
		},
	})
	require.ElementsMatch(t, []string{"requested", "default-mapped", "opus-mapped", "exact-mapped"}, models)
}

func TestEnterpriseMemberBudgetReachableModelsIncludeChannelAndAccountMappings(t *testing.T) {
	channelService := &ChannelService{}
	channelCache := newEmptyChannelCache()
	channelCache.loadedAt = time.Now()
	channelCache.channelByGroupID[9] = &Channel{ID: 19, Status: StatusActive, BillingModelSource: BillingModelSourceUpstream}
	channelCache.groupPlatform[9] = PlatformOpenAI
	channelCache.mappingByGroupModel[channelModelKey{groupID: 9, platform: PlatformOpenAI, model: "requested"}] = "channel-mapped"
	channelService.cache.Store(channelCache)

	budgetService := &EnterpriseMemberBudgetService{
		pricingResolver: NewModelPricingResolver(channelService, NewBillingService(nil, nil)),
		accountRepo: &enterpriseMemberBudgetAccountRepoStub{accounts: []Account{{
			ID: 71, Platform: PlatformOpenAI, Credentials: map[string]any{
				"model_mapping": map[string]any{"channel-mapped": "upstream-mapped"},
			},
		}}},
	}
	models, err := budgetService.enterpriseMemberBudgetReachableModelCandidates(context.Background(), "requested", &Group{ID: 9})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"requested", "channel-mapped", "upstream-mapped"}, models)
}

func TestEnterpriseMemberBudgetFailsClosedWhenReachableAccountsCannotBeLoaded(t *testing.T) {
	budgetService := &EnterpriseMemberBudgetService{
		pricingResolver: NewModelPricingResolver(nil, NewBillingService(nil, nil)),
		accountRepo:     &enterpriseMemberBudgetAccountRepoStub{err: errors.New("database unavailable")},
	}
	memberID := int64(44)
	_, err := budgetService.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey:         &APIKey{MemberID: &memberID, Member: &EnterpriseMember{Groups: []Group{{ID: 9, RateMultiplier: 1}}}},
		RequestedModel: "gpt-5",
		Endpoint:       "/v1/responses",
		Body:           []byte(`{"max_output_tokens":1}`),
	})
	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetUnbounded)
}

func TestEnterpriseMemberBudgetReservesForMostExpensiveAccountMappedModel(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"requested":       {InputCostPerToken: 1e-9, OutputCostPerToken: 1e-9, MaxOutputTokens: 100},
		"upstream-mapped": {InputCostPerToken: 1e-3, OutputCostPerToken: 2e-3, MaxOutputTokens: 100},
	}}
	resolver := NewModelPricingResolver(nil, NewBillingService(nil, pricingService))
	memberID := int64(44)
	input := EnterpriseMemberBudgetEstimateInput{
		APIKey:         &APIKey{MemberID: &memberID, Member: &EnterpriseMember{Groups: []Group{{ID: 9, RateMultiplier: 1}}}},
		RequestedModel: "requested",
		Endpoint:       "/v1/responses",
		Body:           []byte(`{"input":"hello","max_output_tokens":10}`),
	}

	requestedOnly := &EnterpriseMemberBudgetService{pricingResolver: resolver}
	requestedCost, err := requestedOnly.estimateUpperBound(context.Background(), input)
	require.NoError(t, err)

	mapped := &EnterpriseMemberBudgetService{
		pricingResolver: resolver,
		accountRepo: &enterpriseMemberBudgetAccountRepoStub{accounts: []Account{{
			ID: 71, Platform: PlatformOpenAI,
			Credentials: map[string]any{"model_mapping": map[string]any{"requested": "upstream-mapped"}},
		}}},
	}
	mappedCost, err := mapped.estimateUpperBound(context.Background(), input)
	require.NoError(t, err)
	require.Greater(t, mappedCost, requestedCost*1000)
}

func TestEnterpriseMemberBudgetFailsClosedForUnpricedMappingWithinPricedGroup(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"requested": {InputCostPerToken: 1e-6, OutputCostPerToken: 1e-6, MaxOutputTokens: 100},
	}}
	memberID := int64(44)
	budgetService := &EnterpriseMemberBudgetService{
		pricingResolver: NewModelPricingResolver(nil, NewBillingService(nil, pricingService)),
		accountRepo: &enterpriseMemberBudgetAccountRepoStub{accounts: []Account{{
			ID: 71, Platform: PlatformOpenAI,
			Credentials: map[string]any{"model_mapping": map[string]any{"requested": "unpriced-upstream-model"}},
		}}},
	}

	_, err := budgetService.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey:         &APIKey{MemberID: &memberID, Member: &EnterpriseMember{Groups: []Group{{ID: 9, RateMultiplier: 1}}}},
		RequestedModel: "requested",
		Endpoint:       "/v1/responses",
		Body:           []byte(`{"input":"hello","max_output_tokens":10}`),
	})

	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetUnbounded)
}

func TestEnterpriseMemberBudgetAlphaSearchUsesPerCallPriceWithoutModelPricing(t *testing.T) {
	memberID := int64(44)
	unitPrice := 0.005
	budgetService := &EnterpriseMemberBudgetService{
		pricingResolver: NewModelPricingResolver(nil, NewBillingService(nil, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})),
	}

	cost, err := budgetService.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey: &APIKey{MemberID: &memberID, Member: &EnterpriseMember{Groups: []Group{{
			ID: 9, RateMultiplier: 2, WebSearchPricePerCall: &unitPrice,
		}}}},
		RequestedModel: "search-model-without-token-pricing",
		Endpoint:       "/v1/alpha/search",
		Body:           []byte(`{"query":"weather"}`),
	})

	require.NoError(t, err)
	require.InDelta(t, 0.0125, cost, 1e-12)
}

func TestResolvedPricingUpperBoundUsesWorstTokenAndTierPrices(t *testing.T) {
	input := 0.02
	output := 0.05
	pricing := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{InputPricePerToken: 0.01, OutputPricePerToken: 0.03},
		Intervals:   []PricingInterval{{InputPrice: &input, OutputPrice: &output}},
	}
	require.InDelta(t, 0.2, resolvedPricingUpperBound(pricing, 5, 2, 1), 1e-12)
}

func TestResolvedPricingUpperBoundMultipliesTokenCostByRequestedCount(t *testing.T) {
	pricing := &ResolvedPricing{
		Mode: BillingModeToken,
		BasePricing: &ModelPricing{
			InputPricePerToken:  0.01,
			OutputPricePerToken: 0.03,
		},
	}

	require.InDelta(t, 0.38, resolvedPricingUpperBound(pricing, 2, 3, 4), 1e-12)
}

func TestEnterpriseMemberOutputTokenUpperBoundUsesConservativeHardCapWhenOmitted(t *testing.T) {
	require.Equal(t, enterpriseMemberMaxOutputTokensUpperBound, enterpriseMemberOutputTokenUpperBound(0, 0))
	require.Equal(t, 64000, enterpriseMemberOutputTokenUpperBound(0, 64000))
	require.Equal(t, 8192, enterpriseMemberOutputTokenUpperBound(8192, 64000))
	require.Equal(t, enterpriseMemberMaxOutputTokensUpperBound, enterpriseMemberOutputTokenUpperBound(0, enterpriseMemberMaxOutputTokensUpperBound+1))
}

func TestEnterpriseMemberBudgetRejectsRequestCountAboveSafeBound(t *testing.T) {
	service := &EnterpriseMemberBudgetService{
		pricingResolver: &ModelPricingResolver{billingService: &BillingService{}},
	}
	memberID := int64(44)
	_, err := service.estimateUpperBound(context.Background(), EnterpriseMemberBudgetEstimateInput{
		APIKey: &APIKey{
			MemberID: &memberID,
			Member: &EnterpriseMember{Groups: []Group{{
				ID:             9,
				RateMultiplier: 1,
			}}},
		},
		RequestedModel: "gpt-5",
		Endpoint:       "/v1/responses",
		Body:           []byte(`{"n":1025,"max_output_tokens":1}`),
	})
	require.ErrorIs(t, err, ErrEnterpriseMemberBudgetUnbounded)
}

func TestEnterpriseMemberVideoUpperBoundIncludesDurationAndCount(t *testing.T) {
	price := 0.10
	group := &Group{VideoPrice720P: &price}
	billingService := &BillingService{}

	require.InDelta(t, 2*15*defaultImageGenerationPrice*1.5, enterpriseMemberVideoUpperBound(billingService, "unknown", group, 2, 15), 1e-12, "unset resolution prices fall back to the model default")
	require.InDelta(t, VideoBillingDefaultDurationSeconds*defaultImageGenerationPrice*1.5, enterpriseMemberVideoUpperBound(billingService, "unknown", group, 1, 0), 1e-12, "omitted duration and prices use upstream defaults")
	require.InDelta(t, 3.75, enterpriseMemberVideoUpperBound(billingService, "grok-imagine-video-1.5", &Group{}, 1, 15), 1e-12, "missing group prices use the most expensive model default")
}

func TestEnterpriseMemberRequestMayExpandInput(t *testing.T) {
	require.False(t, enterpriseMemberRequestMayExpandInput([]byte(`{"input":"plain text"}`)))
	require.True(t, enterpriseMemberRequestMayExpandInput([]byte(`{"previous_response_id":"resp_123"}`)))
	require.True(t, enterpriseMemberRequestMayExpandInput([]byte(`{"input":[{"type":"input_image","image_url":"https://example.com/a.png"}]}`)))
	require.Equal(t, 200000, enterpriseMemberInputTokenUpperBound(200000))
	require.Equal(t, enterpriseMemberMaxInputTokensUpperBound, enterpriseMemberInputTokenUpperBound(0))
}

func TestEnterpriseMemberImageUpperBoundUsesConfiguredAndDefaultPrices(t *testing.T) {
	price := 0.20
	billingService := &BillingService{}
	require.InDelta(t, 2*defaultImageGenerationPrice*2, enterpriseMemberImageUpperBound(billingService, "unknown", &Group{ImagePrice2K: &price}, 2), 1e-12, "unset tiers keep their default prices")
	require.InDelta(t, defaultGrokImagineImageQualityPrice2K, enterpriseMemberImageUpperBound(billingService, "grok-imagine-image-quality", &Group{}, 1), 1e-12)
}

func TestEnterpriseMemberBudgetRequestIDScopesClientIDByKey(t *testing.T) {
	require.Equal(t, "42:client-request", EnterpriseMemberBudgetRequestID(42, " client-request "))
}

func TestNormalizeEnterpriseMemberBudgetRequestIDMatchesUnifiedBilling(t *testing.T) {
	requestID, err := normalizeEnterpriseMemberBudgetRequestID(" request-uuid ")
	require.NoError(t, err)
	require.Equal(t, "client:request-uuid", requestID)

	requestID, err = normalizeEnterpriseMemberBudgetRequestID("client:request-uuid")
	require.NoError(t, err)
	require.Equal(t, "client:request-uuid", requestID)
}

func TestEnterpriseMemberEndpointIsBillableDelegatesAsyncBatchLifecycle(t *testing.T) {
	require.False(t, enterpriseMemberEndpointIsBillable("POST", "/v1/images/batches"))
	require.False(t, enterpriseMemberEndpointIsBillable("GET", "/v1/images/batches/job-1"))
	require.False(t, enterpriseMemberEndpointIsBillable("GET", "/v1/images/tasks/imgtask-1"))
	require.True(t, enterpriseMemberEndpointIsBillable("POST", "/v1/images/generations"))
	require.True(t, enterpriseMemberEndpointIsBillable("POST", "/v1/videos/generations"))
	require.True(t, enterpriseMemberEndpointIsBillable("POST", "/v1/videos/edits"))
	require.True(t, enterpriseMemberEndpointIsBillable("POST", "/v1/videos/extensions"))
	require.False(t, enterpriseMemberEndpointIsBillable("GET", "/v1/videos/video-1"))
	require.False(t, enterpriseMemberEndpointIsBillable("POST", "/v1/messages/count_tokens"))
	require.False(t, enterpriseMemberEndpointIsBillable("POST", "/v1beta/models/gemini-2.5-pro:countTokens"))
	require.False(t, enterpriseMemberEndpointIsBillable("POST", "/v1/responses/input_tokens"))
	require.False(t, enterpriseMemberEndpointIsBillable("GET", "/v1beta/models/gemini-2.5-pro"))
	require.False(t, enterpriseMemberEndpointIsBillable("GET", "/v1beta/models"))
	require.True(t, enterpriseMemberEndpointIsBillable("GET", "/v1/responses"), "responses websocket requests remain billable")
}

func TestEnterpriseMemberCurrentBudgetPeriodUsesShanghaiCalendarBoundary(t *testing.T) {
	start, end := enterpriseMemberCurrentBudgetPeriod(time.Date(2026, time.January, 31, 16, 30, 0, 0, time.UTC))
	require.Equal(t, "2026-02-01T00:00:00+08:00", start.Format(time.RFC3339))
	require.Equal(t, "2026-03-01T00:00:00+08:00", end.Format(time.RFC3339))
}

func TestEnterpriseMemberSystemUsageNoteIsAutomaticAndOnlyWrittenForUsage(t *testing.T) {
	require.Equal(t, "usage values updated by member creation", enterpriseMemberSystemUsageNote(true, "member creation"))
	require.Equal(t, "usage values updated by member editor", enterpriseMemberSystemUsageNote(true, "member editor"))
	require.Empty(t, enterpriseMemberSystemUsageNote(false, "member creation"))
}

type enterpriseMemberBudgetUsageSpy struct {
	EnterpriseMemberBudgetRepository
	note        string
	batchKey    string
	batchDelta  EnterpriseMemberUsageDelta
	batchTarget []EnterpriseMemberBatchTarget
}

func (s *enterpriseMemberBudgetUsageSpy) SetUsage(_ context.Context, _, _ int64, _ time.Time, _, _, _, _ float64, _ int64, _, note string) error {
	s.note = note
	return nil
}

func (s *enterpriseMemberBudgetUsageSpy) BatchAdjustUsage(_ context.Context, _ int64, _ time.Time, targets []EnterpriseMemberBatchTarget, delta EnterpriseMemberUsageDelta, _ int64, key, note string) ([]BatchEnterpriseMemberUsageUpdate, error) {
	s.note = note
	s.batchKey = key
	s.batchDelta = delta
	s.batchTarget = append([]EnterpriseMemberBatchTarget(nil), targets...)
	return []BatchEnterpriseMemberUsageUpdate{{ID: targets[0].ID, MonthlyUsedUSD: 12}}, nil
}

func TestEnterpriseMemberSetUsageSuppliesSystemAuditNoteWhenClientOmitsIt(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	service := NewEnterpriseMemberBudgetService(repo, nil, nil)

	err := service.SetUsage(context.Background(), 7, 11, EnterpriseMemberUsageAdjustmentInput{
		MonthlyUsedUSD: 30,
		Usage5h:        5,
		Usage1d:        10,
		Usage7d:        20,
	}, "member-editor-test")

	require.NoError(t, err)
	require.Equal(t, "usage values updated by member editor", repo.note)
}

func TestEnterpriseMemberBatchAdjustUsageUsesSignedDeltaAndStableLedgerScope(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	updated, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members: []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{
			MonthlyUsedUSD: 4.5,
			Usage5h:        -1,
		},
	}, "batch-usage-request")

	require.NoError(t, err)
	require.Equal(t, []BatchEnterpriseMemberUsageUpdate{{ID: 11, MonthlyUsedUSD: 12}}, updated)
	require.Equal(t, []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}}, repo.batchTarget)
	require.Equal(t, EnterpriseMemberUsageDelta{MonthlyUsedUSD: 4.5, Usage5h: -1}, repo.batchDelta)
	require.Contains(t, repo.batchKey, "usage-batch:7:")
	require.Equal(t, "usage values updated by batch member editor", repo.note)
}

func TestEnterpriseMemberBatchAdjustUsageRejectsEmptyAndDuplicateTargets(t *testing.T) {
	budgetService := NewEnterpriseMemberBudgetService(&enterpriseMemberBudgetUsageSpy{}, nil, nil)

	_, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "batch-empty")
	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)

	_, err = budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members:                    []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}, {ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "batch-duplicate")
	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)
}

func TestEnterpriseMemberBatchAdjustUsageRequiresIdempotencyKey(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	_, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members:                    []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "  ")

	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	require.Empty(t, repo.batchTarget)
}

func TestEnterpriseMemberUsageValidationMatchesDatabaseNumericRange(t *testing.T) {
	require.NoError(t, validateEnterpriseUsageValues(EnterpriseMemberMaxMonetaryValue))
	require.ErrorIs(t, validateEnterpriseUsageValues(EnterpriseMemberMaxMonetaryValue+1), ErrEnterpriseMemberInvalid)
	require.NoError(t, validateEnterpriseUsageDeltas(-EnterpriseMemberMaxMonetaryValue))
	require.ErrorIs(t, validateEnterpriseUsageDeltas(EnterpriseMemberMaxMonetaryValue+1), ErrEnterpriseMemberInvalid)
}
