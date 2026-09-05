package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 固定账号 manifest 配置必须在认证快照 JSON 往返后完整保留，
// 否则开关在缓存路径上会静默失效（投影对账由集成测试兜底，此处覆盖快照序列化）。
func TestAPIKeyAuthSnapshotGroupCodexModelsManifestRoundtrip(t *testing.T) {
	groupID := int64(60)
	apiKey := &APIKey{
		ID: 92, UserID: 46, GroupID: &groupID, Key: "sk-codex-manifest-roundtrip", Status: StatusActive,
		User: &User{ID: 46, Status: StatusActive},
		Group: &Group{
			ID: groupID, Name: "codex-manifest-roundtrip", Platform: PlatformOpenAI, Status: StatusActive,
			Hydrated: true,
			CodexModelsManifestConfig: GroupCodexModelsManifestConfig{
				Enabled:             true,
				AccountIDs:          []int64{7, 8},
				FallbackToScheduler: true,
			},
		},
	}
	svc := &APIKeyService{}

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.CodexModelsManifestConfig.Enabled)
	require.Equal(t, []int64{7, 8}, materialized.Group.CodexModelsManifestConfig.AccountIDs)
	require.True(t, materialized.Group.CodexModelsManifestConfig.FallbackToScheduler)
	require.Equal(t, apiKeyAuthSnapshotVersion, cached.Snapshot.Version)
}

func TestAPIKeyAuthSnapshotMemberGroupCodexModelsManifestRoundtrip(t *testing.T) {
	config := GroupCodexModelsManifestConfig{
		Enabled: true, AccountIDs: []int64{8, 7}, FallbackToScheduler: true,
	}
	memberID := int64(12)
	apiKey := &APIKey{
		ID: 92, UserID: 46, MemberID: &memberID, Key: "sk-member-manifest-roundtrip", Status: StatusActive,
		User: &User{ID: 46, Status: StatusActive},
		Member: &EnterpriseMember{
			ID: memberID, EnterpriseUserID: 46, Status: StatusActive,
			Groups: []Group{{ID: 60, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, CodexModelsManifestConfig: config}},
		},
	}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Nil(t, materialized.Group, "member keys retain request-level group selection")
	require.NotNil(t, materialized.Member)
	require.Equal(t, []int64{60}, materialized.Member.GroupIDs)
	require.Len(t, materialized.Member.Groups, 1)
	require.Equal(t, config, materialized.Member.Groups[0].CodexModelsManifestConfig)
}
