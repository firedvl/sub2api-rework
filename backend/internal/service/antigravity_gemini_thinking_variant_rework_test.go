//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiThinkingVariantUsesDiscoveredOrExplicitRoutes(t *testing.T) {
	svc := &AntigravityGatewayService{}
	account := &Account{Platform: PlatformAntigravity, Credentials: map[string]any{}}
	account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{
		Source: "account", Models: []string{"gemini-3.6-flash-high"},
	})
	require.Equal(t, "gemini-3.6-flash-high", svc.getMappedModelForThinkingLevel(account, "gemini-3.6-flash", "low"))
	require.Empty(t, svc.getMappedModel(account, "gemini-3-flash"), "a static self-mapping cannot route an ID absent from discovery")
	require.Equal(t, "gemini-3.6-flash", mapAntigravityModel(account, "gemini-3.6-flash"), "scheduler mapping remains unchanged")

	account.Credentials["model_mapping"] = map[string]any{"gemini-3.6-flash": "custom-upstream"}
	require.Equal(t, "custom-upstream", svc.getMappedModel(account, "gemini-3.6-flash"))

	account.Credentials["model_mapping"] = map[string]any{"gemini-3.6-flash-low": "custom-low"}
	require.Equal(t, "custom-low", svc.getMappedModelForThinkingLevel(account, "gemini-3.6-flash", "low"))
}
