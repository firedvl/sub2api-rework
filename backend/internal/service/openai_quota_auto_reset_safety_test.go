package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoResetCreditThresholdsAreUsedQuotaOR(t *testing.T) {
	service := &OpenAIQuotaAutoResetService{}
	account := &Account{Extra: map[string]any{
		"auto_pause_5h_disabled": true,
		"auto_pause_7d_disabled": true,
	}}
	config := OpenAIAutoResetCreditConfig{Enabled: true, Threshold5h: 0.9, Threshold7d: 0.9}

	for _, test := range []struct {
		name     string
		fiveHour float64
		sevenDay float64
		eligible bool
		trigger  string
	}{
		{name: "5h 89.9 percent is below threshold", fiveHour: 0.899, sevenDay: 0.2},
		{name: "5h 90 percent reaches threshold", fiveHour: 0.9, sevenDay: 0.2, eligible: true, trigger: "5h"},
		{name: "5h 95 percent reaches threshold", fiveHour: 0.95, sevenDay: 0.2, eligible: true, trigger: "5h"},
		{name: "weekly 89.9 percent is below threshold", fiveHour: 0.2, sevenDay: 0.899},
		{name: "weekly 90 percent reaches threshold", fiveHour: 0.2, sevenDay: 0.9, eligible: true, trigger: "7d"},
		{name: "weekly 95 percent reaches threshold", fiveHour: 0.2, sevenDay: 0.95, eligible: true, trigger: "7d"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assessment := service.buildAssessment(account, config, test.fiveHour, test.sevenDay)
			require.Equal(t, test.eligible, assessment.resetReached)
			require.Equal(t, test.trigger, assessment.triggerWindow)
		})
	}
}

func TestOpenAIAutoResetCreditDisabledNeverConsumes(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	quota := &autoResetTestQuota{}
	service := NewOpenAIQuotaAutoResetService(&autoResetTestAccountRepo{account: account}, quota, nil, nil, nil, nil, nil)

	require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
	require.Zero(t, quota.resetCalls.Load())
}

func TestOpenAIAutoResetCreditFailsClosedBeforeConsumption(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name       string
		credits    *OpenAIRateLimitResetCredits
		candidates []openAIAutoResetCreditCandidate
		wantStatus string
		wantCode   string
		wantErr    bool
	}{
		{name: "no reset credits", credits: &OpenAIRateLimitResetCredits{}, wantStatus: OpenAIAutoResetStatusNoCredit, wantCode: "NO_RESET_CREDIT"},
		{name: "incomplete reset credit details", credits: &OpenAIRateLimitResetCredits{AvailableCount: 1}, wantStatus: OpenAIAutoResetStatusFailed, wantCode: "OPENAI_AUTO_RESET_CREDIT_DETAILS_INCOMPLETE", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := &Account{
				ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
				Extra: map[string]any{
					OpenAIAutoResetCreditEnabledExtraKey:     true,
					OpenAIAutoResetCredit5hThresholdExtraKey: 0.9,
					OpenAIAutoResetCredit7dThresholdExtraKey: 0.9,
					"codex_5h_used_percent":                  95.0,
					"codex_usage_updated_at":                 now.Format(time.RFC3339),
					"codex_5h_reset_at":                      now.Add(time.Hour).Format(time.RFC3339),
				},
			}
			quota := &autoResetTestQuota{usage: &OpenAIQuotaUsage{
				FetchedAt: now.Unix(),
				RateLimit: &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{
					UsedPercent: 95, LimitWindowSeconds: 5 * 60 * 60, ResetAt: now.Add(time.Hour).Unix(),
				}},
				RateLimitResetCredits: test.credits,
				autoResetCandidates:   test.candidates,
			}}
			repo := &autoResetTestAccountRepo{account: account}
			service := NewOpenAIQuotaAutoResetService(repo, quota, nil, nil, nil, nil, nil)

			if test.wantErr {
				require.Error(t, service.evaluateAccount(context.Background(), account.ID))
			} else {
				require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
			}
			require.Zero(t, quota.resetCalls.Load())
			state := openAIAutoResetStateFromExtra(repo.account.Extra)
			require.NotNil(t, state)
			require.Equal(t, test.wantStatus, state.Status)
			require.Equal(t, test.wantCode, state.ErrorCode)
		})
	}
}

func TestOpenAIAutoResetCreditConfigAndBulkBounds(t *testing.T) {
	for _, threshold := range []float64{0.001, 1} {
		extra, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: threshold,
		})
		require.NoError(t, err)
		require.Equal(t, threshold, extra[OpenAIAutoResetCredit5hThresholdExtraKey])

		updates, err := bulkOpenAIAutoResetCreditUpdates(&BulkUpdateAccountsInput{AutoResetCredit5hThreshold: &threshold})
		require.NoError(t, err)
		require.Equal(t, threshold, updates[OpenAIAutoResetCredit5hThresholdExtraKey])
	}

	for _, threshold := range []float64{0, -0.001, 0.0009, 1.0001, math.Inf(1), math.NaN()} {
		_, err := normalizeOpenAIAutoResetCreditExtra(PlatformOpenAI, AccountTypeOAuth, false, map[string]any{
			OpenAIAutoResetCreditEnabledExtraKey:     true,
			OpenAIAutoResetCredit5hThresholdExtraKey: threshold,
		})
		require.Error(t, err)

		_, err = bulkOpenAIAutoResetCreditUpdates(&BulkUpdateAccountsInput{AutoResetCredit5hThreshold: &threshold})
		require.Error(t, err)
	}

	threshold := 0.9 // UI percent input 90 is converted to this API/storage ratio.
	updates, err := bulkOpenAIAutoResetCreditUpdates(&BulkUpdateAccountsInput{AutoResetCredit5hThreshold: &threshold})
	require.NoError(t, err)
	require.Equal(t, 0.9, updates[OpenAIAutoResetCredit5hThresholdExtraKey])

	tooLarge := 90.0
	_, err = bulkOpenAIAutoResetCreditUpdates(&BulkUpdateAccountsInput{AutoResetCredit5hThreshold: &tooLarge})
	require.Error(t, err)
}
