package service

import (
	"context"
	"errors"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type configuredGroupModelsRepoStub struct {
	AccountRepository

	accounts        []Account
	err             error
	gotGroupID      *int64
	gotPlatforms    []string
	gotIncludeGroup bool
	calls           int
}

func (s *configuredGroupModelsRepoStub) ListModelAvailabilityCandidates(
	_ context.Context,
	groupID *int64,
	platforms []string,
	includeGrouped bool,
) ([]Account, error) {
	s.calls++
	if groupID != nil {
		copied := *groupID
		s.gotGroupID = &copied
	}
	s.gotPlatforms = append([]string(nil), platforms...)
	s.gotIncludeGroup = includeGrouped
	if s.err != nil {
		return nil, s.err
	}
	return append([]Account(nil), s.accounts...), nil
}

func TestGetConfiguredGroupModelsDistinguishesEmptyPoolFromDefaultModelAccounts(t *testing.T) {
	groupID := int64(91)

	t.Run("no persistently enabled account", func(t *testing.T) {
		repo := &configuredGroupModelsRepoStub{}
		svc := &GatewayService{accountRepo: repo}

		models, hasAccounts, err := svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)

		require.NoError(t, err)
		require.False(t, hasAccounts)
		require.Empty(t, models)
		require.NotNil(t, repo.gotGroupID)
		require.Equal(t, groupID, *repo.gotGroupID)
		require.Equal(t, []string{PlatformOpenAI}, repo.gotPlatforms)
		require.False(t, repo.gotIncludeGroup)
	})

	t.Run("enabled account without mapping requests platform defaults", func(t *testing.T) {
		repo := &configuredGroupModelsRepoStub{
			accounts: []Account{{ID: 1, Platform: PlatformOpenAI}},
		}
		svc := &GatewayService{accountRepo: repo}

		models, hasAccounts, err := svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)

		require.NoError(t, err)
		require.True(t, hasAccounts)
		require.Empty(t, models)
	})

	t.Run("mapped models are aggregated, sorted, and deduplicated", func(t *testing.T) {
		repo := &configuredGroupModelsRepoStub{
			accounts: []Account{
				{ID: 1, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"z-model": "upstream-z", "a-model": "upstream-a"}}},
				{ID: 2, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"a-model": "other-a"}}},
			},
		}
		svc := &GatewayService{accountRepo: repo}

		models, hasAccounts, err := svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)

		require.NoError(t, err)
		require.True(t, hasAccounts)
		require.Equal(t, []string{"a-model", "z-model"}, models)
	})
}

func TestGetConfiguredGroupModelsReturnsRepositoryErrorsWithoutAdvertisingFallbacks(t *testing.T) {
	repoErr := errors.New("database unavailable")
	svc := &GatewayService{accountRepo: &configuredGroupModelsRepoStub{err: repoErr}}

	groupID := int64(92)
	models, hasAccounts, err := svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformAnthropic)

	require.ErrorIs(t, err, repoErr)
	require.False(t, hasAccounts)
	require.Empty(t, models)
}

func TestConfiguredGroupModelsCachePreservesStateWhileFreshLookupSeesAdministrativeDisableImmediately(t *testing.T) {
	groupID := int64(93)
	repo := &configuredGroupModelsRepoStub{
		accounts: []Account{{
			ID:       1,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{"gpt-before-disable": "upstream"},
			},
		}},
	}
	svc := &GatewayService{
		accountRepo:           repo,
		configuredModelsCache: gocache.New(time.Minute, time.Minute),
		modelsListCacheTTL:    time.Minute,
	}

	models, hasAccounts, err := svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.True(t, hasAccounts)
	require.Equal(t, []string{"gpt-before-disable"}, models)
	require.Equal(t, 1, repo.calls)

	repo.accounts = nil
	models, hasAccounts, err = svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.True(t, hasAccounts)
	require.Equal(t, []string{"gpt-before-disable"}, models)
	require.Equal(t, 1, repo.calls, "cached discovery must not poll the database on every request")

	models, hasAccounts, err = svc.GetConfiguredGroupModelsFresh(context.Background(), &groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.False(t, hasAccounts)
	require.Empty(t, models)
	require.Equal(t, 2, repo.calls, "fresh Key usage lookup must observe the disabled pool immediately")

	svc.InvalidateAvailableModelsCache(&groupID, PlatformOpenAI)
	models, hasAccounts, err = svc.GetConfiguredGroupModels(context.Background(), &groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.False(t, hasAccounts)
	require.Empty(t, models)
	require.Equal(t, 3, repo.calls)
}
