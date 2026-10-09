package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerProjectionRetainsIndependentResetWindows(t *testing.T) {
	for _, windows := range []struct{ five, weekly bool }{{true, false}, {false, true}, {true, true}} {
		account := service.Account{ID: 40, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Extra: map[string]any{
			service.OpenAIAutoResetCreditEnabledExtraKey:     true,
			service.OpenAIAutoResetCredit5hEnabledExtraKey:   windows.five,
			service.OpenAIAutoResetCredit7dEnabledExtraKey:   windows.weekly,
			service.OpenAIAutoResetCredit5hThresholdExtraKey: 0.0,
			service.OpenAIAutoResetCredit7dThresholdExtraKey: 0.98,
			service.OpenAIAutoResetCreditStateExtraKey:       map[string]any{"status": service.OpenAIAutoResetStatusAvailable, "available_count": 3},
		}}
		_, payload, err := marshalSchedulerCacheAccount(account)
		require.NoError(t, err)
		var cached service.Account
		require.NoError(t, json.Unmarshal(payload, &cached))
		config := service.ResolveOpenAIAutoResetCreditConfig(&cached)
		require.True(t, config.Enabled)
		require.Equal(t, windows.five, config.Enabled5h)
		require.Equal(t, windows.weekly, config.Enabled7d)
		require.Zero(t, config.Threshold5h)
		require.Equal(t, 0.98, config.Threshold7d)
		require.NotNil(t, cached.Extra[service.OpenAIAutoResetCreditStateExtraKey])
	}
}
