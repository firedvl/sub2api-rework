package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImageCacheUsageBounds(test *testing.T) {
	for _, scenario := range []struct {
		name, details       string
		cached, imageCached int
	}{
		{"explicit total", `"cached_tokens":400,"cached_tokens_details":{"image_tokens":300,"text_tokens":100}`, 400, 300},
		{"details only", `"cached_tokens_details":{"image_tokens":3e2,"text_tokens":1e2}`, 400, 300},
		{"no inferred split", `"cached_tokens":400`, 400, 0},
		{"bounded by image input", `"cached_tokens":700,"cached_tokens_details":{"image_tokens":900}`, 700, 600},
		{"bounded by cache total", `"cached_tokens":200,"cached_tokens_details":{"image_tokens":900}`, 200, 200},
		{"explicit zero", `"cached_tokens":0,"cached_tokens_details":{"image_tokens":300}`, 0, 0},
		{"invalid image count", `"cached_tokens":400,"cached_tokens_details":{"image_tokens":1e999999}`, 400, 0},
		{"negative image count", `"cached_tokens":400,"cached_tokens_details":{"image_tokens":-1}`, 400, 0},
		{"bounded details sum", `"cached_tokens_details":{"image_tokens":900,"text_tokens":900}`, 1000, 600},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			body := []byte(`{"usage":{"input_tokens":1000,"output_tokens":400,"input_tokens_details":{"image_tokens":600,` + scenario.details + `},"output_tokens_details":{"image_tokens":400}}}`)
			usage, ok := extractOpenAIUsageFromJSONBytes(body)
			require.True(test, ok)
			require.Equal(test, scenario.cached, usage.CacheReadInputTokens)
			require.Equal(test, scenario.imageCached, usage.ImageCacheReadTokens)
			var streamed OpenAIUsage
			(&OpenAIGatewayService{}).parseSSEUsageBytes(body, &streamed)
			require.Equal(test, usage, streamed)
		})
	}
}

func TestImageCachePricingMultipliers(test *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	base := &ModelPricing{InputPricePerToken: 5e-6, ImageInputPricePerToken: 8e-6,
		CacheReadPricePerToken: 1.25e-6, ImageCacheReadPricePerToken: 2e-6,
		OutputPricePerToken: 10e-6, ImageOutputPricePerToken: 30e-6,
		ReasoningEffortMultipliers: map[string]float64{"high": 3}}
	tokens := UsageTokens{InputTokens: 600, ImageInputTokens: 300, CacheReadTokens: 400,
		ImageCacheReadTokens: 300, OutputTokens: 400, ImageOutputTokens: 400}
	for _, scenario := range []struct {
		name, tier string
		pricing    *ModelPricing
		multiplier float64
		total      float64
	}{
		{"standard", "", base, 1, 0.016625},
		{"priority generic", "priority", base, 2, 0.03325},
		{"priority catalog", "priority", pricingWithPriorityMultiplier(base, 2), 2, 0.01885},
		{"flex", "flex", base, 0.5, 0.0083125},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			resolved := &ResolvedPricing{Mode: BillingModeToken, BasePricing: scenario.pricing}
			cost, err := billing.CalculateCostUnified(CostInput{
				Ctx: context.Background(), Model: "gpt-image-2", Tokens: tokens,
				ServiceTier: scenario.tier, ReasoningEffort: "high", RateMultiplier: 4,
				Resolver: &ModelPricingResolver{}, Resolved: resolved,
			})
			require.NoError(test, err)
			require.InDelta(test, 0.000725*scenario.multiplier*3, cost.CacheReadCost, 1e-12)
			require.InDelta(test, scenario.total*3, cost.TotalCost, 1e-12)
			require.InDelta(test, scenario.total*3*4, cost.ActualCost, 1e-12)
		})
	}
	custom := *base
	fast := 5.0
	custom.FastMultiplier = &fast
	cost := billing.computeTokenBreakdown(&custom, tokens, 1, "priority", false)
	require.InDelta(test, 0.000725*5, cost.CacheReadCost, 1e-12)
	custom.LongContextInputThreshold = 500
	custom.LongContextInputMultiplier = 2
	custom.LongContextOutputMultiplier = 1
	cost = billing.computeTokenBreakdown(&custom, tokens, 1, "", true)
	require.InDelta(test, 0.000725*2, cost.CacheReadCost, 1e-12)
	custom.ImageCacheReadPricePerToken = 0
	custom.ImageCacheReadPriceExplicit = true
	cost = billing.computeTokenBreakdown(&custom, tokens, 1, "", false)
	require.InDelta(test, 100*1.25e-6, cost.CacheReadCost, 1e-12)
	zero := 0.0
	pricing := NewBillingService(&config.Config{}, nil)
	pricing.fallbackPrices["gpt-5.5"] = base
	channel, err := pricing.GetModelPricingWithChannel("gpt-5.5", &ChannelModelPricing{CacheReadPrice: &zero})
	require.NoError(test, err)
	cost = pricing.computeTokenBreakdown(channel, tokens, 1, "priority", false)
	require.Zero(test, cost.CacheReadCost)
	require.Equal(test, 2e-6, base.ImageCacheReadPricePerToken)
}

