package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsContainsClaudeFable51(t *testing.T) {
	t.Parallel()

	for _, model := range DefaultModels {
		if model.ID == "claude-fable-5-1" {
			require.Equal(t, "Claude Fable 5.1", model.DisplayName)
			require.Equal(t, "2026-09-01T00:00:00Z", model.CreatedAt)
			return
		}
	}
	t.Fatal("claude-fable-5-1 missing from DefaultModels")
}

func TestDefaultModelsIncludesLatestClaudeCodeModels(t *testing.T) {
	byID := make(map[string]Model)
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}
	for id, name := range map[string]string{
		"claude-fable-5-2":  "Claude Fable 5.2",
		"claude-haiku-5-5":  "Claude Haiku 5.5",
		"claude-sonnet-5-5": "Claude Sonnet 5.5",
		"claude-haiku-4-5":  "Claude Haiku 4.5",
		"claude-sonnet-4-5": "Claude Sonnet 4.5",
		"claude-opus-4-5":   "Claude Opus 4.5",
	} {
		require.Equal(t, name, byID[id].DisplayName, id)
	}
}
