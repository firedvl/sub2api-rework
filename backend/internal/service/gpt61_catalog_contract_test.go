package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

const gpt61ContractCatalog = `{"models":[{"slug":"gpt-6.1-sol","supported_in_api":true,"default_reasoning_level":"ultra","supported_reasoning_levels":[{"effort":"low","description":"provider low"},{"effort":"medium"},{"effort":"high"},{"effort":"xhigh"},{"effort":"max"},{"effort":"ultra"},{"effort":"none"},{"effort":"minimal"}],"future_metadata":{"preserved":true}}]}`

func assertGPT61CatalogContract(t *testing.T, body []byte) {
	t.Helper()
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	model := models[0]
	efforts := effortsFromManifestModel(t, model)
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, efforts)
	require.Contains(t, efforts, model["default_reasoning_level"])
	for _, effort := range efforts {
		for _, field := range []string{`"reasoning":{"effort":%q}`, `"reasoning_effort":%q`, `"output_config":{"effort":%q}`} {
			request := []byte(`{"model":"public-sol",` + fmt.Sprintf(field, effort) + `,"stream":true}`)
			require.NoError(t, validateGPT61SolCompatRequest(request, "gpt-6.1-sol"))
		}
	}
	for _, effort := range []string{"ultra", "none", "minimal", "unknown"} {
		require.Error(t, validateGPT61SolCompatRequest([]byte(fmt.Sprintf(`{"model":"public-sol","reasoning":{"effort":%q}}`, effort)), "gpt-6.1-sol"))
	}
}

func TestGPT61CatalogContractSyncedGroup(t *testing.T) {
	_, metadata, err := extractUpstreamModelCatalog([]byte(gpt61ContractCatalog), false)
	require.NoError(t, err)
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		account := Account{ID: 1, Platform: PlatformOpenAI, Type: accountType, Status: StatusActive, Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"public-sol": "gpt-6.1-sol"}}}
		account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: metadata})
		body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-sol"}, []Account{account}, nil, nil, true)
		require.NoError(t, err)
		// The original production mismatch: observed ultra is rejected by requests.
		require.Contains(t, account.GetUpstreamModelMetadataSnapshot().Models["gpt-6.1-sol"].SupportedReasoningLevels, "ultra")
		require.Error(t, openai.ValidateGPT61SolReasoningEffort("gpt-6.1-sol", "ultra"))
		assertGPT61CatalogContract(t, body)
	}
}

func TestGPT61CatalogContractLiveAndCached(t *testing.T) {
	_, oauthCalls := newCodexModelsOAuthCacheServer(t, gpt61ContractCatalog)
	upstream := &codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"ETag": []string{`"provider-catalog"`}}, Body: io.NopCloser(strings.NewReader(gpt61ContractCatalog))}, nil
	}}
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		t.Run(accountType, func(t *testing.T) {
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: accountType, Credentials: map[string]any{
				"base_url": "https://upstream.example", "api_key": "synthetic", "access_token": "synthetic", "chatgpt_account_id": "synthetic",
			}}
			svc := newCodexModelsAPIKeyTestService(upstream)
			for range 2 {
				manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.153.0", "")
				require.NoError(t, err)
				require.NoError(t, svc.CompleteAPIKeyCodexModelsManifestForClient(manifest, account))
				assertGPT61CatalogContract(t, manifest.Body)
				require.Equal(t, "provider low", decodeCodexManifestModels(t, manifest.Body)[0]["supported_reasoning_levels"].([]any)[0].(map[string]any)["description"])
				var raw map[string]any
				require.NoError(t, json.Unmarshal(manifest.upstreamSourceBody, &raw))
				require.Contains(t, string(manifest.upstreamSourceBody), `"effort":"ultra"`)
			}
			if accountType == AccountTypeOAuth {
				require.EqualValues(t, 1, oauthCalls.Load())
			}
		})
	}
}

