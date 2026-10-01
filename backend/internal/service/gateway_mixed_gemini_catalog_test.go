package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type mixedGeminiCatalogRepo struct {
	AccountRepository
	accounts       []Account
	groupID        *int64
	includeGrouped bool
	platforms      []string
}

func (repo *mixedGeminiCatalogRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]Account, error) {
	repo.groupID, repo.platforms, repo.includeGrouped = groupID, platforms, includeGrouped
	filtered := make([]Account, 0, len(repo.accounts))
	for _, account := range repo.accounts {
		for _, platform := range platforms {
			if account.Platform == platform {
				filtered = append(filtered, account)
				break
			}
		}
	}
	return filtered, nil
}

func TestCatalogModelsMixedGeminiUsesDurableCandidatesAndOptIn(t *testing.T) {
	for _, mode := range []string{config.RunModeStandard, config.RunModeSimple} {
		t.Run(mode, func(t *testing.T) {
			blockedUntil := time.Now().Add(time.Hour)
			repo := &mixedGeminiCatalogRepo{accounts: []Account{
				{ID: 1, Platform: PlatformAntigravity, RateLimitResetAt: &blockedUntil,
					Extra: map[string]any{"mixed_scheduling": true},
					Credentials: map[string]any{"model_mapping": map[string]any{
						"gemini-custom": "gemini-provider", "claude-custom": "claude-provider",
						"gemini-*": "gemini-provider", "models/gemini-prefixed": "gemini-provider",
						"gemini-pro-agent": "gemini-pro-agent",
					}}},
				{ID: 2, Platform: PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-no-opt-in": "gemini-provider"}}},
				{ID: 3, Platform: PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": false}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-disabled": "gemini-provider"}}},
				{ID: 4, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-wrong-provider": "claude-provider"}}},
			}}
			groupID := int64(42)
			gateway := &GatewayService{accountRepo: repo, cfg: &config.Config{RunMode: mode}}
			models, backed := gateway.GetCatalogModels(context.Background(), &groupID, PlatformGemini)
			require.True(t, backed)
			require.Contains(t, models, "gemini-custom")
			require.Contains(t, models, "gemini-prefixed")
			for _, forbidden := range []string{"claude-custom", "gemini-no-opt-in", "gemini-disabled", "gemini-wrong-provider", "gemini-*", "gemini-pro-agent"} {
				require.NotContains(t, models, forbidden)
			}
			require.Equal(t, []string{PlatformGemini, PlatformAntigravity}, repo.platforms)
			if mode == config.RunModeSimple {
				require.Nil(t, repo.groupID)
				require.True(t, repo.includeGrouped)
			} else {
				require.Equal(t, &groupID, repo.groupID)
				require.False(t, repo.includeGrouped)
			}
			nativeModels, err := gateway.AntigravityGeminiCatalogModelIDs(context.Background(), &groupID, true)
			require.NoError(t, err)
			require.Equal(t, models, nativeModels)
			forcedModels, err := gateway.AntigravityGeminiCatalogModelIDs(context.Background(), &groupID, false)
			require.NoError(t, err)
			require.Contains(t, forcedModels, "gemini-no-opt-in")
			require.Contains(t, forcedModels, "gemini-disabled")
			require.NotContains(t, forcedModels, "claude-custom")
		})
	}
}
