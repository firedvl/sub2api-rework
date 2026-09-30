package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestCatalogProjectionPreservesDisplayNamesOnly(test *testing.T) {
	body, err := standardOpenAIModelsBody([]byte(`{"models":[{"slug":"custom","display_name":" Custom Name ","instructions":"private prompt","model_messages":{"secret":"private"}},{"slug":"blank","display_name":" "},{"slug":"non-string","display_name":7}]}`), true)
	require.NoError(test, err)
	var catalog struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(test, json.Unmarshal(body, &catalog))
	require.Len(test, catalog.Data, 3)
	require.Equal(test, "Custom Name", catalog.Data[0]["display_name"])
	require.NotContains(test, string(body), "private")
	require.NotContains(test, catalog.Data[1], "display_name")
	require.NotContains(test, catalog.Data[2], "display_name")
}

func TestDeepSeekVisionManifestProjection(test *testing.T) {
	const model = "deepseek-v4-flash-vision-exp"
	newAccount := func(id int64, platform, upstream string, modalities []string) Account {
		account := Account{ID: id, Platform: platform, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"model_mapping": map[string]any{"vision-alias": upstream}}}
		if modalities != nil {
			account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
				upstream: {ID: upstream, InputModalities: modalities},
			}})
		}
		return account
	}
	for _, scenario := range []struct {
		name, platform string
		accounts       []Account
		want           []any
	}{
		{"native", PlatformDeepseek, []Account{newAccount(1, PlatformDeepseek, model, nil)}, []any{"text", "image"}},
		{"compatible", PlatformOpenAI, []Account{newAccount(1, PlatformOpenAI, model, nil)}, []any{"text", "image"}},
		{"composite", PlatformComposite, []Account{newAccount(1, PlatformDeepseek, model, nil)}, []any{"text", "image"}},
		{"text_model", PlatformDeepseek, []Account{newAccount(1, PlatformDeepseek, "deepseek-v4-flash", nil)}, []any{"text"}},
		{"metadata_override", PlatformDeepseek, []Account{newAccount(1, PlatformDeepseek, model, []string{"text"})}, []any{"text"}},
		{"mixed", PlatformDeepseek, []Account{newAccount(1, PlatformDeepseek, model, nil), newAccount(2, PlatformDeepseek, "deepseek-v4-flash", nil)}, []any{"text"}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{790: scenario.accounts}}}
			body, err := svc.BuildCodexModelsManifestForGroup(context.Background(), &Group{ID: 790, Platform: scenario.platform}, "", []string{"vision-alias"})
			require.NoError(test, err)
			models := decodeCodexManifestModels(test, body)
			require.Len(test, models, 1)
			require.Equal(test, scenario.want, models[0]["input_modalities"])
		})
	}
}
