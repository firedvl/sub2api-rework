package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayAccountFeaturesUnknownWithoutMetadata(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{}}
	features := gatewayAccountFeatures(account, "dynamic-upstream-slug", "responses")
	for _, key := range gatewayFeatureKeys {
		if key == "image_input" {
			require.Equal(t, gatewayFeatureUnsupported, features.Features[key], key)
			continue
		}
		require.Equal(t, gatewayFeatureUnknown, features.Features[key], key)
	}
}

func TestGatewayAccountFeaturesImageRequiresOpenAIVisionGate(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"openai_capabilities": []any{"chat_completions", "vision_input"},
		},
		Extra: map[string]any{"openai_responses_supported": false},
	}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"qualified": {ID: "qualified", InputModalities: []string{"text", "image"}},
	}})
	features := gatewayAccountFeatures(account, "qualified", "responses")
	require.Equal(t, gatewayFeatureUnsupported, features.Features["image_input"])

	account.Extra["openai_responses_supported"] = true
	features = gatewayAccountFeatures(account, "qualified", "responses")
	require.Equal(t, gatewayFeatureSupported, features.Features["image_input"])
}

func TestGatewayIntersectFeaturesDoesNotUnionRouteEvidence(t *testing.T) {
	first := gatewayUnknownFeatures()
	first.Features["functions"] = gatewayFeatureSupported
	first.ReasoningEfforts = []string{"low", "high"}
	first.ContextWindow = 200
	second := gatewayUnknownFeatures()
	second.Features["web_search"] = gatewayFeatureSupported
	second.ReasoningEfforts = []string{"high", "xhigh"}
	second.ContextWindow = 100

	features := gatewayIntersectFeatures([]GatewayFeatureSet{first, second})
	require.Equal(t, gatewayFeatureUnknown, features.Features["functions"])
	require.Equal(t, gatewayFeatureUnknown, features.Features["web_search"])
	require.Equal(t, []string{"high"}, features.ReasoningEfforts)
	require.EqualValues(t, 100, features.ContextWindow)

	unknown := gatewayUnknownFeatures()
	features = gatewayIntersectFeatures([]GatewayFeatureSet{first, unknown})
	require.Nil(t, features.ReasoningEfforts)
	require.Zero(t, features.ContextWindow)
}

func TestGatewayAccountFeaturesUsesFreshSameIdentityManifestOnly(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	body := `{"models":[{"slug":"dynamic-model","supported_in_api":true,"input_modalities":["text","audio"],"supported_reasoning_levels":["low","high"],"context_window":321}]}`
	setCodexManifestSnapshotForTest(account, "test", body, time.Now())
	features := gatewayAccountFeatures(account, "dynamic-model", "responses")
	require.Equal(t, gatewayFeatureSupported, features.Features["protocol"])
	require.Equal(t, gatewayFeatureUnknown, features.Features["audio_input"])
	require.Equal(t, []string{"low", "high"}, features.ReasoningEfforts)
	require.EqualValues(t, 321, features.ContextWindow)

	account.Extra[OpenAICodexManifestSnapshotExtraKey] = openAICodexManifestSnapshots{Identity: "other", Versions: map[string]openAICodexManifestSnapshot{"test": {SyncedAt: time.Now().UTC().Format(time.RFC3339Nano), Body: []byte(body)}}}
	features = gatewayAccountFeatures(account, "dynamic-model", "responses")
	require.Equal(t, gatewayFeatureUnknown, features.Features["protocol"])
}

func TestGatewayFeatureConstraintMatchesAntigravityMixedTools(t *testing.T) {
	constraint := GatewayCapabilityConstraint{AllOf: []string{"functions", "web_search"}}
	require.True(t, gatewayFeatureConstraintMatches(constraint, map[string]bool{"functions": true, "web_search": true}))
	require.False(t, gatewayFeatureConstraintMatches(constraint, map[string]bool{"functions": true}))
	require.True(t, antigravityV1InternalUsesMixedTools([]byte(`{"tools":[{"functionDeclarations":[{"name":"f"}]},{"googleSearch":{}}]}`)))

	features := gatewayAccountFeatures(&Account{Platform: PlatformAntigravity}, "observed-model", "responses")
	require.Equal(t, gatewayFeatureSupported, features.Features["protocol"])
	require.Equal(t, gatewayFeatureSupported, features.Features["functions"])
	require.Equal(t, gatewayFeatureSupported, features.Features["web_search"])
	require.Equal(t, "CAPABILITY_COMBINATION_UNSUPPORTED", features.Constraints[0].Reason)
}

func TestGatewayProtocolFeatureMessagesUsesOpenAIChatGate(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.Equal(t, gatewayFeatureSupported, gatewayProtocolFeature(account, "messages"))
	account = &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"openai_capabilities": []any{"responses"}}}
	require.Equal(t, gatewayFeatureUnsupported, gatewayProtocolFeature(account, "messages"))
}

func TestGatewayAccountFeaturesCompleteManifestMergeRemainsKnown(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"future": {ID: "future", InputModalities: []string{"text"}, ContextWindow: 321},
	}})
	setCodexManifestSnapshotForTest(account, "test", `{"models":[{"slug":"future","input_modalities":["text"],"context_window":321}]}`, time.Now())
	features := gatewayAccountFeatures(account, "future", "responses")
	require.Equal(t, "supported", features.Features["text_input"])
	require.NotNil(t, gatewayIntersectFeatures([]GatewayFeatureSet{features}).Constraints)
}

func TestGatewayAccountMetadataPreservesToolFallbackAndManifestNegativeEvidence(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"future": {ID: "future", CodexToolCapabilities: map[string]json.RawMessage{
			"supported_in_api":     []byte("true"),
			"supports_search_tool": []byte("false"),
		}},
	}})
	setCodexManifestSnapshotForTest(account, "test", `{"models":[{"slug":"future","supported_in_api":false}]}`, time.Now())
	metadata := gatewayAccountMetadata(account)
	require.Equal(t, json.RawMessage("false"), metadata["future"].CodexToolCapabilities["supported_in_api"])
	require.Equal(t, json.RawMessage("false"), metadata["future"].CodexToolCapabilities["supports_search_tool"])
	require.Equal(t, gatewayFeatureUnsupported, gatewayAccountFeaturesWithMetadata(account, "future", "responses", metadata).Features["protocol"])

	setCodexManifestSnapshotForTest(account, "test", `{"models":[{"slug":"future"}]}`, time.Now())
	features := gatewayAccountFeaturesWithMetadata(account, "future", "responses", gatewayAccountMetadata(account))
	require.Equal(t, gatewayFeatureUnsupported, features.Features["tool_discovery"])
}

func TestGatewayAccountMetadataUsesStableLatestManifestTieBreak(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	syncedAt := time.Now().UTC().Format(time.RFC3339Nano)
	account.Extra[OpenAICodexManifestSnapshotExtraKey] = openAICodexManifestSnapshots{
		Identity: openAICodexManifestIdentity(account),
		Versions: map[string]openAICodexManifestSnapshot{
			"a": {SyncedAt: syncedAt, Body: []byte(`{"models":[{"slug":"tie","input_modalities":["text"]}]}`)},
			"b": {SyncedAt: syncedAt, Body: []byte(`{"models":[{"slug":"tie","input_modalities":["image"]}]}`)},
		},
	}
	for range 10 {
		features := gatewayAccountFeatures(account, "tie", "responses")
		require.Equal(t, gatewayFeatureUnsupported, features.Features["image_input"])
		require.Equal(t, gatewayFeatureSupported, features.Features["text_input"])
	}
}
