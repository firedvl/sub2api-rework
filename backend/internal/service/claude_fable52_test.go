package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFable52PreservesSignedHistoryAndValidatesThinking(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5-2","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed"},{"type":"text","text":"working"}]}],"output_config":{"effort":"high"}}`)
	require.NoError(t, validateClaude55Request(body, "claude-fable-5-2"))
	require.JSONEq(t, string(body), string(FilterThinkingBlocks(body, "claude-fable-5-2")))
	for _, field := range []string{`"thinking":{"type":"enabled","budget_tokens":1000}`, `"thinking":{"type":"disabled"}`, `"tool_choice":{"type":"any"}`, `"temperature":0.5`} {
		require.ErrorContains(t, validateClaude55Request([]byte(`{`+field+`}`), "claude-fable-5-2"), "claude-fable-5-2")
	}
	descriptor := newConfiguredCodexModelDescriptor("claude-fable-5-2")
	require.Equal(t, int64(1_000_000), descriptor.ContextWindow)
	require.Equal(t, "high", *descriptor.DefaultReasoningLevel)
}
