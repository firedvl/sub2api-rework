package service

import (
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAstraUltrafastPricingUsesSixTimesStandard(t *testing.T) {
	data, err := os.ReadFile("../../resources/model-pricing/model_prices_and_context_window.json")
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(data)
	require.NoError(t, err)
	for _, svc := range []*BillingService{NewBillingService(&config.Config{}, nil), NewBillingService(&config.Config{}, &PricingService{}), NewBillingService(&config.Config{}, catalog)} {
		for _, model := range []string{"gpt-6-astra", "gpt-6", "openai/gpt-6-astra"} {
			for _, n := range []int{271999, 272000, 272001} {
				tokens := UsageTokens{InputTokens: n - 3000, CacheReadTokens: 2000, CacheCreationTokens: 1000, OutputTokens: 500}
				cost, err := svc.CalculateCostWithServiceTier(model, tokens, 1, "ultrafast")
				require.NoError(t, err)
				im, om := 1.0, 1.0
				if n > 272000 {
					im, om = 2, 1.5
				}
				require.InDelta(t, float64(n-3000)*60e-6*im, cost.InputCost, 1e-10)
				require.InDelta(t, 2000*6e-6*im, cost.CacheReadCost, 1e-10)
				require.InDelta(t, 1000*75e-6*im, cost.CacheCreationCost, 1e-10)
				require.InDelta(t, 500*300e-6*om, cost.OutputCost, 1e-10)
			}
		}
	}
	svc := NewBillingService(&config.Config{}, nil)
	for _, custom := range []float64{0, 1e-6} {
		fast := 3.0
		p, err := svc.GetModelPricingWithChannel("gpt-6-astra", &ChannelModelPricing{InputPrice: &custom, OutputPrice: &custom, CacheWritePrice: &custom, CacheReadPrice: &custom, FastMultiplier: &fast})
		require.NoError(t, err)
		require.Equal(t, 6.0, configuredServiceTierMultiplier("ultrafast", p))
		require.Equal(t, 3.0, configuredServiceTierMultiplier("priority", p))
		cost := svc.computeTokenBreakdown(p, UsageTokens{InputTokens: 1000, OutputTokens: 1000, CacheReadTokens: 1000, CacheCreationTokens: 1000}, 1, "ultrafast", false)
		require.InDelta(t, custom*24000, cost.TotalCost, 1e-10)
	}
	p, err := svc.GetModelPricing("gpt-6-sol")
	require.NoError(t, err)
	require.Equal(t, 2.0, configuredServiceTierMultiplier("ultrafast", p))
}

func TestGPT61UltrafastSelectedPricingAppliedOnce(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)
	for _, model := range []string{"gpt-6.1-sol", "openai/gpt-6.1-sol", "gpt-6.1-sol-2026-09-23"} {
		for _, price := range []float64{0, 1e-6} {
			fast := 3.0
			base := &ModelPricing{InputPricePerToken: price, OutputPricePerToken: price, CacheReadPricePerToken: price, CacheCreationPricePerToken: price,
				CacheCreationPriceExplicit: true, FastMultiplier: &fast,
				LongContextInputThreshold: 272000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5}
			pricing := svc.applyModelSpecificPricingPolicyEx(model, base, false, time.Time{})
			require.Equal(t, 6.0, pricing.UltrafastMultiplier)
			require.Zero(t, base.UltrafastMultiplier)
			for _, input := range []int{271999, 272000, 272001} {
				tokens := UsageTokens{InputTokens: input - 3000, CacheReadTokens: 2000, CacheCreationTokens: 1000, OutputTokens: 500}
				standard := svc.computeTokenBreakdown(pricing, tokens, 1.7, "default", true)
				ultra := svc.computeTokenBreakdown(pricing, tokens, 1.7, "ultrafast", true)
				require.InDelta(t, standard.TotalCost*6, ultra.TotalCost, 1e-10)
				require.InDelta(t, standard.InputCost*6, ultra.InputCost, 1e-10)
				require.InDelta(t, standard.OutputCost*6, ultra.OutputCost, 1e-10)
				require.InDelta(t, standard.CacheReadCost*6, ultra.CacheReadCost, 1e-10)
				require.InDelta(t, standard.CacheCreationCost*6, ultra.CacheCreationCost, 1e-10)
			}
			require.Equal(t, 3.0, configuredServiceTierMultiplier("priority", pricing))
		}
	}
	pricing := svc.applyModelSpecificPricingPolicyEx("gpt-6.1-other", &ModelPricing{}, false, time.Time{})
	require.Zero(t, pricing.UltrafastMultiplier)
}
