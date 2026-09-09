package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelAllowlistMigrationFailsClosedOnMissingProvenance(t *testing.T) {
	content, err := FS.ReadFile("244_group_model_allowlist.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "LOCK TABLE groups IN ACCESS EXCLUSIVE MODE")
	require.Contains(t, sql, "IS DISTINCT FROM")
	require.Contains(t, sql, "atttypid = 'jsonb'::regtype")
	require.Contains(t, sql, "attnotnull")
	require.Contains(t, sql, "IS DISTINCT FROM")
	require.False(t, strings.Contains(sql, "RENAME COLUMN"))
	require.False(t, strings.Contains(sql, "SET model_allowlist = models_list_config"))
}

func TestGroupModelAllowlistMigrationKeeps235Through239Immutable(t *testing.T) {
	expected := map[string]string{
		"235_user_restrict_public_groups.sql":       "9867df4258aa7c4e55db96998ed07f4369032c6469dcf146a13c9906bb243514",
		"236_channel_cache_write_1h_pricing.sql":    "62279a094bfd2090c4bc8e54e9fb45fd14ca3c361d6dccd2e068361d053dda80",
		"237_group_force_openai_fast.sql":           "47ac2b5f0c50685538b642a9ecdd1aaa4fcd820390a4b8b87983a2805dbe1c78",
		"238_group_reasoning_effort_over_limit.sql": "b503ea8571f4f16c3f7de0d45b9035693ee15d708e0c19feb30e247646f00bd6",
		"239_group_free_openai_fast.sql":            "80925a7deb54ef23ed5fae1eb1e519764d3952686be7101e2cf739028269f9c3",
	}
	for name, want := range expected {
		content, err := FS.ReadFile(name)
		require.NoError(t, err)
		sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
		require.Equal(t, want, hex.EncodeToString(sum[:]), name)
	}
}
