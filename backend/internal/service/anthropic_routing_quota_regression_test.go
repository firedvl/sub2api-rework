//go:build unit

package service

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCompositeOwnershipIncludesNativeClaudeInventory(t *testing.T) {
	native := Account{ID: 1, Platform: PlatformAnthropic}
	native.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Models: []string{
		"claude-opus-4-6", "claude-haiku-4-5-20251001", "claude-sonnet-4-5-20250929",
	}})
	proxy := Account{ID: 2, Platform: PlatformAntigravity}
	for _, model := range []string{"claude-opus-4-6", "claude-haiku-4-5", "claude-sonnet-4-5"} {
		t.Run(model, func(t *testing.T) {
			require.Equal(t, CompositeModelOwnership{TargetPlatform: PlatformAnthropic, Matched: true},
				compositeModelOwnershipFromAccounts([]Account{native, proxy}, model, true))
			require.Equal(t, CompositeModelOwnership{Ambiguous: true},
				compositeModelOwnershipFromAccounts([]Account{native, proxy}, model, false))
			ctx := WithCompositeRouteDecision(context.Background(), CompositeRouteDecision{
				Matched: true, Source: CompositeRouteSourceAccount, PublicModel: model, TargetPlatform: PlatformAnthropic, UpstreamModel: model,
			})
			require.True(t, (&GatewayService{}).isModelSupportedByAccountWithContext(ctx, &native, model), "the inventory owner must remain selectable")
		})
	}
	native.Credentials = map[string]any{"model_mapping": map[string]any{"claude-opus-5-5": "claude-opus-5-5"}}
	require.Equal(t, CompositeModelOwnership{TargetPlatform: PlatformAntigravity, Matched: true},
		compositeModelOwnershipFromAccounts([]Account{native, proxy}, "claude-opus-4-6", true), "inventory must not expand an explicit allowlist")
	for _, model := range []string{"claude-opus-4-6", "claude-haiku-4-5"} {
		native.Credentials = map[string]any{"model_mapping": map[string]any{anthropicModelRateLimitKey(model): ""}}
		require.Equal(t, CompositeModelOwnership{TargetPlatform: PlatformAntigravity, Matched: true},
			compositeModelOwnershipFromAccounts([]Account{native, proxy}, model, true), "inventory must not revive an empty mapping target")
	}
}

func TestAnthropicOpusWeeklyLimitKeepsOtherFamiliesEligible(t *testing.T) {
	reset := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d_opus-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-7d_opus-reset", strconv.FormatInt(reset.Unix(), 10))
	repo := &anthropicWindowLimitRepo{}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
		http.StatusTooManyRequests, headers, nil, "claude-opus-5-5")
	require.Zero(t, repo.rateLimitCalls)
	require.Equal(t, 1, repo.modelRateLimitCalls)
	setAccountModelRateLimitSnapshot(account, repo.lastModelRateLimitScope, reset, "test", time.Now())
	require.True(t, account.isModelRateLimitedWithContext(context.Background(), "claude-opus-4-6[1m]"))
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "claude-sonnet-5-5"))
}

func TestAnthropicSonnetWeeklyLimitKeepsOtherFamiliesEligible(t *testing.T) {
	reset := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	headers := fable429Headers(time.Now().Add(time.Hour), reset)
	for _, suffix := range []string{"reset", "status", "utilization", "surpassed-threshold"} {
		headers.Del("anthropic-ratelimit-unified-7d_oi-" + suffix)
	}
	headers.Set("anthropic-ratelimit-unified-7d_sonnet-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-7d_sonnet-reset", strconv.FormatInt(reset.Unix(), 10))
	repo := &anthropicWindowLimitRepo{}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
		http.StatusTooManyRequests, headers, nil, "claude-sonnet-4-6")
	require.Zero(t, repo.rateLimitCalls)
	require.Zero(t, repo.sessionWindowCalls)
	require.Equal(t, 1, repo.modelRateLimitCalls)
	require.Equal(t, reset, repo.lastModelRateLimitReset)
	setAccountModelRateLimitSnapshot(account, repo.lastModelRateLimitScope, reset, "test", time.Now())
	for _, model := range []string{"claude-sonnet-4-6", "claude-sonnet-4-5-20250929", "claude-sonnet-5-5[1m]"} {
		require.True(t, account.isModelRateLimitedWithContext(context.Background(), model), model)
	}
	for _, model := range []string{"claude-opus-4-6", "claude-fable-5-2", "claude-haiku-4-5"} {
		require.False(t, account.isModelRateLimitedWithContext(context.Background(), model), model)
	}
}

