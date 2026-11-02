package service

import (
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHaiku55PricingAndLongContextBoundary(t *testing.T) {
	data, err := os.ReadFile("../../resources/model-pricing/model_prices_and_context_window.json")
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(data)
	require.NoError(t, err)
	for _, pricing := range []*PricingService{nil, catalog, {pricingData: map[string]*LiteLLMModelPricing{}}} {
		svc := NewBillingService(&config.Config{}, pricing)
		for _, model := range []string{"claude-haiku-5-5", "anthropic/claude-haiku-5.5", "global.anthropic.claude-haiku-5-5"} {
			for _, total := range []int{100_000, 100_001} {
				multiplier := 1.0
				if total > 100_000 {
					multiplier = 5
				}
				cost, err := svc.CalculateCost(model, UsageTokens{InputTokens: total - 2000, OutputTokens: 1000, CacheReadTokens: 1000, CacheCreationTokens: 1000, CacheCreation5mTokens: 400, CacheCreation1hTokens: 600}, 1)
				require.NoError(t, err)
				require.InDelta(t, float64(total-2000)*0.1e-6*multiplier, cost.InputCost, 1e-10)
				require.InDelta(t, 1000*0.5e-6*multiplier, cost.OutputCost, 1e-10)
				require.InDelta(t, (400*0.125e-6+600*0.2e-6)*multiplier, cost.CacheCreationCost, 1e-10)
				require.InDelta(t, 1000*0.01e-6*multiplier, cost.CacheReadCost, 1e-10)
				require.Equal(t, total > 100_000, cost.LongContextBillingApplied)
			}
		}
	}
}

func TestHaiku55RequestPreservesSignedHistoryAndForcedToolChoice(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-5-5","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}],"tool_choice":{"type":"any"},"output_config":{"effort":"medium"}}`)
	require.NoError(t, validateClaude55Request(body, "claude-haiku-5-5"))
	require.JSONEq(t, string(body), string(FilterThinkingBlocks(body, "claude-haiku-5-5")))
	for _, field := range []string{`"thinking":{"type":"enabled","budget_tokens":1000}`, `"thinking":{"type":"disabled"}`, `"temperature":0.5`} {
		require.ErrorContains(t, validateClaude55Request([]byte(`{`+field+`}`), "claude-haiku-5-5"), "claude-haiku-5-5")
	}
}

func TestClaudePublicAliasesUseExistingDatedAccountMappings(t *testing.T) {
	account := &Account{Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001"}}}
	require.True(t, account.IsModelSupported("claude-haiku-4-5"))
	require.Equal(t, "claude-haiku-4-5-20251001", account.GetMappedModel("claude-haiku-4-5"))
	require.Equal(t, []string{"claude-haiku-4-5-20251001"}, FilterModelsByCustomList([]string{"claude-haiku-4-5"}, nil, []string{"claude-haiku-4-5-20251001"}))
	for _, id := range []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001"} {
		allowlist := GroupModelAllowlist{Enabled: true, Models: []string{id}}
		require.True(t, allowlist.Allows("claude-haiku-4-5"))
		require.True(t, allowlist.Allows("claude-haiku-4-5-20251001"))
		require.Equal(t, []string{id}, allowlist.FilterForListing([]string{"claude-haiku-4-5"}))
	}
}
