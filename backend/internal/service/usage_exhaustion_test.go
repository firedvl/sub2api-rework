//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const usageExhaustionTestGroupID = int64(90)

func usageExhaustionTestAccount(id int64, platform string) Account {
	return Account{
		ID:            id,
		Platform:      platform,
		Type:          AccountTypeOAuth,
		Status:        StatusActive,
		Schedulable:   true,
		AccountGroups: []AccountGroup{{GroupID: usageExhaustionTestGroupID}},
	}
}

func diagnoseAnthropicUsageExhaustion(t *testing.T, model string, accounts ...Account) ModelAvailabilityDiagnosis {
	t.Helper()
	groupID := usageExhaustionTestGroupID
	svc := &GatewayService{
		accountRepo: &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{}},
		cfg:         testConfig(),
	}
	return svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, model, PlatformAnthropic)
}

func TestDiagnoseUsageExhaustion_EarliestResetAndFiveHourWindow(t *testing.T) {
	sooner := time.Now().Add(time.Hour).Truncate(time.Second)
	later := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	first := usageExhaustionTestAccount(1, PlatformAnthropic)
	first.RateLimitResetAt = &later
	second := usageExhaustionTestAccount(2, PlatformAnthropic)
	second.RateLimitResetAt = &sooner
	second.SessionWindowEnd = &sooner

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", first, second)

	require.True(t, diag.HasModelSupport)
	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(sooner), "the pool is usable again at the earliest account reset")
	require.Equal(t, "5h", diag.UsageExhaustion.Window)
}

func TestDiagnoseUsageExhaustion_SevenDayWindowFromStoredReset(t *testing.T) {
	reset := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	account := usageExhaustionTestAccount(1, PlatformAnthropic)
	account.RateLimitResetAt = &reset
	// A 429 without a 5h reset header also stores its reset as the session
	// window end; a reset three days out cannot be the five-hour window.
	account.SessionWindowEnd = &reset
	account.Extra = map[string]any{"passive_usage_7d_reset": reset.Unix()}

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", account)

	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(reset))
	require.Equal(t, "7d", diag.UsageExhaustion.Window)

	account.Extra = nil
	diag = diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", account)
	require.NotNil(t, diag.UsageExhaustion)
	require.Empty(t, diag.UsageExhaustion.Window, "an unidentified window is reported as unknown, never guessed")
}

func TestDiagnoseUsageExhaustion_LatestBlockOfOneAccountWithUnknownWindow(t *testing.T) {
	rateLimit := time.Now().Add(time.Hour)
	overload := time.Now().Add(10 * time.Minute)
	tempUnsched := time.Now().Add(2 * time.Hour)
	account := usageExhaustionTestAccount(1, PlatformAnthropic)
	account.RateLimitResetAt = &rateLimit
	account.OverloadUntil = &overload
	account.TempUnschedulableUntil = &tempUnsched

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", account)

	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(tempUnsched), "an account is usable only after its last block ends")
	require.Empty(t, diag.UsageExhaustion.Window)
}

func TestDiagnoseUsageExhaustion_UsableAccountMeansNotExhausted(t *testing.T) {
	reset := time.Now().Add(time.Hour)
	limited := usageExhaustionTestAccount(1, PlatformAnthropic)
	limited.RateLimitResetAt = &reset
	expiredCooldown := time.Now().Add(-time.Minute)
	usable := usageExhaustionTestAccount(2, PlatformAnthropic)
	usable.RateLimitResetAt = &expiredCooldown

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", limited, usable)

	require.True(t, diag.HasModelSupport)
	require.Nil(t, diag.UsageExhaustion, "a schedulable account means selection failed for another reason")
}

