package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountEffectiveAPIKeysForModel_SkipsOnlyMatchingModelCooldown(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	statusCode := 400
	account := &Account{
		ID: 1,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
		APIKeys: []AccountAPIKey{
			{
				ID:                   11,
				AccountID:            1,
				Name:                 "key-haiku-cooling",
				APIKey:               "key-1",
				Priority:             10,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "whitelist",
				ModelMapping: map[string]string{
					"claude-haiku-4.5": "claude-haiku-4.5",
					"claude-opus-4.7":  "claude-opus-4.7",
				},
				ModelCooldowns: map[string]AccountAPIKeyModelCooldown{
					"claude-haiku-4.5": {
						UpstreamModel: "claude-haiku-4.5",
						StatusCode:    &statusCode,
						CooldownUntil: now.Add(5 * time.Minute),
					},
				},
			},
			{
				ID:                   12,
				AccountID:            1,
				Name:                 "key-ready",
				APIKey:               "key-2",
				Priority:             20,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "whitelist",
				ModelMapping: map[string]string{
					"claude-haiku-4.5": "claude-haiku-4.5",
					"claude-opus-4.7":  "claude-opus-4.7",
				},
				ModelCooldowns: map[string]AccountAPIKeyModelCooldown{},
			},
		},
	}

	haikuKeys := account.EffectiveAPIKeysForModel("claude-haiku-4.5", now)
	require.Len(t, haikuKeys, 1)
	require.Equal(t, int64(12), haikuKeys[0].ID)

	opusKeys := account.EffectiveAPIKeysForModel("claude-opus-4.7", now)
	require.Len(t, opusKeys, 2)
	require.Equal(t, int64(11), opusKeys[0].ID, "same key remains usable for other upstream models")
}

func TestAccountEffectiveAPIKeysForModel_SortsByPriorityThenLeastRecentlyUsed(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	older := now.Add(-10 * time.Minute)
	newer := now.Add(-1 * time.Minute)
	account := &Account{
		ID: 2,
		APIKeys: []AccountAPIKey{
			{ID: 21, APIKey: "key-newer", Priority: 10, Status: AccountAPIKeyStatusActive, LastUsedAt: &newer, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"claude-haiku-4.5": "claude-haiku-4.5"}},
			{ID: 22, APIKey: "key-older", Priority: 10, Status: AccountAPIKeyStatusActive, LastUsedAt: &older, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"claude-haiku-4.5": "claude-haiku-4.5"}},
			{ID: 23, APIKey: "key-high-priority", Priority: 5, Status: AccountAPIKeyStatusActive, LastUsedAt: &newer, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"claude-haiku-4.5": "claude-haiku-4.5"}},
		},
	}

	keys := account.EffectiveAPIKeysForModel("claude-haiku-4.5", now)
	require.Len(t, keys, 3)
	require.Equal(t, []int64{23, 22, 21}, []int64{keys[0].ID, keys[1].ID, keys[2].ID})
}

func TestAccountEffectiveAPIKeysForModel_FallsBackToLegacyKeyOnlyWhenPoolNotConfigured(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 3,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
	}

	keys := account.EffectiveAPIKeysForModel("claude-haiku-4.5", now)
	require.Len(t, keys, 1)
	require.True(t, keys[0].IsLegacy())
	require.Equal(t, "legacy-key", keys[0].APIKey)
}

func TestAccountEffectiveAPIKeysForModel_DoesNotFallbackToLegacyWhenPoolConfiguredButCooling(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 4,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
		APIKeys: []AccountAPIKey{
			{ID: 31, APIKey: "inactive", Priority: 10, Status: AccountAPIKeyStatusInactive},
		},
	}

	keys := account.EffectiveAPIKeysForModel("claude-haiku-4.5", now)
	require.Empty(t, keys)
}

func TestAccountEffectiveAPIKeySelectionsForRequest_AppliesChildKeyModelMapping(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 5,
		APIKeys: []AccountAPIKey{
			{
				ID:                   41,
				APIKey:               "key-haiku",
				Priority:             10,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "mapping",
				ModelMapping: map[string]string{
					"haiku4.5": "relay-haiku-key-1",
				},
			},
			{
				ID:                   42,
				APIKey:               "key-opus-only",
				Priority:             20,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "mapping",
				ModelMapping: map[string]string{
					"opus4.7": "relay-opus-key-2",
				},
			},
		},
	}

	selections := account.EffectiveAPIKeySelectionsForRequest("haiku4.5", "claude-haiku-4.5", now)
	require.Len(t, selections, 1)
	require.Equal(t, int64(41), selections[0].Key.ID)
	require.Equal(t, "relay-haiku-key-1", selections[0].UpstreamModel)
}

func TestAccountEffectiveAPIKeySelectionsForRequest_AppliesChildKeyWhitelist(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 6,
		APIKeys: []AccountAPIKey{
			{
				ID:                   51,
				APIKey:               "key-sonnet-only",
				Priority:             10,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "whitelist",
				ModelMapping: map[string]string{
					"claude-sonnet-4.6": "claude-sonnet-4.6",
				},
			},
			{
				ID:                   52,
				APIKey:               "key-haiku",
				Priority:             20,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "whitelist",
				ModelMapping: map[string]string{
					"claude-haiku-4.5": "claude-haiku-4.5",
				},
			},
		},
	}

	selections := account.EffectiveAPIKeySelectionsForRequest("haiku4.5", "claude-haiku-4.5", now)
	require.Len(t, selections, 1)
	require.Equal(t, int64(52), selections[0].Key.ID)
	require.Equal(t, "claude-haiku-4.5", selections[0].UpstreamModel)
}

func TestAccountEffectiveAPIKeySelectionsForRequest_ChildKeyMappingCanMatchAccountMappedModel(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 7,
		APIKeys: []AccountAPIKey{
			{
				ID:                   61,
				APIKey:               "key-haiku",
				Priority:             10,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "mapping",
				ModelMapping: map[string]string{
					"claude-haiku-4.5": "relay-haiku-key-1",
				},
			},
		},
	}

	selections := account.EffectiveAPIKeySelectionsForRequest("haiku4.5", "claude-haiku-4.5", now)
	require.Len(t, selections, 1)
	require.Equal(t, "relay-haiku-key-1", selections[0].UpstreamModel)
}

func TestAccountEffectiveAPIKeySelectionsForRequest_EmptyChildRuleIsNotScheduled(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 8,
		APIKeys: []AccountAPIKey{
			{
				ID:                   71,
				APIKey:               "key-empty-whitelist",
				Priority:             10,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "whitelist",
				ModelMapping:         map[string]string{},
			},
			{
				ID:                   72,
				APIKey:               "key-empty-mapping",
				Priority:             20,
				Status:               AccountAPIKeyStatusActive,
				ModelRestrictionMode: "mapping",
				ModelMapping:         map[string]string{},
			},
			{
				ID:       73,
				APIKey:   "key-inherit",
				Priority: 30,
				Status:   AccountAPIKeyStatusActive,
			},
		},
	}

	selections := account.EffectiveAPIKeySelectionsForRequest("haiku4.5", "claude-haiku-4.5", now)
	require.Empty(t, selections)
}