func TestGPT61CatalogContractAliasesAndEmptyIntersection(t *testing.T) {
	for _, target := range []string{"gpt-6.1-sol", "GPT_6.1_SOL", "openai/gpt-6.1-sol", "gpt-6.1-sol-max", "openai/gpt-6.1-sol-high"} {
		t.Run(target, func(t *testing.T) {
			reasoning := true
			account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"base_url": "https://relay.example", "model_mapping": map[string]any{"public-sol": target}}}
			for _, levels := range [][]string{{"low", "medium", "high", "xhigh", "max", "ultra"}, {"ultra", "none", "minimal", "unknown"}} {
				account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
					target: {ID: target, Reasoning: &reasoning, DefaultReasoningLevel: "ultra", SupportedReasoningLevels: levels},
				}})
				body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-sol"}, []Account{account}, nil, nil, true)
				require.NoError(t, err)
				if levels[0] == "low" {
					assertGPT61CatalogContract(t, body)
				} else {
					model := decodeCodexManifestModels(t, body)[0]
					require.Empty(t, effortsFromManifestModel(t, model))
					require.NotContains(t, model, "default_reasoning_level", "official metadata must not restore an invalid default")
				}
				features := gatewayAccountFeatures(&account, target, CompositeRouteEndpointResponses)
				require.NotContains(t, features.ReasoningEfforts, "ultra")
			}
			manifest := &CodexModelsManifest{Body: []byte(strings.ReplaceAll(gpt61ContractCatalog, "gpt-6.1-sol", " public-sol "))}
			require.NoError(t, (&OpenAIGatewayService{}).CompleteAPIKeyCodexModelsManifestForClient(manifest, &account))
			require.NoError(t, qualifyGroupCodexManifestReasoning(manifest, &Group{Platform: PlatformOpenAI}, []Account{account}, nil, true))
			assertGPT61CatalogContract(t, manifest.Body)
		})
	}
}

func TestGPT61CatalogContractMixedTargetsAndComposite(t *testing.T) {
	reasoning := true
	newAccount := func(id int64, target string, levels []string) Account {
		account := Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"model_mapping": map[string]any{"public-sol": target}}}
		account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
			target: {ID: target, Reasoning: &reasoning, DefaultReasoningLevel: "ultra", SupportedReasoningLevels: levels},
		}})
		return account
	}
	accounts := []Account{newAccount(1, "gpt-6.1-sol", []string{"max", "high", "ultra"}), newAccount(2, "gpt-5.6-sol", []string{"high", "ultra"})}
	group := &Group{Platform: PlatformOpenAI, ModelRoutingEnabled: true, ModelRouting: map[string][]int64{"public-sol": {1, 2}}}
	body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-sol"}, accounts, group, nil, true)
	require.NoError(t, err)
	require.Equal(t, []string{"high"}, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]))
	group.ModelRoutingEnabled = false
	body, err = buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-sol"}, accounts, group, nil, true)
	require.NoError(t, err)
	require.Equal(t, []string{"high"}, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]), "only shared, qualified capabilities survive")
	accounts[1] = newAccount(2, "gpt-5.6-sol", []string{"ultra"})
	body, err = buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-sol"}, accounts, group, nil, true)
	require.NoError(t, err)
	require.Empty(t, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]))
	routes := []CompositeModelRoute{{PublicModel: "public-sol", TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-6.1-sol", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointResponses, Enabled: true}}
	accounts[0].Credentials["model_mapping"].(map[string]any)["gpt-6.1-sol"] = "gpt-6.1-sol"
	body, err = buildCodexModelsManifestForAccounts(PlatformComposite, []string{"public-sol"}, accounts[:1], &Group{Platform: PlatformComposite}, routes, true)
	require.NoError(t, err)
	require.Equal(t, []string{"max", "high"}, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]))
	accounts[0].Credentials["model_mapping"] = map[string]any{"gpt-5.6-sol": "gpt-6.1-sol"}
	accounts[0].Extra = nil
	routes[0].UpstreamModel = "gpt-5.6-sol"
	body, err = buildCodexModelsManifestForAccounts(PlatformComposite, []string{"public-sol"}, accounts[:1], &Group{Platform: PlatformComposite}, routes, true)
	require.NoError(t, err)
	assertGPT61CatalogContract(t, body)
	accounts[0].Credentials["model_mapping"] = map[string]any{"gpt-6.1-sol": "gpt-5.6-sol"}
	accounts[0].SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-5.6-sol": {ID: "gpt-5.6-sol", Reasoning: &reasoning, DefaultReasoningLevel: "ultra", SupportedReasoningLevels: []string{"high", "ultra"}},
	}})
	routes[0].UpstreamModel = "gpt-6.1-sol"
	body, err = buildCodexModelsManifestForAccounts(PlatformComposite, []string{"public-sol"}, accounts[:1], &Group{Platform: PlatformComposite}, routes, true)
	require.NoError(t, err)
	require.Equal(t, []string{"high", "ultra"}, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]))
}

