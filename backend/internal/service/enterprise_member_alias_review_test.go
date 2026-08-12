package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEnterpriseMemberAliasReviewModelRejectsPollution(t *testing.T) {
	_, _, err := NormalizeEnterpriseMemberAliasReviewModel("   ")
	require.Error(t, err)

	_, _, err = NormalizeEnterpriseMemberAliasReviewModel("bad\nmodel")
	require.Error(t, err)

	tooLong := make([]byte, EnterpriseMemberAliasReviewMaxModelLen+1)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	_, _, err = NormalizeEnterpriseMemberAliasReviewModel(string(tooLong))
	require.Error(t, err)

	model, norm, err := NormalizeEnterpriseMemberAliasReviewModel("  GPT-Exact  ")
	require.NoError(t, err)
	require.Equal(t, "GPT-Exact", model)
	require.Equal(t, "gpt-exact", norm)
}

func TestEnterpriseMemberAliasReviewServiceRegisteredRequiresExactStableRoute(t *testing.T) {
	groupID := int64(12)
	channelID := int64(34)
	svc := NewEnterpriseMemberAliasReviewService(
		&aliasReviewRepoFake{},
		&aliasReviewPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			groupID: {GroupID: groupID, ChannelID: channelID, PublicModel: "alias", ChannelMappedModel: "upstream", Source: EnterpriseMemberRoutePublicationExactMapping},
		}},
		&aliasReviewDeliveryFake{projection: &ModelDeliveryProjection{Models: map[string]map[int64]*ModelDeliveryGroupProjection{
			"alias": {groupID: {PublicModel: "alias", GroupID: groupID, Routes: []ModelDeliveryRoute{{Decisions: map[ModelProtocol]ModelDeliveryDecision{
				ModelProtocolOpenAIResponses: {Eligible: true},
			}}}, Endpoints: map[ModelProtocol]ModelDeliveryMode{
				ModelProtocolOpenAIResponses: ModelDeliveryModeNative,
			}}},
		}}},
		&aliasReviewGroupRepoFake{group: &Group{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}},
	)

	record, err := svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "alias", Endpoint: "/v1/responses", Status: EnterpriseMemberAliasReviewStatusRegistered,
		FinalGroupID: &groupID, ChannelID: &channelID,
	}, 77)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberAliasReviewStatusRegistered, record.Status)
	require.JSONEq(t, `{"publication_source":"exact_mapping","channel_id":34,"channel_mapped_model":"upstream","inbound_protocol":"openai_responses","route_count":1,"validated_at":"ignored"}`, maskValidatedAt(t, record.ValidationEvidence))
}

func TestEnterpriseMemberAliasReviewServiceRegisteredRejectsWildcardAndDoesNotWrite(t *testing.T) {
	groupID := int64(12)
	repo := &aliasReviewRepoFake{}
	svc := NewEnterpriseMemberAliasReviewService(
		repo,
		&aliasReviewPublicationFake{items: map[int64]EnterpriseMemberRoutePublication{
			groupID: {GroupID: groupID, ChannelID: 34, PublicModel: "alias", ChannelMappedModel: "upstream", Source: EnterpriseMemberRoutePublicationWildcardMappingPricedExpansion},
		}},
		&aliasReviewDeliveryFake{},
		&aliasReviewGroupRepoFake{group: &Group{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}},
	)

	_, err := svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "alias", Endpoint: "/v1/responses", Status: EnterpriseMemberAliasReviewStatusRegistered,
		FinalGroupID: &groupID,
	}, 77)
	require.Error(t, err)
	require.False(t, repo.wrote)
}

func TestEnterpriseMemberAliasReviewServiceNonRegisteredDoesNotValidateRoute(t *testing.T) {
	repo := &aliasReviewRepoFake{}
	svc := NewEnterpriseMemberAliasReviewService(repo, nil, nil, nil)
	record, err := svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "typo-model", Status: EnterpriseMemberAliasReviewStatusRejectedInvalid,
	}, 77)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberAliasReviewStatusRejectedInvalid, record.Status)
	require.True(t, repo.wrote)
}

