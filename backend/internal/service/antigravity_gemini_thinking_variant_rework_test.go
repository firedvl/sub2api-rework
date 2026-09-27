//go:build unit

package service

import (
	"context"
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
	require.True(t, (&GatewayService{}).isModelSupportedByAccountWithContext(context.Background(), account, "gemini-3.6-flash"))

	account.Credentials["model_mapping"] = map[string]any{"gemini-3.6-flash": "custom-upstream"}
	require.Equal(t, "custom-upstream", svc.getMappedModel(account, "gemini-3.6-flash"))

	account.Credentials["model_mapping"] = map[string]any{"gemini-3.6-flash-low": "custom-low"}
	require.Equal(t, "custom-low", svc.getMappedModelForThinkingLevel(account, "gemini-3.6-flash", "low"))
}

func TestGeminiThinkingVariantSchedulerSkipsKnownMissingAccount(t *testing.T) {
	missing := &Account{ID: 1, Platform: PlatformAntigravity, Credentials: map[string]any{}}
	missing.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "account", Models: []string{"claude-sonnet-4-6"}})
	available := &Account{ID: 2, Platform: PlatformAntigravity, Credentials: map[string]any{}}
	available.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "account", Models: []string{"gemini-3.6-flash-high"}})
	gateway := &GatewayService{}
	require.False(t, gateway.isModelSupportedByAccountWithContext(context.Background(), missing, "gemini-3.6-flash"))
	require.True(t, gateway.isModelSupportedByAccountWithContext(context.Background(), available, "gemini-3.6-flash"))
	require.False(t, (&GeminiMessagesCompatService{}).isModelSupportedByAccount(missing, "gemini-3.6-flash"))
	require.True(t, (&GeminiMessagesCompatService{}).isModelSupportedByAccount(available, "gemini-3.6-flash"))
}