func TestGPT61CatalogContractRawMappingCollisionAndMixedAccounts(t *testing.T) {
	for _, target := range []string{"gpt-6.1-sol", "gpt-5.6-sol"} {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-6.1-sol": "gpt-5.6-sol", "gpt-5.6-sol": "gpt-6.1-sol", "public-sol": target},
		}}
		manifest := &CodexModelsManifest{Body: []byte(strings.ReplaceAll(gpt61ContractCatalog, "gpt-6.1-sol", target))}
		require.NoError(t, (&OpenAIGatewayService{}).CompleteAPIKeyCodexModelsManifestForClient(manifest, account))
		body, err := mergeRoutableCodexManifests([]codexAccountManifest{{account: account, manifest: manifest}}, nil)
		require.NoError(t, err)
		if target == "gpt-6.1-sol" {
			assertGPT61CatalogContract(t, body)
		} else {
			require.Contains(t, effortsFromManifestModel(t, decodeCodexManifestModels(t, body)[0]), "ultra")
		}
	}
	for _, reversed := range []bool{false, true} {
		accounts := []Account{
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": map[string]any{"public-sol": "gpt-5.6-sol"}}},
			{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": map[string]any{"public-sol": "gpt-6.1-sol"}}},
		}
		if reversed {
			accounts[0], accounts[1] = accounts[1], accounts[0]
		}
		sources := []codexAccountManifest{}
		bodies := [][]byte{}
		group := &Group{Platform: PlatformOpenAI}
		for i := range accounts {
			account := &accounts[i]
			manifest := &CodexModelsManifest{Body: []byte(strings.ReplaceAll(gpt61ContractCatalog, "gpt-6.1-sol", account.GetMappedModel("public-sol")))}
			require.NoError(t, (&OpenAIGatewayService{}).CompleteAPIKeyCodexModelsManifestForClient(manifest, account))
			sources = append(sources, codexAccountManifest{account: account, manifest: manifest})
			projected, err := projectAccountModelsBody(manifest.Body, account, group, true)
			require.NoError(t, err)
			bodies = append(bodies, projected)
		}
		dynamic, err := mergeRoutableCodexManifests(sources, nil)
		require.NoError(t, err)
		pinned, err := mergeCodexModelsManifestBodies(bodies)
		require.NoError(t, err)
		for _, body := range [][]byte{dynamic, pinned} {
			manifest := &CodexModelsManifest{Body: body}
			require.NoError(t, qualifyGroupCodexManifestReasoning(manifest, group, accounts, nil, true))
			assertGPT61CatalogContract(t, manifest.Body)
		}
	}
}

