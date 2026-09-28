package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexTurnMetadataRewritesRemainASCIIAndRoundTrip(t *testing.T) {
	const original = `{"note":"\u4e2d\u6587\ud83d\ude80\u007f","keep":{"value":"quoted\\text"}}`
	header := http.Header{}
	header.Set(openAIWSTurnMetadataHeader, original)
	rewriteCodexTurnMetadataFields(header, map[string]any{"identity": "test"})
	got := header.Get(openAIWSTurnMetadataHeader)
	for _, b := range []byte(got) {
		require.Less(t, b, byte(0x7f))
	}
	var before, after map[string]any
	require.NoError(t, json.Unmarshal([]byte(original), &before))
	require.NoError(t, json.Unmarshal([]byte(got), &after))
	require.Equal(t, before["note"], after["note"])
	require.Equal(t, before["keep"], after["keep"])
	require.Equal(t, "test", after["identity"])
	embedded := map[string]any{"x-codex-turn-metadata": original}
	rewriteClientMetadataEmbeddedTurnMetadata(embedded, map[string]any{"identity": "test"})
	require.Equal(t, got, embedded["x-codex-turn-metadata"])
}
