package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayEffectiveOpenAIRuntimeStateIsPassive(t *testing.T) {
	now := time.Now()
	account := &Account{ID: 7101, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc := &OpenAIGatewayService{}

	t.Run("expired account block remains stored", func(t *testing.T) {
		expired := now.Add(-time.Minute)
		svc.openaiAccountRuntimeBlockUntil.Store(account.ID, expired)

		require.Equal(t, gatewayEffectiveRuntimeAvailable, gatewayEffectiveOpenAIRuntimeState(context.Background(), svc, account, "gpt-5"))
		stored, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
		require.True(t, ok)
		require.Equal(t, expired, stored)
	})

	t.Run("model cooldown is read without touching the entry", func(t *testing.T) {
		state := newOpenAIAccountModelTransientState(8)
		key, ok := openAIAccountModelTransientKey(account.ID, canonicalOpenAIAccountSchedulingModel(account, "gpt-5"))
		require.True(t, ok)
		before := openAIAccountModelTransientEntry{
			failureStreak: 2,
			lastFailure:   now,
			blockUntil:    now.Add(time.Minute),
			lastTouched:   now.Add(-time.Minute),
		}
		state.entries[key] = before
		svc.openaiModelTransient = state

		require.Equal(t, gatewayEffectiveRuntimeTemporarilyUnavailable, gatewayEffectiveOpenAIRuntimeState(context.Background(), svc, account, "gpt-5"))
		state.mu.Lock()
		after := state.entries[key]
		state.mu.Unlock()
		require.Equal(t, before, after)
	})

	t.Run("auto reset quota is evaluated without notification", func(t *testing.T) {
		resetService := &OpenAIQuotaAutoResetService{
			ctx:   context.Background(),
			queue: make(chan int64, 1),
		}
		setOpenAIAutoResetNotifier(resetService)
		t.Cleanup(func() { clearOpenAIAutoResetNotifier(resetService) })
		quotaAccount := &Account{
			ID:       7102,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				OpenAIAutoResetCreditEnabledExtraKey:     true,
				OpenAIAutoResetCredit5hThresholdExtraKey: 1.0,
				"codex_5h_used_percent":                  100.0,
				"codex_usage_updated_at":                 now.Format(time.RFC3339),
				"codex_5h_reset_at":                      now.Add(time.Hour).Format(time.RFC3339),
			},
		}

		require.Equal(t, gatewayEffectiveRuntimeTemporarilyUnavailable, gatewayEffectiveOpenAIRuntimeState(context.Background(), &OpenAIGatewayService{}, quotaAccount, "gpt-5"))
		require.Empty(t, resetService.queue)
		_, notified := resetService.pending.Load(quotaAccount.ID)
		require.False(t, notified)
	})
}
