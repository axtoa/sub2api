package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTokenUsageMultiplierMigration(t *testing.T) {
	content, err := FS.ReadFile("242_group_token_usage_multiplier.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_usage_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1.0")
	require.Contains(t, sql, "CHECK (token_usage_multiplier >= 1.0 AND token_usage_multiplier <= 100.0)")
	for _, column := range []string{
		"actual_input_tokens", "actual_output_tokens",
		"actual_cache_creation_tokens", "actual_cache_read_tokens",
		"actual_cache_creation_5m_tokens", "actual_cache_creation_1h_tokens",
		"actual_image_input_tokens", "actual_image_output_tokens",
	} {
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+column+" INTEGER NOT NULL DEFAULT 0")
	}
}
