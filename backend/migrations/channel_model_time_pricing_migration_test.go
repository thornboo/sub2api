package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelModelTimePricingMigration(t *testing.T) {
	content, err := FS.ReadFile("225_channel_model_time_pricing.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE channel_model_pricing ADD COLUMN IF NOT EXISTS time_pricing JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "COMMENT ON COLUMN channel_model_pricing.time_pricing")
}
