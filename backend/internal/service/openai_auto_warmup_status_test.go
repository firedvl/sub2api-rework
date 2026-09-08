package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoWarmupAnchoredPreflightRecovery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account := newAutoWarmupTestAccount(1, now)
	reset := now.Add(3 * time.Hour)
	account.Extra["codex_5h_reset_at"] = reset.Format(time.RFC3339)
	account.Extra["codex_usage_updated_at"] = now.Add(-10 * time.Minute).Format(time.RFC3339)
	usage := newAutoWarmupTestUsage(3*time.Hour, 20)
	for _, test := range []struct {
		code     string
		status   string
		age      time.Duration
		eligible bool
	}{
		{"OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "failed", 11 * time.Minute, true},
		{"OPENAI_AUTO_WARMUP_MODEL_UNAVAILABLE", "failed", 11 * time.Minute, true},
		{"OPENAI_AUTO_WARMUP_MODEL_UNAVAILABLE", "failed", 9 * time.Minute, false},
		{"OPENAI_AUTO_WARMUP_UPSTREAM_FAILED", "failed", time.Hour, false},
		{"", "pending", time.Hour, false},
		{"", "succeeded", time.Hour, false},
	} {
		account.Extra[OpenAIAutoWarmupStateExtraKey] = &OpenAIAutoWarmupState{
			Status: test.status, ErrorCode: test.code, ResetAt: reset.Format(time.RFC3339), AttemptedAt: now.Add(-test.age).Format(time.RFC3339),
		}
		_, eligible := assessOpenAIAutoWarmupWindow(account, usage, false, now)
		require.Equal(t, test.eligible, eligible, test.code)
	}
}

func TestOpenAIAutoWarmupEvaluationIsManaged(t *testing.T) {
	extra := map[string]any{OpenAIAutoWarmupEvaluationExtraKey: map[string]any{"reason": "pending"}}
	got, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, extra)
	require.NoError(t, err)
	require.NotContains(t, got, OpenAIAutoWarmupEvaluationExtraKey)
	require.NotContains(t, stripOpenAIAutoResetCreditManagedExtra(extra, false), OpenAIAutoWarmupEvaluationExtraKey)
}

func TestOpenAIAutoWarmupQuotaFailureHasBoundedBackoff(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, age := range []time.Duration{time.Minute, 9 * time.Minute, 10 * time.Minute, time.Hour} {
		extra := map[string]any{OpenAIAutoWarmupEvaluationExtraKey: OpenAIAutoWarmupEvaluation{
			Reason: "quota_unavailable", CheckedAt: now.Add(-age).Format(time.RFC3339),
		}}
		require.Equal(t, age < OpenAIAutoWarmupPreflightRetry, openAIAutoWarmupQuotaRetryPending(extra, now))
	}
}
