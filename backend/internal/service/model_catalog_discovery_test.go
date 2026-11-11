package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogIncludesDiscoveredModelsAcrossProviders(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo} {
		t.Run(platform, func(t *testing.T) {
			model := "provider-new-model"
			account := Account{Platform: platform, Type: AccountTypeAPIKey}
			account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "upstream", Models: []string{model}})
			require.Contains(t, availableModelIDsFromAccounts([]Account{account}, platform), model)
			account.Credentials = map[string]any{"model_mapping": map[string]any{"my-alias": "provider-old-model"}}
			require.Equal(t, []string{"my-alias"}, availableModelIDsFromAccounts([]Account{account}, platform), "discovery must respect explicit model restrictions")
			account.Credentials["model_mapping"] = map[string]any{"*": "*"}
			require.Equal(t, []string{model}, availableModelIDsFromAccounts([]Account{account}, platform))
		})
	}
}
