package antigravity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaCleanerNormalizedObjectHasProperties(t *testing.T) {
	for _, schemaType := range []any{[]any{"object", "null"}, "OBJECT"} {
		cleaned := CleanJSONSchema(map[string]any{"type": schemaType})
		require.Equal(t, "object", cleaned["type"])
		require.NotEmpty(t, cleaned["properties"])
		require.Equal(t, []any{"reason"}, cleaned["required"])
	}
}

func TestSchemaCleanerEmptyEnumRemainsScalar(t *testing.T) {
	cleaned := CleanJSONSchema(map[string]any{"enum": []any{}})
	require.Equal(t, "string", cleaned["type"])
	require.NotContains(t, cleaned, "properties")
}

func TestSchemaCleanerEnumOnlyTupleBeatsNull(t *testing.T) {
	cleaned := CleanJSONSchema(map[string]any{
		"type": "array",
		"prefixItems": []any{
			map[string]any{"type": "null"},
			map[string]any{"enum": []any{"read", "write"}},
		},
	})
	items, ok := cleaned["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"read", "write"}, items["enum"])
	require.NotContains(t, cleaned, "prefixItems")
}
