package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginDirRepoStub embeds AccountRepository so it satisfies the full interface;
// only ListByPlatform is implemented (the sole method the directory listing uses).
// Any other call panics, which keeps the test honest about the surface it touches.
type pluginDirRepoStub struct {
	AccountRepository
	byPlatform map[string][]Account
}

func (r *pluginDirRepoStub) ListByPlatform(_ context.Context, platform string) ([]Account, error) {
	return r.byPlatform[platform], nil
}

func TestListPluginAccounts_ScopeAndSchedulable(t *testing.T) {
	future := time.Now().Add(time.Hour)
	parentID := int64(1)
	openai := []Account{
		// Schedulable active OAuth; credentials and Extra must not reach metadata.
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
			Name:        "primary",
			Credentials: map[string]any{"access_token": "SECRET-TOKEN", "refresh_token": "SECRET-REFRESH"},
			Extra:       map[string]any{"session_ciphertext": "SECRET-SESSION"}},
		// active but temp-unschedulable (paused) — status stays active, must still be
		// returned, but not schedulable
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
			TempUnschedulableUntil: &future, TempUnschedulableReason: "429 from upstream"},
		// active but rate-limited (429) — status stays active, returned, not schedulable
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
			RateLimitResetAt: &future},
		// shadow oauth — excluded entirely
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
			ParentAccountID: &parentID},
		// apikey type — out of (openai, oauth) scope, excluded
		{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
	}
	svc := &OpenAIGatewayService{accountRepo: &pluginDirRepoStub{byPlatform: map[string][]Account{PlatformOpenAI: openai}}}
	scope := newPluginAccountScope(pluginAccountScopeEntry{Platform: PlatformOpenAI, AccountType: AccountTypeOAuth})

	infos, err := svc.ListPluginAccounts(context.Background(), scope, "", "")
	require.NoError(t, err)

	got := map[int64]PluginAccountInfo{}
	for _, info := range infos {
		got[info.ID] = info
	}
	// 1,2,3 in scope; 4 shadow and 5 apikey excluded.
	require.Len(t, infos, 3)
	assert.Contains(t, got, int64(1))
	assert.Contains(t, got, int64(2))
	assert.Contains(t, got, int64(3))
	assert.NotContains(t, got, int64(4), "shadow account must be excluded")
	assert.NotContains(t, got, int64(5), "out-of-scope account type must be excluded")

	// Host-authoritative schedulable decision.
	assert.True(t, got[1].Schedulable, "active oauth is schedulable")
	assert.False(t, got[2].Schedulable, "temp-unschedulable account is not schedulable")
	assert.False(t, got[3].Schedulable, "rate-limited account is not schedulable")

	// Metadata carries operational state without raw provider or host secrets.
	meta := string(got[1].MetadataJSON)
	assert.NotContains(t, meta, "SECRET-TOKEN", "credentials must never appear in metadata")
	assert.NotContains(t, meta, "SECRET-REFRESH", "credentials must never appear in metadata")
	assert.NotContains(t, meta, "SECRET-SESSION", "managed Extra must never appear in metadata")
	assert.Equal(t, "primary", got[1].Name)
	assert.NotContains(t, string(got[2].MetadataJSON), "429 from upstream", "free-form pause reasons stay private")
	assert.Contains(t, string(got[2].MetadataJSON), "temp_unschedulable_until")
}

func TestAccountReadableMetadataAllowlist(t *testing.T) {
	until := time.Now().Add(time.Hour).UTC()
	acct := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Concurrency: 4, Priority: 2, RateLimitResetAt: &until,
		Credentials:  map[string]any{"refresh_token": "LEAK-REFRESH"},
		Extra:        map[string]any{"session_ciphertext": "LEAK-SESSION"},
		Proxy:        &Proxy{Host: "host", Port: 1, Password: "LEAK-PROXY"},
		ErrorMessage: "LEAK-ERROR", TempUnschedulableReason: "LEAK-REASON",
		GroupIDs: []int64{987654321},
	}
	group := &Group{ID: 7}
	acct.Groups = []*Group{group}
	acct.AccountGroups = []AccountGroup{{GroupID: 7, Group: group, Account: acct}}

	snap, err := accountReadableMetadataJSON(acct)
	require.NoError(t, err)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(snap, &metadata))
	require.Equal(t, float64(4), metadata["concurrency"])
	require.Equal(t, float64(2), metadata["priority"])
	require.Contains(t, metadata, "rate_limit_reset_at")
	for _, field := range []string{"Credentials", "Extra", "Proxy", "error_message", "temp_unschedulable_reason", "group_ids", "Groups", "AccountGroups"} {
		require.NotContains(t, metadata, field)
	}
	for _, secret := range []string{"LEAK-REFRESH", "LEAK-SESSION", "LEAK-PROXY", "LEAK-ERROR", "LEAK-REASON", "987654321"} {
		require.NotContains(t, string(snap), secret)
	}
}

func TestListPluginAccounts_ExcludesNonActiveDefenseInDepth(t *testing.T) {
	// Even if a repo were to return a non-active account, the directory must not
	// expose it (defense-in-depth beyond ListByPlatform's DB filter).
	svc := &OpenAIGatewayService{accountRepo: &pluginDirRepoStub{byPlatform: map[string][]Account{
		PlatformOpenAI: {
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
			{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusDisabled},
			{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError},
		},
	}}}
	scope := newPluginAccountScope(pluginAccountScopeEntry{Platform: PlatformOpenAI, AccountType: AccountTypeOAuth})
	infos, err := svc.ListPluginAccounts(context.Background(), scope, "", "")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, int64(1), infos[0].ID)
}

func TestListPluginAccounts_EmptyScopeReturnsNothing(t *testing.T) {
	svc := &OpenAIGatewayService{accountRepo: &pluginDirRepoStub{byPlatform: map[string][]Account{
		PlatformOpenAI: {{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}},
	}}}
	infos, err := svc.ListPluginAccounts(context.Background(), PluginAccountScope{}, "", "")
	require.NoError(t, err)
	assert.Empty(t, infos, "an empty scope must never enumerate accounts")
}
