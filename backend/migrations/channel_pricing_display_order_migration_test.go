package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelPricingDisplayOrderMigrationSeparatesPresentationFromMatching(t *testing.T) {
	content, err := FS.ReadFile("198_channel_pricing_display_order.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "model_mapping_order JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "sort_order INTEGER NOT NULL DEFAULT 0")
	require.Contains(t, sql, "PARTITION BY channel_id, platform ORDER BY id")
	require.Contains(t, sql, "does not affect matching precedence")
	require.NotContains(t, sql, "DROP COLUMN")
}