func TestDiagnoseUsageExhaustion_BlockWithoutResetContributesNoTime(t *testing.T) {
	expiredAt := time.Now().Add(-time.Hour)
	expired := usageExhaustionTestAccount(1, PlatformAnthropic)
	expired.AutoPauseOnExpired = true
	expired.ExpiresAt = &expiredAt
	// A cooldown on an account that stays blocked afterwards is not a reset.
	cooldown := time.Now().Add(5 * time.Minute)
	expired.OverloadUntil = &cooldown

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", expired)
	require.True(t, diag.HasModelSupport)
	require.Nil(t, diag.UsageExhaustion, "no candidate has a known reset, so no time is invented")

	reset := time.Now().Add(time.Hour)
	limited := usageExhaustionTestAccount(2, PlatformAnthropic)
	limited.RateLimitResetAt = &reset
	diag = diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", expired, limited)
	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(reset))
}

func TestDiagnoseUsageExhaustion_ModelScopedLimitOnlyAffectsItsModel(t *testing.T) {
	reset := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	account := usageExhaustionTestAccount(1, PlatformAnthropic)
	setAccountModelRateLimitSnapshot(&account, anthropicFableRateLimitKey, reset, anthropicFableWindowReason, time.Now())

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-fable-5", account)
	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(reset))
	require.Empty(t, diag.UsageExhaustion.Window, "the Fable-only window is not the 5h or 7d window")

	diag = diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", account)
	require.Nil(t, diag.UsageExhaustion, "other models stay schedulable on the account")
}

func TestDiagnoseUsageExhaustion_OnlyModelCapableAccountsCount(t *testing.T) {
	reset := time.Now().Add(time.Hour)
	capable := usageExhaustionTestAccount(1, PlatformAnthropic)
	capable.RateLimitResetAt = &reset
	capable.Credentials = map[string]any{"model_mapping": map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"}}
	otherModel := usageExhaustionTestAccount(2, PlatformAnthropic)
	otherModel.Credentials = map[string]any{"model_mapping": map[string]any{"claude-haiku-4-5": "claude-haiku-4-5"}}

	diag := diagnoseAnthropicUsageExhaustion(t, "claude-sonnet-4-6", capable, otherModel)
	require.NotNil(t, diag.UsageExhaustion, "a usable account that cannot serve the model does not hide the exhaustion")
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(reset))

	diag = diagnoseAnthropicUsageExhaustion(t, "claude-opus-4-8", capable, otherModel)
	require.True(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport)
	require.Nil(t, diag.UsageExhaustion, "model_not_found stays authoritative over a rate-limited pool")
}

func TestOpenAIDiagnoseUsageExhaustion_RateLimitAndQuotaAutoPause(t *testing.T) {
	groupID := usageExhaustionTestGroupID
	rateLimitReset := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	limited := usageExhaustionTestAccount(1, PlatformOpenAI)
	limited.RateLimitResetAt = &rateLimitReset

	// The scheduler pauses this account from its stored Codex snapshot without
	// persisting a cooldown; its window reset is still the known reset time.
	quotaReset := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	paused := usageExhaustionTestAccount(2, PlatformOpenAI)
	paused.Extra = map[string]any{
		"auto_pause_5h_threshold": 0.9,
		"codex_5h_used_percent":   100.0,
		"codex_5h_reset_at":       quotaReset.Format(time.RFC3339),
		"codex_usage_updated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	require.True(t, paused.IsSchedulable(), "quota auto-pause is not a persisted cooldown")

	svc := &OpenAIGatewayService{
		accountRepo: &mockAccountRepoForPlatform{accounts: []Account{limited, paused}, accountsByID: map[int64]*Account{}},
		cfg:         testConfig(),
	}
	diag := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "gpt-5.5", PlatformOpenAI)

	require.True(t, diag.HasModelSupport)
	require.NotNil(t, diag.UsageExhaustion)
	require.True(t, diag.UsageExhaustion.ResetAt.Equal(quotaReset))
	require.Empty(t, diag.UsageExhaustion.Window, "the window is reported for Anthropic accounts only")
}
