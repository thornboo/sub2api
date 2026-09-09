package service

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
)

func TestResolveAccountTestModelMatrix(t *testing.T) {
	for _, tt := range []struct {
		name    string
		account *Account
		modelID string
		want    string
	}{
		{
			name: "anthropic apikey maps once",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeAPIKey, map[string]any{
				"a-model": "b-model",
				"b-model": "c-model",
			}, nil),
			modelID: "a-model",
			want:    "b-model",
		},
		{
			name: "anthropic mapping preserves raw upstream target whitespace",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeAPIKey, map[string]any{
				"spaced-model": " upstream-model ",
			}, nil),
			modelID: "spaced-model",
			want:    " upstream-model ",
		},
		{
			name: "anthropic oauth explicit mapping overrides normalize",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeOAuth, map[string]any{
				"claude-3-5-sonnet-latest": "custom-upstream",
			}, nil),
			modelID: "claude-3-5-sonnet-latest",
			want:    "custom-upstream",
		},
		{
			name:    "anthropic setup token preserves requested model without mapping",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeSetupToken, nil, nil),
			modelID: "claude-3-5-sonnet-latest",
			want:    "claude-3-5-sonnet-latest",
		},
		{
			name: "vertex explicit mapping skips dated normalization",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeServiceAccount, map[string]any{
				"claude-sonnet-4-5-20250929": "custom-vertex-target",
			}, nil),
			modelID: "claude-sonnet-4-5-20250929",
			want:    "custom-vertex-target",
		},
		{
			name:    "vertex normalizes dated fallback",
			account: accountTestModelAccount(PlatformAnthropic, AccountTypeServiceAccount, nil, nil),
			modelID: "claude-sonnet-4-5-20250929",
			want:    "claude-sonnet-4-5@20250929",
		},
		{
			name: "openai apikey maps once",
			account: accountTestModelAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
				"a-model": "b-model",
				"b-model": "c-model",
			}, nil),
			modelID: "a-model",
			want:    "b-model",
		},
		{
			name:    "openai oauth normalizes codex alias",
			account: accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, nil, nil),
			modelID: "gpt-5.6-max",
			want:    "gpt-5.6-sol",
		},
		{
			name: "openai setup token maps before normalize",
			account: accountTestModelAccount(PlatformOpenAI, AccountTypeSetupToken, map[string]any{
				"public-codex": "gpt-5.6-max",
			}, nil),
			modelID: "public-codex",
			want:    "gpt-5.6-sol",
		},
		{
			name: "openai image model bypasses codex text normalization",
			account: accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{
				"image-public": "gpt-image-2",
			}, nil),
			modelID: "image-public",
			want:    "gpt-image-2",
		},
		{
			name: "openai passthrough ignores explicit mapping in default preview",
			account: accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{
				"public-codex": "gpt-5.6-max",
			}, map[string]any{"openai_passthrough": true}),
			modelID: "public-codex",
			want:    "gpt-5.3-codex",
		},
		{
			name: "gemini apikey wildcard mapping",
			account: accountTestModelAccount(PlatformGemini, AccountTypeAPIKey, map[string]any{
				"gemini-custom-*": "gemini-2.5-pro",
			}, nil),
			modelID: "gemini-custom-a",
			want:    "gemini-2.5-pro",
		},
		{
			name: "gemini oauth explicit mapping",
			account: accountTestModelAccount(PlatformGemini, AccountTypeOAuth, map[string]any{
				"gemini-public": "gemini-2.5-flash",
			}, nil),
			modelID: "gemini-public",
			want:    "gemini-2.5-flash",
		},
		{
			name: "gemini service account mapping",
			account: accountTestModelAccount(PlatformGemini, AccountTypeServiceAccount, map[string]any{
				"gemini-public": "gemini-2.5-pro",
			}, nil),
			modelID: "gemini-public",
			want:    "gemini-2.5-pro",
		},
		{
			name:    "kimi empty model uses platform preset",
			account: accountTestModelAccount(PlatformKimi, AccountTypeAPIKey, nil, nil),
			modelID: "",
			want:    "kimi-k2.5",
		},
		{
			name:    "zhipu empty model uses platform preset",
			account: accountTestModelAccount(PlatformZhipu, AccountTypeAPIKey, nil, nil),
			modelID: "",
			want:    "glm-4.7",
		},
		{
			name:    "deepseek empty model uses platform preset",
			account: accountTestModelAccount(PlatformDeepseek, AccountTypeAPIKey, nil, nil),
			modelID: "",
			want:    "deepseek-v4-pro",
		},
		{
			name: "cn provider maps after platform default",
			account: accountTestModelAccount(PlatformKimi, AccountTypeAPIKey, map[string]any{
				"kimi-k2.5": "kimi-upstream",
			}, nil),
			modelID: "",
			want:    "kimi-upstream",
		},
		{
			name: "grok default mapping",
			account: accountTestModelAccount(PlatformGrok, AccountTypeOAuth, map[string]any{
				"grok-public": "grok-4.5",
			}, nil),
			modelID: "grok-public",
			want:    "grok-4.5",
		},
		{
			name: "grok mapping trims target like media test path",
			account: accountTestModelAccount(PlatformGrok, AccountTypeOAuth, map[string]any{
				"grok-public": " grok-imagine ",
			}, nil),
			modelID: "grok-public",
			want:    "grok-imagine-image-quality",
		},
		{
			name:    "grok media alias normalizes for default preview",
			account: accountTestModelAccount(PlatformGrok, AccountTypeAPIKey, nil, nil),
			modelID: "grok-imagine",
			want:    "grok-imagine-image-quality",
		},
		{
			name: "antigravity apikey returns mapped upstream",
			account: accountTestModelAccount(PlatformAntigravity, AccountTypeAPIKey, map[string]any{
				"claude-public": "gemini-2.5-pro",
			}, nil),
			modelID: "claude-public",
			want:    "gemini-2.5-pro",
		},
		{
			name: "antigravity oauth returns mapped upstream",
			account: accountTestModelAccount(PlatformAntigravity, AccountTypeOAuth, map[string]any{
				"gemini-public": "claude-sonnet-4-6",
			}, nil),
			modelID: "gemini-public",
			want:    "claude-sonnet-4-6",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAccountTestModel(tt.account, tt.modelID)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveAccountTestModelRejectsInvalidResolvedTargets(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mapping map[string]any
		wantErr string
	}{
		{
			name:    "whitespace only target",
			mapping: map[string]any{"public": "   "},
			wantErr: "empty upstream model",
		},
		{
			name:    "wildcard target",
			mapping: map[string]any{"public": "upstream-*"},
			wantErr: "wildcard upstream model",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := accountTestModelAccount(PlatformAnthropic, AccountTypeAPIKey, tt.mapping, nil)

			_, err := ResolveAccountTestModel(account, "public")

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestResolveAccountTestModelRejectsOpenAIInvalidMappedTargetBeforeNormalize(t *testing.T) {
	for _, tt := range []struct {
		name    string
		target  string
		wantErr string
	}{
		{name: "empty", target: "   ", wantErr: "empty upstream model"},
		{name: "wildcard", target: "bad-*", wantErr: "wildcard upstream model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{"public": tt.target}, nil)

			_, err := ResolveAccountTestModel(account, "public")

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestResolveAccountTestModelBedrock(t *testing.T) {
	account := accountTestModelAccount(PlatformAnthropic, AccountTypeBedrock, nil, map[string]any{
		"aws_region": "us-east-1",
	})

	got, err := ResolveAccountTestModel(account, "claude-sonnet-4-5")

	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Contains(t, got, "anthropic.claude")
}

func TestResolveAccountTestModelRejectsUnsupportedBedrockModel(t *testing.T) {
	account := accountTestModelAccount(PlatformAnthropic, AccountTypeBedrock, nil, map[string]any{
		"aws_region": "us-east-1",
	})

	_, err := ResolveAccountTestModel(account, "unknown-bedrock-model")

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported Bedrock model")
}

func TestAccountTestUsesModelMapping(t *testing.T) {
	require.False(t, AccountTestUsesModelMapping(nil))
	require.False(t, AccountTestUsesModelMapping(accountTestModelAccount(PlatformAnthropic, AccountTypeAPIKey, nil, nil)))
	require.True(t, AccountTestUsesModelMapping(accountTestModelAccount(PlatformAnthropic, AccountTypeOAuth, map[string]any{"a": "b"}, nil)))
	require.False(t, AccountTestUsesModelMapping(accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{"a": "b"}, map[string]any{"openai_passthrough": true})))
	require.True(t, AccountTestUsesModelMapping(accountTestModelAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{"a": "b"}, nil)))
	require.True(t, AccountTestUsesModelMapping(accountTestModelAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{"a": "b"}, map[string]any{
		"openai_passthrough":                true,
		openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
	})))
}

func accountTestModelAccount(platform, accountType string, mapping map[string]any, extra map[string]any) *Account {
	credentials := map[string]any{}
	if mapping != nil {
		credentials["model_mapping"] = mapping
	}
	if platform == PlatformAnthropic && accountType == AccountTypeBedrock {
		credentials["aws_region"] = "us-east-1"
	}
	for key, value := range extra {
		if strings.HasPrefix(key, "aws_") {
			credentials[key] = value
		}
	}
	return &Account{
		ID:          1001,
		Name:        platform + "-" + accountType,
		Platform:    platform,
		Type:        accountType,
		Credentials: credentials,
		Extra:       extra,
	}
}
