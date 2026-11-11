package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOpenAIOAuthInventoryDoesNotOverrideManifestVisibility(t *testing.T) {
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "upstream", Models: []string{"gpt-public", "gpt-hidden"}})
	for _, published := range [][]string{{"gpt-public"}, nil} {
		models := availableModelIDsFromAccountsWithManifestIDs([]Account{account}, PlatformOpenAI, func(*Account) []string { return published })
		require.Equal(t, published, models)
	}
	account.Credentials = map[string]any{"model_mapping": map[string]any{"my-alias": "gpt-hidden"}}
	models := availableModelIDsFromAccountsWithManifestIDs([]Account{account}, PlatformOpenAI, func(*Account) []string { return []string{"gpt-public"} })
	require.Equal(t, []string{"my-alias"}, models)
}
