package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicModelsHideInternalAndRetiredClaudeIDs(t *testing.T) {
	ids := []string{"ag-claude-sonnet", "claude-sonnet-4-5-thinking", "claude-haiku-4-5-20251001", "claude-haiku-4-5", "claude-3-5-sonnet-20241022", "claude-nonexistent", "claude-haiku-5-5", "gemini-2.5-flash", "custom-route"}
	require.Equal(t, []string{"claude-haiku-4-5", "claude-haiku-5-5", "gemini-2.5-flash", "custom-route"}, FilterPublicModelIDs(ids))
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, EffortLevelsForModel("claude-haiku-5-5"))
	require.True(t, IsHaiku55("anthropic/claude-haiku-5.5"))
	require.False(t, IsHaiku55("claude-haiku-5-5-preview"))
}
