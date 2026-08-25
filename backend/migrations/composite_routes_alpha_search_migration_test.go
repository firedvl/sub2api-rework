package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompositeRoutesAlphaSearchMigration(t *testing.T) {
	content, err := FS.ReadFile("231_composite_routes_add_alpha_search.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS composite_model_routes_endpoint_check")
	require.Contains(t, sql,
		"CHECK (endpoint IN ('any', 'messages', 'count_tokens', 'responses', 'alpha_search', 'chat_completions', 'embeddings', 'images', 'gemini'))")
}
