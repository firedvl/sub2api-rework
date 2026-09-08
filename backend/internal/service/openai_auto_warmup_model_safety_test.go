package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveOpenAIAutoWarmupModelUsesCanonicalVersionedLKG(t *testing.T) {
	account := newOpenAIAutoWarmupModelSafetyAccount()
	version := CodexCanonicalClientVersion()
	account.Extra[OpenAICodexManifestSnapshotExtraKey] = openAICodexManifestSnapshots{
		Identity: openAICodexManifestIdentity(account),
		Versions: map[string]openAICodexManifestSnapshot{
			version: {SyncedAt: time.Now().UTC().Format(time.RFC3339Nano), Body: json.RawMessage(`{"models":[{"slug":"lkg-text-model"}]}`)},
		},
	}
	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.URL.Query().Get("client_version")
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	previousURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	t.Cleanup(func() { chatgptCodexModelsURL = previousURL })

	service := &OpenAIGatewayService{cfg: &config.Config{}}
	model, err := service.resolveOpenAIAutoWarmupModel(context.Background(), account, "synthetic-token")

	require.NoError(t, err)
	require.Equal(t, "lkg-text-model", model)
	require.Equal(t, version, gotVersion)
}

func TestResolveOpenAIAutoWarmupModelDoesNotUseVersionedLKGAfter401(t *testing.T) {
	account := newOpenAIAutoWarmupModelSafetyAccount()
	version := CodexCanonicalClientVersion()
	account.Extra[OpenAICodexManifestSnapshotExtraKey] = openAICodexManifestSnapshots{
		Identity: openAICodexManifestIdentity(account),
		Versions: map[string]openAICodexManifestSnapshot{
			version: {SyncedAt: time.Now().UTC().Format(time.RFC3339Nano), Body: json.RawMessage(`{"models":[{"slug":"must-not-use"}]}`)},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	previousURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	t.Cleanup(func() { chatgptCodexModelsURL = previousURL })

	model, err := (&OpenAIGatewayService{cfg: &config.Config{}}).resolveOpenAIAutoWarmupModel(context.Background(), account, "synthetic-token")

	require.Error(t, err)
	require.Empty(t, model)
}

func TestSelectOpenAIAutoWarmupModelRespectsExplicitUpstreamTargets(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Credentials: map[string]any{
		"model_mapping": map[string]any{
			"alias-disabled": "mapped-disabled",
			"alias-media":    "mapped-media",
			"alias-output":   "mapped-output-media",
			"alias-text":     "mapped-text",
		},
	}}
	model, err := selectOpenAIAutoWarmupModelForAccount([]byte(`{"models":[
		{"slug":"unmapped-smaller","context_window":1,"max_output_tokens":1},
		{"slug":"mapped-disabled","supported_in_api":false},
		{"slug":"mapped-media","input_modalities":["image"]},
		{"slug":"mapped-output-media","output_modalities":["image"]},
		{"slug":"mapped-text","supported_in_api":true,"input_modalities":["text"],"output_modalities":["text"]}
	]}`), account)

	require.NoError(t, err)
	require.Equal(t, "mapped-text", model)
}

func newOpenAIAutoWarmupModelSafetyAccount() *Account {
	return &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "stored-token", "chatgpt_account_id": "org-test"},
		Extra:       make(map[string]any),
	}
}
