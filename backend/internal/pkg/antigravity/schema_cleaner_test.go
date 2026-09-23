package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCleanJSONSchemaTupleAndUnionItems(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tuple": map[string]any{"type": "array", "items": false, "prefixItems": []any{
				map[string]any{"enum": []any{"read", "write"}},
			}},
			"empty": map[string]any{"type": "array", "prefixItems": []any{}},
		},
		"anyOf": []any{map[string]any{"properties": map[string]any{
			"merged": map[string]any{"type": "array", "prefixItems": []any{map[string]any{"type": "integer"}}},
		}}},
	}
	cleaned, err := json.Marshal(CleanJSONSchema(schema))
	require.NoError(t, err)
	for _, name := range []string{"tuple", "empty", "merged"} {
		require.False(t, gjson.GetBytes(cleaned, "properties."+name+".prefixItems").Exists())
		require.True(t, gjson.GetBytes(cleaned, "properties."+name+".items").IsObject())
	}
	require.Equal(t, "string", gjson.GetBytes(cleaned, "properties.tuple.items.type").String())
	require.Equal(t, `["read","write"]`, gjson.GetBytes(cleaned, "properties.tuple.items.enum").Raw)
	require.Equal(t, "string", gjson.GetBytes(cleaned, "properties.empty.items.type").String())
	require.Equal(t, "integer", gjson.GetBytes(cleaned, "properties.merged.items.type").String())
	require.Equal(t, "array", gjson.GetBytes(cleaned, "properties.tuple.type").String())
}

func TestCleanJSONSchemaConstAndMissingArrayItems(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"action": map[string]any{"const": "ping"},
		"list":   map[string]any{"type": "array"},
	}}
	cleaned, err := json.Marshal(CleanJSONSchema(schema))
	require.NoError(t, err)
	require.Equal(t, "string", gjson.GetBytes(cleaned, "properties.action.type").String())
	require.Equal(t, `["ping"]`, gjson.GetBytes(cleaned, "properties.action.enum").Raw)
	require.False(t, gjson.GetBytes(cleaned, "properties.action.const").Exists())
	require.Equal(t, "string", gjson.GetBytes(cleaned, "properties.list.items.type").String())
}
