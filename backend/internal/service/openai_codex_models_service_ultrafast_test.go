package service

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAstraUltrafastCatalogUsesAccountCapabilities(t *testing.T) {
	for _, tc := range []struct {
		accountType, base, model string
		want                     bool
	}{
		{AccountTypeAPIKey, "https://api.openai.com", "gpt-6-astra", false},
		{AccountTypeAPIKey, "https://api.openai.com", "gpt-6.1-sol", false},
		{AccountTypeAPIKey, "https://proxy.example", "gpt-6-astra", false},
		{AccountTypeOAuth, "https://chatgpt.com", "gpt-6-astra", false},
	} {
		account := &Account{Platform: PlatformOpenAI, Type: tc.accountType, Credentials: map[string]any{"base_url": tc.base, "plan_type": "promax"}}
		caps := accountCodexToolCapabilities(account, tc.model)
		require.Equal(t, tc.want, bytes.Contains(caps["service_tiers"], []byte("ultrafast")))
	}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"gpt-6-astra": {ID: "gpt-6-astra", CodexToolCapabilities: map[string]json.RawMessage{
			"service_tiers": json.RawMessage(`[{"id":"ultrafast","name":"Ultrafast"}]`),
		}},
	}})
	require.Contains(t, string(accountCodexToolCapabilities(account, "gpt-6-astra")["service_tiers"]), "ultrafast")
	require.Empty(t, accountCodexToolCapabilities(account, "unknown")["service_tiers"])
	metadata := intersectUpstreamModelMetadata("public-alias", []UpstreamModelMetadata{
		{CodexToolCapabilities: accountCodexToolCapabilities(account, "gpt-6-astra")},
		{},
	})
	descriptor := newConfiguredCodexModelDescriptor("public-alias")
	applyUpstreamModelMetadataToCodexDescriptor(&descriptor, metadata)
	require.Empty(t, descriptor.ServiceTiers)
	// Explicit native null/empty fields and account-provided tiers remain authoritative.
	for _, raw := range []string{"null", "[]", `[{"id":"ultrafast","name":"Ultrafast"}]`} {
		dst := map[string]json.RawMessage{"service_tiers": json.RawMessage(raw)}
		require.False(t, applyCodexToolCapabilities(dst, map[string]json.RawMessage{"service_tiers": json.RawMessage(`[{"id":"priority"}]`)}, false))
		require.JSONEq(t, raw, string(dst["service_tiers"]))
	}
}
