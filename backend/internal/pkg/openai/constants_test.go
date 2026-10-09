package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPT61SolIdentityAndEffort(t *testing.T) {
	require.NotContains(t, DefaultModelIDs(), "gpt-6.1-sol")
	for _, id := range []string{"gpt-6.1-sol", "openai/gpt-6.1-sol-max", "GPT_6.1_SOL", "gpt-6.1-sol-openai-compact"} {
		require.True(t, IsGPT61SolModelSpelling(id), id)
		require.False(t, IsGPT6SolOrLunaModelSpelling(id), id)
	}
	for _, id := range []string{"gpt-6.1", "gpt-6.1-solitude", "gpt-6.1-sol-preview", "gpt-6-sol"} {
		require.False(t, IsGPT61SolModelSpelling(id), id)
	}
	for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max"} {
		require.NoError(t, ValidateGPT61SolReasoningEffort("gpt-6.1-sol", effort))
	}
	for _, effort := range []string{"none", "minimal", "ultra", "unknown"} {
		require.Error(t, ValidateGPT61SolReasoningEffort("gpt-6.1-sol", effort))
		require.NoError(t, ValidateGPT61SolReasoningEffort("gpt-6-sol", effort))
	}
	require.Contains(t, CodexBaseInstructionsForModel("gpt-6.1-sol"), "based on GPT-6")
}

func TestDefaultModelsIncludeBareGPT56Alias(t *testing.T) {
	require.Contains(t, DefaultModelIDs(), "gpt-5.6")
}

func TestAstraPublicationRequiresDiscovery(t *testing.T) {
	require.NotContains(t, DefaultModelIDs(), "gpt-6-astra")
	require.NotContains(t, DefaultModelIDs(), "gpt-6")
	require.NotContains(t, DefaultModelIDs(), "gpt-6-sol")
	require.NotContains(t, DefaultModelIDs(), "gpt-6-luna")
}

func TestGPT6SolLunaIdentityWithoutStaticPublication(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "openai/GPT-6_SOL", "gpt-6-sol-max", "gpt-6-luna-none"} {
		require.True(t, IsGPT6SolOrLunaModelSpelling(model), model)
	}
	for _, model := range []string{"gpt-6-astra", "gpt-6-solitude", "gpt-6-luna-preview"} {
		require.False(t, IsGPT6SolOrLunaModelSpelling(model), model)
	}
}

func TestDefaultModelsPreferConcreteGPT56SolForAccountTests(t *testing.T) {
	require.NotEmpty(t, DefaultModels)
	require.Equal(t, "gpt-5.6-sol", DefaultModels[0].ID)
}
