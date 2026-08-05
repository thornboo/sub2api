package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelProtocolCapabilitiesMigrationKeepsFactsAndPolicySeparate(t *testing.T) {
	content, err := FS.ReadFile("197_account_model_protocol_capabilities.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "UNIQUE (account_id, upstream_model, protocol)")
	require.Contains(t, sql, "'anthropic_messages'")
	require.Contains(t, sql, "'openai_chat_completions'")
	require.Contains(t, sql, "'openai_responses'")
	require.Contains(t, sql, "'auto', 'supported', 'unsupported'")
	require.Contains(t, sql, "extra ->> 'openai_responses_supported'")
	require.NotContains(t, sql, "openai_responses_mode")
}

func TestModelProtocolCapabilitiesNonTextMigrationExtendsProtocolCheck(t *testing.T) {
	content, err := FS.ReadFile("202_account_model_protocol_capabilities_non_text.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS account_model_protocol_capability_protocol_check")
	for _, protocol := range []string{
		"anthropic_messages",
		"openai_chat_completions",
		"openai_responses",
		"openai_embeddings",
		"openai_images",
		"openai_live",
		"batch_images",
		"grok_video",
		"gemini_native",
	} {
		require.Contains(t, sql, "'"+protocol+"'")
	}
}
