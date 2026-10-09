package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestSonnet55PricingAliasesAndCache(t *testing.T) {
	data, err := os.ReadFile("../../resources/model-pricing/model_prices_and_context_window.json")
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(data)
	require.NoError(t, err)
	for _, svc := range []*BillingService{NewBillingService(&config.Config{}, nil), NewBillingService(&config.Config{}, catalog)} {
		for _, model := range []string{"claude-sonnet-5-5", "anthropic/claude-sonnet-5.5", "us.anthropic.claude-sonnet-5-5"} {
			cost, err := svc.CalculateCost(model, UsageTokens{InputTokens: 100000, OutputTokens: 500, CacheReadTokens: 1000, CacheCreationTokens: 1000, CacheCreation5mTokens: 400, CacheCreation1hTokens: 600}, 1)
			require.NoError(t, err)
			require.InDelta(t, 0.2, cost.InputCost, 1e-10)
			require.InDelta(t, 400*2.5e-6+600*4e-6, cost.CacheCreationCost, 1e-10)
			require.InDelta(t, 1000*0.2e-6, cost.CacheReadCost, 1e-10)
			require.InDelta(t, 500*10e-6, cost.OutputCost, 1e-10)
			require.False(t, cost.LongContextBillingApplied)
		}
		for _, model := range []string{"claude-opus-5-5", "anthropic/claude-opus-5.5"} {
			cost, err := svc.CalculateCost(model, UsageTokens{InputTokens: 1000, OutputTokens: 1000}, 1)
			require.NoError(t, err)
			require.InDelta(t, 0.024, cost.TotalCost, 1e-10)
		}
	}
}
