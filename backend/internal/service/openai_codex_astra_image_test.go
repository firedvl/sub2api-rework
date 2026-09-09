package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAstraWithoutAccountMetadataDoesNotGainVisionOrWorkflowCapabilities(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		t.Run(accountType, func(t *testing.T) {
			account := Account{ID: 7801, Platform: PlatformOpenAI, Type: accountType,
				Credentials: map[string]any{"api_key": "test-key", "chatgpt_account_id": "synthetic-first"}}
			for _, identity := range []string{"synthetic-first", "synthetic-replaced"} {
				account.Credentials["chatgpt_account_id"] = identity
				body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"gpt-6-astra"}, []Account{account}, nil, nil, true)
				require.NoError(t, err)
				model := decodeCodexManifestModels(t, body)[0]
				require.Equal(t, []any{"text"}, model["input_modalities"])
				require.Equal(t, []string{"none"}, effortsFromManifestModel(t, model))
				require.Empty(t, model["service_tiers"])
				require.Nil(t, model["multi_agent_version"])
			}
		})
	}
}

func TestBuildCodexModelsManifestForGroupKeepsAstraTextOnlyWithoutQualification(t *testing.T) {
	t.Parallel()

	const groupID int64 = 780
	account := Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{
		Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {ID: "gpt-6-astra", InputModalities: []string{"text"}},
		},
	})

	svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{
		byGroup: map[int64][]Account{groupID: {account}},
	}}

	body, err := svc.BuildCodexModelsManifestForGroup(
		context.Background(),
		&Group{ID: groupID, Platform: PlatformOpenAI},
		"",
		[]string{"gpt-6-astra"},
	)
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text"}, models[0]["input_modalities"])
}

func TestBuildCodexModelsManifestForGroupPreservesCompatibleAstraTextOnlyMetadata(t *testing.T) {
	t.Parallel()

	const groupID int64 = 781
	account := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://relay.example.test/v1"},
	}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{
		Models: map[string]UpstreamModelMetadata{
			"gpt-6-astra": {ID: "gpt-6-astra", InputModalities: []string{"text"}},
		},
	})

	svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{
		byGroup: map[int64][]Account{groupID: {account}},
	}}

	body, err := svc.BuildCodexModelsManifestForGroup(
		context.Background(),
		&Group{ID: groupID, Platform: PlatformOpenAI},
		"",
		[]string{"gpt-6-astra"},
	)
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text"}, models[0]["input_modalities"])
}
