package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOAuthInputMetadataRemovalIsScopedAndIdempotent(t *testing.T) {
	body := []byte(`{"input":[{"role":"user","content":[{"type":"input_text","text":"internal_chat_message_metadata_passthrough"}],"internal_chat_message_metadata_passthrough":{"secret":true}},"plain",{"type":"function_call_output","output":{"internal_chat_message_metadata_passthrough":"user content"}}],"metadata":{"internal_chat_message_metadata_passthrough":"keep"}}`)
	normalized, changed, err := normalizeOpenAIOAuthResponsesCompatibilityBody(body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "input.0.internal_chat_message_metadata_passthrough").Exists())
	require.Equal(t, gjson.GetBytes(body, "input.0.content").Raw, gjson.GetBytes(normalized, "input.0.content").Raw)
	require.Equal(t, gjson.GetBytes(body, "input.2.output").Raw, gjson.GetBytes(normalized, "input.2.output").Raw)
	require.Equal(t, gjson.GetBytes(body, "metadata").Raw, gjson.GetBytes(normalized, "metadata").Raw)
	again, changed, err := normalizeOpenAIOAuthResponsesCompatibilityBody(normalized)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, normalized, again)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.True(t, normalizeOpenAIOAuthResponsesCompatibilityFields(decoded))
	encoded, err := json.Marshal(decoded)
	require.NoError(t, err)
	require.JSONEq(t, string(normalized), string(encoded))
}
