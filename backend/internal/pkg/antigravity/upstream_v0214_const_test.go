package antigravity

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStringConstPreservesEnumIntersectionAndType(t *testing.T) {
	for _, tc := range []struct {
		values   []any
		expected []any
	}{
		{nil, []any{"fixed"}},
		{[]any{"fixed", "other"}, []any{"fixed"}},
		{[]any{"other"}, []any{}},
	} {
		schema := map[string]any{"const": "fixed"}
		if tc.values != nil {
			schema["enum"] = tc.values
		}
		got := CleanJSONSchema(schema)
		require.Equal(t, tc.expected, got["enum"])
		require.Equal(t, "string", got["type"])
		require.NotContains(t, got, "const")
	}
}