func TestEnterpriseMemberAliasReviewServiceRejectsControlAndLongReviewNotes(t *testing.T) {
	repo := &aliasReviewRepoFake{}
	svc := NewEnterpriseMemberAliasReviewService(repo, nil, nil, nil)

	_, err := svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "typo-model", Status: EnterpriseMemberAliasReviewStatusRejectedInvalid,
		ReviewNote: "operator note\ninjected header",
	}, 77)
	require.ErrorContains(t, err, "review note contains control characters")
	require.False(t, repo.wrote)

	_, err = svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "typo-model", Status: EnterpriseMemberAliasReviewStatusRejectedInvalid,
		ReviewNote: "safe prefix\u202edoc.exe",
	}, 77)
	require.ErrorContains(t, err, "review note contains control characters")
	require.False(t, repo.wrote)

	longNote := strings.Repeat("a", EnterpriseMemberAliasReviewMaxNoteLen+1)
	_, err = svc.Review(context.Background(), EnterpriseMemberAliasReviewRequest{
		PublicModel: "typo-model", Status: EnterpriseMemberAliasReviewStatusRejectedInvalid,
		ReviewNote: longNote,
	}, 77)
	require.ErrorContains(t, err, "review note is too long")
	require.False(t, repo.wrote)
}

func maskValidatedAt(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	payload["validated_at"] = "ignored"
	out, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(out)
}

type aliasReviewRepoFake struct {
	wrote bool
}

func (f *aliasReviewRepoFake) ListLegacySuccessNewPruned(context.Context, EnterpriseMemberAliasReviewListInput) ([]EnterpriseMemberAliasReviewItem, error) {
	return nil, nil
}

func (f *aliasReviewRepoFake) UpsertReview(_ context.Context, input EnterpriseMemberAliasReviewUpsert) (*EnterpriseMemberAliasReviewRecord, error) {
	f.wrote = true
	return &EnterpriseMemberAliasReviewRecord{
		PublicModel: input.PublicModel, PublicModelNorm: input.PublicModelNorm, Endpoint: input.Endpoint,
		Status: input.Status, FinalGroupID: input.FinalGroupID, ChannelID: input.ChannelID,
		ValidationEvidence: input.ValidationEvidence,
	}, nil
}

func (f *aliasReviewRepoFake) GetReadinessSummary(context.Context, time.Time) (*EnterpriseMemberAliasReviewReadinessSummary, error) {
	return nil, nil
}

type aliasReviewPublicationFake struct {
	err   error
	items map[int64]EnterpriseMemberRoutePublication
}

func (f *aliasReviewPublicationFake) ResolveEnterpriseMemberRoutePublications(_ context.Context, groupIDs []int64, _ string) (map[int64]EnterpriseMemberRoutePublication, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[int64]EnterpriseMemberRoutePublication)
	for _, id := range groupIDs {
		if item, ok := f.items[id]; ok {
			out[id] = item
		}
	}
	return out, nil
}

type aliasReviewDeliveryFake struct {
	err        error
	projection *ModelDeliveryProjection
}

func (f *aliasReviewDeliveryFake) ResolveForEnterpriseMemberRoute(context.Context, []*Group, string, ModelProtocol) (*ModelDeliveryProjection, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.projection == nil {
		return &ModelDeliveryProjection{Models: map[string]map[int64]*ModelDeliveryGroupProjection{}}, nil
	}
	return f.projection, nil
}

type aliasReviewGroupRepoFake struct {
	GroupRepository
	group *Group
}

func (f *aliasReviewGroupRepoFake) GetByID(context.Context, int64) (*Group, error) {
	if f.group == nil {
		return nil, errors.New("not found")
	}
	return f.group, nil
}
