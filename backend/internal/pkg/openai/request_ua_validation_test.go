package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPairCodexClientIdentityRejectsInvalidHeaderBytes(t *testing.T) {
	for _, ua := range []string{
		"codex-tui/0.146.0 (Linux)\r\nX-Injected: value",
		"codex-tui/0.146.0 (Linux\x00)",
		"codex-tui/0.146.0 terminal\x7f",
		"\ncodex-tui/0.146.0",
		"codex-tui/0.146.0\n",
		"custom/0.146.0 bad\x01 (codex-tui; 0.146.0)",
	} {
		originator, paired, ok := PairCodexClientIdentity(ua)
		require.False(t, ok, ua)
		require.Empty(t, originator)
		require.Empty(t, paired)
	}
	for _, ua := range []string{
		"codex-tui/0.146.0 (Mac OS X 14.0; arm64) iTerm",
		"codex-tui/0.146.0 (Windows NT 10.0; x86_64) terminal",
		"codex-tui/0.146.0 (Linux)\tterminal",
	} {
		_, paired, ok := PairCodexClientIdentity(ua)
		require.True(t, ok)
		require.Equal(t, ua, paired)
	}
}