func TestImageCacheCatalogPricing(test *testing.T) {
	invalid, err := (&PricingService{}).parsePricingData([]byte(`{"invalid-image":{"input_cost_per_token":5e-6,"cache_read_input_image_token_cost":-1}}`))
	require.ErrorContains(test, err, "no valid pricing entries")
	require.NotContains(test, invalid, "invalid-image")
	for _, price := range []string{"2e-6", "0"} {
		catalog := newStubPricingServiceFromJSON(test, `{"custom-image":{"input_cost_per_token":5e-6,"cache_read_input_token_cost":1.25e-6,"cache_read_input_image_token_cost":`+price+`}}`)
		pricing, err := NewBillingService(&config.Config{}, catalog).GetModelPricing("custom-image")
		require.NoError(test, err)
		require.True(test, pricing.ImageCacheReadPriceExplicit)
		if price == "0" {
			require.Zero(test, pricing.ImageCacheReadPricePerToken)
		} else {
			require.Equal(test, 2e-6, pricing.ImageCacheReadPricePerToken)
		}
	}
	catalog := &PricingService{pricingData: map[string]*LiteLLMModelPricing{}}
	for _, model := range []string{"gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare-2026-09-08", "gpt-image-2.5-sunburst-2026-09-08"} {
		pricing := catalog.matchOpenAIModel(model)
		require.NotNil(test, pricing)
		require.Equal(test, 2e-6, pricing.CacheReadInputImageTokenCost)
		require.Equal(test, 8e-6, pricing.InputCostPerImageToken)
	}
}

func TestRecordUsageImageCacheDoesNotMutateCaller(test *testing.T) {
	repository := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(repository, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService.fallbackPrices["gpt-5.5"] = &ModelPricing{
		InputPricePerToken: 5e-6, ImageInputPricePerToken: 8e-6,
		CacheReadPricePerToken: 1.25e-6, ImageCacheReadPricePerToken: 2e-6,
	}
	result := &OpenAIForwardResult{RequestID: "image-cache-test", Model: "gpt-5.5", Duration: time.Second,
		Usage: OpenAIUsage{InputTokens: 1000, ImageInputTokens: 600, CacheReadInputTokens: 400, ImageCacheReadTokens: 300}}
	require.NoError(test, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3},
	}))
	require.Nil(test, result.ImageSizeBreakdown)
	require.Equal(test, 300, repository.lastLog.ImageSizeBreakdown["image_cache_read_tokens"])
	require.InDelta(test, 0.004625, repository.lastLog.TotalCost, 1e-12)
	require.Equal(test, 600, repository.lastLog.ImageInputTokens)
}
