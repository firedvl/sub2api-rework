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

func TestPublicModelsKeepNewProviderReleases(t *testing.T) {
	ids := []string{"claude-fable-5-2", "claude-sonnet-6-1", "claude-haiku-6", "claude-future-7", "ag-claude-sonnet", "claude-sonnet-6-1-thinking", "claude-mythos-5-2"}
	require.Equal(t, ids[:4], FilterPublicModelIDs(ids))
	model, ok := PublicModel("claude-sonnet-6-1")
	require.True(t, ok)
	require.Equal(t, "Claude Sonnet 6.1", model.DisplayName)
}
