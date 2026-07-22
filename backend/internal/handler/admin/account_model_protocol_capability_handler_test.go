package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSupportsModelProtocolCapabilityManagementOnlyForOpenAIAPIKeys(t *testing.T) {
	t.Parallel()
	require.True(t, supportsModelProtocolCapabilityManagement(&service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
	}))
	require.False(t, supportsModelProtocolCapabilityManagement(&service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
	}))
	require.False(t, supportsModelProtocolCapabilityManagement(&service.Account{
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
	}))
	require.False(t, supportsModelProtocolCapabilityManagement(nil))
}
