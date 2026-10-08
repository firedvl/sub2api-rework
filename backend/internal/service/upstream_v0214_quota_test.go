package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuotaStaleSnapshotRespectsKnownFutureReset(t *testing.T) {
	now := time.Now().UTC()
	for _, window := range []string{"5h", "7d"} {
		for _, reset := range []string{"absolute", "relative", "missing", "invalid", "past"} {
			t.Run(window+"/"+reset, func(t *testing.T) {
				extra := map[string]any{"codex_usage_updated_at": now.Add(-3 * time.Hour).Format(time.RFC3339), "codex_" + window + "_used_percent": 99.0}
				switch reset {
				case "absolute":
					extra["codex_"+window+"_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
				case "relative":
					extra["codex_"+window+"_reset_after_seconds"] = 4 * 3600
				case "invalid":
					extra["codex_"+window+"_reset_at"] = "invalid"
				case "past":
					extra["codex_"+window+"_reset_at"] = now.Add(-time.Minute).Format(time.RFC3339)
				}
				want := reset == "absolute" || reset == "relative"
				utilization, ok := resolveOpenAIQuotaUtilization(extra, window, now)
				require.Equal(t, want, ok)
				candidate := openAIThresholdCandidate(extra, window, now)
				require.Equal(t, want, candidate != nil)
				if want {
					require.Equal(t, 0.99, utilization)
					require.NotNil(t, candidate.until)
					require.True(t, now.Before(*candidate.until))
				}
			})
		}
	}
}

func TestAutoResetEvaluationSuppressesFreshNoCreditAndQueryFailures(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{ID: 501, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{
		OpenAIAutoResetCreditEnabledExtraKey:     true,
		OpenAIAutoResetCredit5hEnabledExtraKey:   false,
		OpenAIAutoResetCredit7dEnabledExtraKey:   true,
		OpenAIAutoResetCredit7dThresholdExtraKey: .95,
		"codex_7d_used_percent":                  99.0,
		"codex_7d_reset_at":                      now.Add(48 * time.Hour).Format(time.RFC3339),
		"codex_usage_updated_at":                 now.Format(time.RFC3339),
		OpenAIAutoResetCreditStateExtraKey:       &OpenAIAutoResetCreditState{Status: OpenAIAutoResetStatusNoCredit, TriggerWindow: "7d", CheckedAt: now.Format(time.RFC3339)},
	}}
	repo := &autoResetTestAccountRepo{account: account}
	quota := &autoResetTestQuota{usage: &OpenAIQuotaUsage{
		FetchedAt:             now.Unix(),
		RateLimit:             &OpenAIRateLimit{SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: 99, LimitWindowSeconds: 7 * 24 * 60 * 60, ResetAt: now.Add(48 * time.Hour).Unix()}},
		RateLimitResetCredits: &OpenAIRateLimitResetCredits{AvailableCount: 0},
	}}
	svc := NewOpenAIQuotaAutoResetService(repo, quota, nil, nil, nil, nil, nil)
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.evaluateAccount(context.Background(), account.ID))
	}
	require.Zero(t, quota.queryCalls.Load())
	stale := now.Add(-openAIAutoResetSnapshotTTL - time.Minute).Format(time.RFC3339)
	require.NoError(t, repo.UpdateExtra(context.Background(), account.ID, map[string]any{OpenAIAutoResetCreditStateExtraKey: &OpenAIAutoResetCreditState{Status: OpenAIAutoResetStatusNoCredit, CheckedAt: stale}, "codex_usage_updated_at": stale}))
	require.NoError(t, svc.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, int32(1), quota.queryCalls.Load())
	quota.queryErr = errors.New("synthetic upstream outage")
	require.NoError(t, repo.UpdateExtra(context.Background(), account.ID, map[string]any{OpenAIAutoResetCreditStateExtraKey: nil, "codex_usage_updated_at": stale}))
	require.Error(t, svc.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, int32(2), quota.queryCalls.Load())
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.evaluateAccount(context.Background(), account.ID))
	}
	require.Equal(t, int32(2), quota.queryCalls.Load())
	current, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	state := openAIAutoResetStateFromExtra(current.Extra)
	require.NotNil(t, state)
	state.LastResultAt = now.Add(-time.Minute - time.Second).Format(time.RFC3339)
	require.NoError(t, repo.UpdateExtra(context.Background(), account.ID, map[string]any{OpenAIAutoResetCreditStateExtraKey: state}))
	quota.queryErr = nil
	require.NoError(t, svc.evaluateAccount(context.Background(), account.ID))
	require.Equal(t, int32(3), quota.queryCalls.Load())
	require.Zero(t, quota.resetCalls.Load(), "all fixtures have no credits and must never redeem")
}