func TestGPT61CatalogContractPreservesOtherModels(t *testing.T) {
	for _, modelID := range []string{"gpt-6-sol", "gpt-6-luna", "gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra"} {
		raw := json.RawMessage(strings.ReplaceAll(gpt61ContractCatalog[len(`{"models":[`):len(gpt61ContractCatalog)-2], "gpt-6.1-sol", modelID))
		qualified, err := qualifyCodexModelReasoning(raw, modelID)
		require.NoError(t, err)
		require.Equal(t, raw, qualified)
	}
	for _, target := range []string{"gpt-6.1-sol-ultra", "gpt-6.1-sol-none", "openai/gpt-6.1-sol-minimal"} {
		raw := json.RawMessage(gpt61ContractCatalog[len(`{"models":[`) : len(gpt61ContractCatalog)-2])
		qualified, err := qualifyCodexModelReasoning(raw, target)
		require.NoError(t, err)
		var model map[string]any
		require.NoError(t, json.Unmarshal(qualified, &model))
		require.Empty(t, effortsFromManifestModel(t, model))
		require.NotContains(t, model, "default_reasoning_level")
	}
	qualified, err := qualifyCodexModelReasoning(json.RawMessage(`{"slug":"gpt-6.1-sol","default_reasoning_level":"ultra","future":true}`), "gpt-6.1-sol")
	require.NoError(t, err)
	require.JSONEq(t, `{"slug":"gpt-6.1-sol","future":true}`, string(qualified))
	for _, raw := range []string{
		`{"slug":"gpt-6.1-sol-ultra","default_reasoning_level":"high","supported_reasoning_levels":[{"effort":"high"}]}`,
		`{"slug":"gpt-6.1-sol-ultra","default_reasoning_level":"high"}`,
	} {
		qualified, err := qualifyCodexModelReasoning(json.RawMessage(raw), "gpt-6.1-sol")
		require.NoError(t, err)
		var model map[string]any
		require.NoError(t, json.Unmarshal(qualified, &model))
		require.Empty(t, model["supported_reasoning_levels"])
		require.NotContains(t, model, "default_reasoning_level")
	}
}

func TestGPT61CatalogContractSchedulerMergeQualifiesAddedAliases(t *testing.T) {
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-6.1-sol"}}}
	group := &Group{ID: 991, Platform: PlatformOpenAI}
	svc := &OpenAIGatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{group.ID: {account}}}}
	manifest := &CodexModelsManifest{Body: []byte(`{"models":[]}`)}
	require.NoError(t, svc.MergeGroupConfiguredCodexModels(context.Background(), group, manifest, ""))
	assertGPT61CatalogContract(t, manifest.Body)
	currentETag := manifest.ETag
	manifest = &CodexModelsManifest{Body: []byte(`{"models":[]}`)}
	require.NoError(t, svc.MergeGroupConfiguredCodexModels(context.Background(), group, manifest, currentETag))
	require.True(t, manifest.NotModified)
}

type gpt61FailingFinalRouteRepo struct {
	compositeRouteRepoStub
	calls atomic.Int32
}

func (r *gpt61FailingFinalRouteRepo) ListByGroup(ctx context.Context, id int64, disabled bool) ([]CompositeModelRoute, error) {
	if r.calls.Add(1) > 1 {
		return nil, errors.New("synthetic route store failure")
	}
	return r.compositeRouteRepoStub.ListByGroup(ctx, id, disabled)
}

func TestGPT61CatalogContractDynamicRouteLookupFailsClosed(t *testing.T) {
	newCodexModelsOAuthCacheServer(t, gpt61ContractCatalog)
	account := *newCodexModelsTestAccount()
	group := &Group{ID: 992, Platform: PlatformComposite}
	routeRepo := &gpt61FailingFinalRouteRepo{compositeRouteRepoStub: compositeRouteRepoStub{routes: []CompositeModelRoute{{
		PublicModel: "gpt-6.1-sol", TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-6.1-sol", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointResponses, Enabled: true,
	}}}}
	svc := &OpenAIGatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{group.ID: {account}}}, compositeResolver: NewCompositeRouteResolver(routeRepo)}
	manifest, used, err := svc.BuildGroupDynamicCodexModelsManifest(context.Background(), group, "0.153.0", "")
	require.True(t, used)
	require.ErrorContains(t, err, "synthetic route store failure")
	require.Nil(t, manifest)
}
