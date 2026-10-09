package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestGPT61ProviderDiscoveryAndRestrictionMatrix(t *testing.T) {
	const groupID int64 = 918
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"models":[{"slug":"gpt-6.1-sol","supported_in_api":true,"input_modalities":["text","image"],"future_metadata":{"preserved":true}},{"slug":"gpt-6-sol","supported_in_api":true}]}`)
	}))
	defer server.Close()
	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	t.Cleanup(func() { chatgptCodexModelsURL = original })
	for _, tc := range []struct {
		name      string
		mapping   map[string]any
		allowlist GroupModelAllowlist
		cooldown  bool
		want      bool
	}{
		{name: "provider-backed", want: true},
		{name: "account-restricted", mapping: map[string]any{"gpt-6-sol": "gpt-6-sol"}},
		{name: "group-denied", allowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-sol"}}},
		{name: "group-authorized", allowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6.1-sol"}}, want: true},
		{name: "cooldown-retains-catalog", cooldown: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "synthetic-token", "chatgpt_account_id": "synthetic-account"}}
			if tc.mapping != nil {
				account.Credentials["model_mapping"] = tc.mapping
			}
			if tc.cooldown {
				until := time.Now().Add(time.Hour)
				account.RateLimitResetAt = &until
			}
			repo := codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{groupID: {account}}}
			group := &Group{ID: groupID, Platform: PlatformOpenAI, ModelAllowlist: tc.allowlist}
			manifest, used, err := (&OpenAIGatewayService{accountRepo: repo}).BuildGroupDynamicCodexModelsManifest(context.Background(), group, "0.153.0", "")
			require.NoError(t, err)
			require.True(t, used)
			slugs := codexManifestModelSlugs(t, manifest.Body)
			require.Equal(t, tc.want, containsGPT61(slugs), tc.name)
			if tc.want {
				require.Contains(t, string(manifest.Body), `"future_metadata":{"preserved":true}`)
			}
		})
	}
	require.NotContains(t, openai.DefaultModelIDs(), "gpt-6.1-sol", "compatibility metadata must not grant publication")
}

func containsGPT61(models []string) bool {
	for _, model := range models {
		if model == "gpt-6.1-sol" {
			return true
		}
	}
	return false
}

func TestGPT61CachedDiscoveryStillEnforcesGroup(t *testing.T) {
	const groupID int64 = 919
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "synthetic-token", "chatgpt_account_id": "synthetic-account"}}
	setCodexManifestSnapshotForTest(account, "0.153.0", `{"models":[{"slug":"gpt-6.1-sol"},{"slug":"gpt-6-sol"}]}`, time.Now())
	repo := &durableCodexModelsAccountRepo{accounts: map[int64]*Account{1: account}, members: map[int64][]int64{groupID: {1}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic temporary failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	t.Cleanup(func() { chatgptCodexModelsURL = original })
	group := &Group{ID: groupID, Platform: PlatformOpenAI, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-sol"}}}
	manifest, used, err := (&OpenAIGatewayService{accountRepo: repo}).BuildGroupDynamicCodexModelsManifest(context.Background(), group, "0.153.0", "")
	require.NoError(t, err)
	require.True(t, used)
	require.NotContains(t, strings.Join(codexManifestModelSlugs(t, manifest.Body), ","), "gpt-6.1-sol")
}