func TestAutoResetSchedulerNotificationCooldownIsAtomicAndScoped(t *testing.T) {
	service := &OpenAIQuotaAutoResetService{ctx: context.Background(), queue: make(chan int64, 8)}
	setOpenAIAutoResetNotifier(service)
	t.Cleanup(func() { clearOpenAIAutoResetNotifier(service) })
	const accountA, accountB int64 = 990001, 990002
	t.Cleanup(func() {
		openAIAutoResetSchedulerNotifiedAt.Delete(accountA)
		openAIAutoResetSchedulerNotifiedAt.Delete(accountB)
	})
	now := time.Now()
	var accepted atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if notifyOpenAIAutoResetFromSchedulerAt(accountA, now) {
				accepted.Add(1)
			}
		}()
	}
	wait.Wait()
	require.Equal(t, int32(1), accepted.Load())
	require.False(t, notifyOpenAIAutoResetFromSchedulerAt(accountA, now.Add(10*time.Second)))
	require.True(t, notifyOpenAIAutoResetFromSchedulerAt(accountB, now))
	require.True(t, notifyOpenAIAutoResetFromSchedulerAt(accountA, now.Add(30*time.Second)))
	// Event-driven notifications are not suppressed by scheduler cooldown.
	service.pending.Delete(accountA)
	NotifyOpenAIAutoResetCredit(accountA)
	require.Len(t, service.queue, 3)
}

func TestAutoResetQueryBackoffExcludesRedemptionFailures(t *testing.T) {
	now := time.Now().UTC()
	for _, code := range []string{"RESET_CREDIT_QUERY_FAILED", "USAGE_SNAPSHOT_WRITE_FAILED", "RESET_CREDIT_DETAILS_UNAVAILABLE", "OPENAI_RESET_CREDIT_FAILED", "OPENAI_RESET_CREDIT_TRANSPORT"} {
		state := &OpenAIAutoResetCreditState{Status: OpenAIAutoResetStatusFailed, ErrorCode: code, LastResultAt: now.Add(-10 * time.Second).Format(time.RFC3339)}
		want := code == "RESET_CREDIT_QUERY_FAILED" || code == "USAGE_SNAPSHOT_WRITE_FAILED" || code == "RESET_CREDIT_DETAILS_UNAVAILABLE"
		require.Equal(t, want, openAIAutoResetQueryFailureBackoffActive(state, now), code)
		state.LastResultAt = now.Add(-time.Minute - time.Second).Format(time.RFC3339)
		require.False(t, openAIAutoResetQueryFailureBackoffActive(state, now))
	}
	require.True(t, openAIAutoResetNoCreditConfirmed(&OpenAIAutoResetCreditState{Status: OpenAIAutoResetStatusNoCredit, CheckedAt: now.Format(time.RFC3339)}, now))
	require.False(t, openAIAutoResetNoCreditConfirmed(&OpenAIAutoResetCreditState{Status: OpenAIAutoResetStatusNoCredit, CheckedAt: now.Add(-openAIAutoResetSnapshotTTL - time.Minute).Format(time.RFC3339)}, now))
}
