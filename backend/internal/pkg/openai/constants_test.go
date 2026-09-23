package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
