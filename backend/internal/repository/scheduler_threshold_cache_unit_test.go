//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type snapshotThresholdRepo struct {
	service.AccountRepository
	accountPauses, modelPauses int
}

func (r *snapshotThresholdRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.accountPauses++
	return nil
}

func (r *snapshotThresholdRepo) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	r.modelPauses++
	return nil
}

func TestSchedulerCacheAnthropicThresholdMetadataAndRefresh(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	end := now.Add(time.Hour)
	cache := newSchedulerCacheUnit(t)
	bucket := service.SchedulerBucket{GroupID: 8, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
	account := service.Account{
		ID: 3, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true, SessionWindowEnd: &end,
		Credentials: map[string]any{"account_scheduling_threshold": 60, "access_token": "test-only"},
	}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
	for _, tc := range []struct {
		name                    string
		fiveHour, weekly, fable float64
		expired                 bool
		pause                   bool
		model                   string
	}{
		{"5h", .60, 0, 0, false, true, ""},
		{"weekly", 0, .66, 0, false, true, ""},
		{"Fable", 0, .40, .75, false, false, "claude-fable-5"},
		{"below", .59, .59, .59, false, false, ""},
		{"expired", .75, .75, .75, true, false, ""},
		{"recovered", .10, .10, .10, false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset := end
			if tc.expired {
				reset = now.Add(-time.Hour)
			}
			account.SessionWindowEnd = &reset
			account.Extra = map[string]any{
				"session_window_utilization":   tc.fiveHour,
				"passive_usage_7d_utilization": tc.weekly, "passive_usage_7d_reset": reset.Unix(),
				"passive_usage_7d_oi_utilization": tc.fable, "passive_usage_7d_oi_reset": reset.Unix(),
				"unrelated_large_payload": "drop me",
			}
			require.NoError(t, cache.SetAccount(ctx, &account))
			candidates, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, candidates, 1)
			require.NotContains(t, candidates[0].Credentials, "access_token")
			require.NotContains(t, candidates[0].Extra, "unrelated_large_payload")
			full, err := cache.GetAccount(ctx, account.ID)
			require.NoError(t, err)
			for _, projected := range []*service.Account{candidates[0], full} {
				repo := &snapshotThresholdRepo{}
				limiter := service.NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
				limiter.SetSettingService(service.NewSettingService(nil, &config.Config{}))
				require.Equal(t, tc.pause, limiter.ApplyAccountSchedulingThreshold(ctx, projected))
				require.Equal(t, tc.pause, repo.accountPauses == 1)
				require.Equal(t, tc.model != "", repo.modelPauses == 1)
				if tc.model != "" {
					require.False(t, projected.IsSchedulableForModel(tc.model))
					require.True(t, projected.IsSchedulableForModel("claude-opus-5"))
				}
			}
		})
	}
}