func TestAnthropicSharedWeeklyRejectedStatusHasPriority(t *testing.T) {
	reset := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	headers.Set("anthropic-ratelimit-unified-7d-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(reset.Unix(), 10))
	repo := &anthropicWindowLimitRepo{}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
		http.StatusTooManyRequests, headers, nil, "claude-opus-5-5")
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Equal(t, reset, repo.lastRateLimitReset)
}

func TestAnthropicModelCreditsDoNotBlockOtherModels(t *testing.T) {
	reset := time.Now().Add(15 * 24 * time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(reset.Unix(), 10))
	repo := &anthropicWindowLimitRepo{}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"details":{"error_code":"credits_required","model":"claude-haiku-4-5-20251001"},"message":"Usage credits are required for this model."}}`)
	NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
		http.StatusTooManyRequests, headers, body, "claude-haiku-4-5")
	require.Zero(t, repo.rateLimitCalls)
	require.Zero(t, repo.sessionWindowCalls)
	require.Equal(t, 1, repo.modelRateLimitCalls)
	setAccountModelRateLimitSnapshot(account, repo.lastModelRateLimitScope, reset, "test", time.Now())
	for _, model := range []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001", "claude-haiku-4-5[1m]"} {
		require.True(t, account.isModelRateLimitedWithContext(context.Background(), model), model)
	}
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "claude-opus-4-6"))
}

func TestAnthropicNativeAdapterKeepsQuotaScopeAndRequestedModel(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(strconv.FormatBool(shared), func(t *testing.T) {
			repo := &anthropicWindowLimitRepo{}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{map[string]any{
						"error_code": float64(http.StatusTooManyRequests), "keywords": []any{"rate limit"}, "duration_minutes": float64(1),
					}},
				},
			}
			headers := http.Header{}
			body := []byte(`{"error":{"details":{"error_code":"credits_required"},"message":"Usage credits are required for this model."}}`)
			if shared {
				headers.Set("anthropic-ratelimit-unified-7d-status", "rejected")
				headers.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))
				body = []byte(`{"error":{"message":"This request would exceed your account's rate limit."}}`)
			}
			svc := &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, nil, nil, nil)}
			require.False(t, svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, body, "claude-opus-5"))
			if shared {
				require.Equal(t, 1, repo.rateLimitCalls)
				require.Zero(t, repo.modelRateLimitCalls, "a custom model rule must not hide a real shared limit")
			} else {
				require.Equal(t, 1, repo.modelRateLimitCalls)
				require.Equal(t, "claude-opus-5", repo.lastModelRateLimitScope)
				require.Zero(t, repo.rateLimitCalls)
			}
		})
	}
}

func TestAnthropicRateLimitMillisecondsDoNotBecomeYears(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	for _, window := range []string{"5h", "7d", "aggregate"} {
		t.Run(window, func(t *testing.T) {
			headers := http.Header{}
			key := "anthropic-ratelimit-unified-reset"
			if window != "aggregate" {
				key = "anthropic-ratelimit-unified-" + window + "-reset"
				headers.Set("anthropic-ratelimit-unified-"+window+"-status", "rejected")
			}
			headers.Set(key, strconv.FormatInt(reset.UnixMilli(), 10))
			repo := &anthropicWindowLimitRepo{}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
			NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
				http.StatusTooManyRequests, headers, nil, "claude-opus-5-5")
			require.Equal(t, 1, repo.rateLimitCalls)
			require.Equal(t, reset, repo.lastRateLimitReset)
		})
	}
}

func TestAnthropicFableCreditsAreSpecificToTheRequestedVersion(t *testing.T) {
	for _, model := range []string{"claude-fable-5", "claude-fable-5-2"} {
		t.Run(model, func(t *testing.T) {
			repo := &anthropicWindowLimitRepo{}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
			body := []byte(`{"error":{"details":{"error_code":"credits_required","model":"` + model + `"}}}`)
			NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account,
				http.StatusTooManyRequests, http.Header{}, body, model)
			require.Equal(t, model, repo.lastModelRateLimitScope)
			setAccountModelRateLimitSnapshot(account, repo.lastModelRateLimitScope, time.Now().Add(time.Hour), repo.lastModelRateLimitReason, time.Now())
			require.True(t, account.isModelRateLimitedWithContext(context.Background(), model))
			other := "claude-fable-5"
			if model == other {
				other = "claude-fable-5-2"
			}
			require.False(t, account.isModelRateLimitedWithContext(context.Background(), other))
		})
	}
}

func TestAnthropicFamilyQuotaWithoutResetDoesNotBecomeShared(t *testing.T) {
	for _, window := range []string{"7d_oi", "7d_sonnet", "7d_opus"} {
		t.Run(window, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("anthropic-ratelimit-unified-"+window+"-status", "rejected")
			headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(time.Now().Add(15*24*time.Hour).Unix(), 10))
			repo := &anthropicWindowLimitRepo{}
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
			started := time.Now()
			NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, nil)
			require.Zero(t, repo.rateLimitCalls)
			require.Equal(t, 1, repo.modelRateLimitCalls)
			require.WithinDuration(t, started.Add(5*time.Second), repo.lastModelRateLimitReset, time.Second)
		})
	}
}

func TestAnthropicThroughputRetryAfterDoesNotUseHealthyQuotaResets(t *testing.T) {
	for _, weeklyRejected := range []bool{false, true} {
		t.Run(strconv.FormatBool(weeklyRejected), func(t *testing.T) {
			now := time.Now()
			headers := http.Header{}
			headers.Set("Retry-After", "9")
			headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
			headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(now.Add(2*time.Hour).Unix(), 10))
			headers.Set("anthropic-ratelimit-unified-7d-status", "allowed")
			weeklyReset := now.Add(72 * time.Hour).Truncate(time.Second)
			headers.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(weeklyReset.Unix(), 10))
			if weeklyRejected {
				headers.Set("anthropic-ratelimit-unified-7d-status", "rejected")
			}
			repo := &anthropicWindowLimitRepo{}
			NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(),
				&Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}, http.StatusTooManyRequests, headers, nil)
			require.Equal(t, 1, repo.rateLimitCalls)
			if weeklyRejected {
				require.Equal(t, weeklyReset, repo.lastRateLimitReset)
			} else {
				require.WithinDuration(t, now.Add(9*time.Second), repo.lastRateLimitReset, time.Second)
				require.Zero(t, repo.sessionWindowCalls)
			}
		})
	}
}

func TestAnthropicInvalidRetryAfterUsesShortFallback(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "Inf", "not-a-date", "999999999999999999999"} {
		t.Run(value, func(t *testing.T) {
			now := time.Now()
			headers := http.Header{}
			headers.Set("Retry-After", value)
			for _, window := range []string{"5h", "7d"} {
				headers.Set("anthropic-ratelimit-unified-"+window+"-status", "allowed")
				headers.Set("anthropic-ratelimit-unified-"+window+"-reset", strconv.FormatInt(now.Add(time.Hour).Unix(), 10))
			}
			repo := &anthropicWindowLimitRepo{}
			NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(),
				&Account{ID: 1, Platform: PlatformAnthropic}, http.StatusTooManyRequests, headers, nil)
			require.WithinDuration(t, now.Add(5*time.Second), repo.lastRateLimitReset, time.Second)
		})
	}
}

func TestAnthropicMissingFamilyResetWithFallbackDisabledKeepsAccountEligible(t *testing.T) {
	repo := &anthropicWindowLimitRepo{}
	settings := newMockSettingRepo()
	settings.data[SettingKeyRateLimit429CooldownSettings] = `{"enabled":false,"cooldown_seconds":5}`
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	svc.settingService = NewSettingService(settings, &config.Config{})
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d_sonnet-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(time.Now().Add(15*24*time.Hour).Unix(), 10))
	svc.HandleUpstreamError(context.Background(), &Account{ID: 1, Platform: PlatformAnthropic}, http.StatusTooManyRequests, headers, nil)
	require.Zero(t, repo.rateLimitCalls)
	require.Zero(t, repo.modelRateLimitCalls)
}

func TestAnthropicAggregateResetDoesNotInventFiveHourWindow(t *testing.T) {
	repo := &anthropicWindowLimitRepo{}
	reset := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(reset.Unix(), 10))
	NewRateLimitService(repo, nil, nil, nil, nil).HandleUpstreamError(context.Background(),
		&Account{ID: 1, Platform: PlatformAnthropic}, http.StatusTooManyRequests, headers, nil)
	require.Equal(t, reset, repo.lastRateLimitReset)
	require.Zero(t, repo.sessionWindowCalls, "an aggregate reset does not identify the five-hour window")
}
