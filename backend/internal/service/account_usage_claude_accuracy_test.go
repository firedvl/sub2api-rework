package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClaudeUsageMissingDoesNotBecomeZero(t *testing.T) {
	for _, raw := range []string{`{}`, `{"five_hour":null}`, `{"five_hour":{}}`, `{"five_hour":{"utilization":null}}`, `{"five_hour":{"resets_at":"2099-01-01T00:00:00Z"}}`} {
		t.Run(raw, func(t *testing.T) {
			var response ClaudeUsageResponse
			require.NoError(t, json.Unmarshal([]byte(raw), &response))
			info := (&AccountUsageService{}).buildUsageInfo(&response, nil)
			require.Nil(t, info.FiveHour, "missing measurement must remain unavailable")
		})
	}
}

func TestClaudePassiveMissingDoesNotBecomeZero(t *testing.T) {
	for _, extra := range []map[string]any{{"r": time.Now().Add(time.Hour).Unix()}, {"u": nil, "r": time.Now().Add(time.Hour).Unix()}, {"u": "bad"}} {
		require.Nil(t, buildPassiveUsageWindow(extra, "u", "r"))
	}
	require.NotNil(t, buildPassiveUsageWindow(map[string]any{"u": float64(0)}, "u", "r"), "measured zero must remain present")
	require.Nil(t, (&AccountUsageService{}).estimateSetupTokenUsage(&Account{}).FiveHour, "no sample is not zero usage")
}

func TestClaudeUsageUnitsAndExpiration(t *testing.T) {
	now := time.Now().UTC()
	for _, percent := range []float64{0, 0.5, 1, 42.5, 100, 102} {
		response := ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(percent), ResetsAt: now.Add(-time.Second).Format(time.RFC3339)}}
		info := (&AccountUsageService{}).buildUsageInfo(&response, &now)
		require.Equal(t, percent, info.FiveHour.Utilization, "active values are already percentages, including values below 1")
		require.True(t, info.FiveHour.Stale)
		require.Zero(t, info.FiveHour.RemainingSeconds)
		passive := buildPassiveUsageWindow(map[string]any{"u": percent / 100, "u_observed_at": now.Format(time.RFC3339Nano)}, "u", "r")
		require.InDelta(t, percent, passive.Utilization, 1e-9)
	}
	for _, invalid := range []any{-0.1, "NaN", "+Inf", "-Inf", true} {
		require.Nil(t, buildPassiveUsageWindow(map[string]any{"u": invalid}, "u", "r"))
	}
}

type claudeAccuracyRepo struct {
	sessionWindowSyncRepo
	account *Account
}

func (r *claudeAccuracyRepo) GetByID(context.Context, int64) (*Account, error) { return r.account, nil }
func (r *claudeAccuracyRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for key, value := range updates {
		r.account.Extra[key] = value
	}
	return r.sessionWindowSyncRepo.UpdateExtra(ctx, id, updates)
}
func (r *claudeAccuracyRepo) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	r.account.SessionWindowEnd = &end
	return r.sessionWindowSyncRepo.UpdateSessionWindowEnd(ctx, id, end)
}

type claudeAccuracyFetcher struct {
	calls    int
	response *ClaudeUsageResponse
	err      error
}

func (f *claudeAccuracyFetcher) FetchUsage(context.Context, string, string) (*ClaudeUsageResponse, error) {
	f.calls++
	return f.response, f.err
}
func (f *claudeAccuracyFetcher) FetchUsageWithOptions(context.Context, *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error) {
	f.calls++
	return f.response, f.err
}

func TestClaudeUsageCacheRefreshAndPassiveActiveTransitions(t *testing.T) {
	observed := time.Now().UTC().Add(-time.Minute)
	reset := time.Now().UTC().Add(time.Hour)
	repo := &claudeAccuracyRepo{account: &Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "synthetic"}, SessionWindowEnd: &reset, Extra: map[string]any{
		"session_window_utilization": 0.75, "session_window_utilization_source": "passive", "session_window_utilization_observed_at": observed.Format(time.RFC3339Nano),
	}}}
	fetcher := &claudeAccuracyFetcher{response: &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(25), ResetsAt: reset.Format(time.RFC3339)}}}
	svc := &AccountUsageService{accountRepo: repo, usageLogRepo: &usageBatchLogRepoStub{}, usageFetcher: fetcher, cache: &UsageCache{}}
	passive, err := svc.GetPassiveUsage(t.Context(), 42)
	require.NoError(t, err)
	require.Equal(t, "passive", passive.FiveHour.Source)
	require.Equal(t, 75.0, passive.FiveHour.Utilization)
	active, err := svc.GetUsage(t.Context(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, fetcher.calls)
	require.Equal(t, "active", active.FiveHour.Source)
	require.Equal(t, 25.0, active.FiveHour.Utilization)
	require.False(t, active.Cached)
	require.Len(t, repo.extraUpdates, 1)

	// A memory cache read cannot create a new observation or persistence write.
	cached, err := svc.GetUsage(t.Context(), 42)
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.Equal(t, active.UpdatedAt, cached.UpdatedAt)
	require.Equal(t, 1, fetcher.calls)
	require.Len(t, repo.extraUpdates, 1)
	passive, err = svc.GetPassiveUsage(t.Context(), 42)
	require.NoError(t, err)
	require.Equal(t, "active", passive.FiveHour.Source)
	require.Equal(t, active.UpdatedAt, passive.FiveHour.ObservedAt)

	// Force must bypass a valid success cache, while missing weekly data stays absent.
	fetcher.response = &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(30)}}
	refreshed, err := svc.GetUsage(t.Context(), 42, true)
	require.NoError(t, err)
	require.Equal(t, 2, fetcher.calls)
	require.Equal(t, 30.0, refreshed.FiveHour.Utilization)
	require.Nil(t, refreshed.FiveHour.ResetsAt)
	require.Nil(t, refreshed.SevenDay)
	passive, err = svc.GetPassiveUsage(t.Context(), 42)
	require.NoError(t, err)
	require.Nil(t, passive.FiveHour.ResetsAt, "active reset:null cannot inherit the previous session reset")

	// Negative cache and cache TTL are also bypassed/expired without sleeps.
	fetcher.err = errors.New("synthetic upstream error")
	_, err = svc.GetUsage(t.Context(), 42, true)
	require.Error(t, err)
	calls := fetcher.calls
	_, err = svc.GetUsage(t.Context(), 42)
	require.Error(t, err)
	require.Equal(t, calls, fetcher.calls)
	fetcher.err = nil
	_, err = svc.GetUsage(t.Context(), 42, true)
	require.NoError(t, err)
	require.Equal(t, calls+1, fetcher.calls)
	svc.cache.apiCache.Store(int64(42), &apiUsageCache{response: fetcher.response, timestamp: time.Now().Add(-apiCacheTTL - time.Second)})
	_, err = svc.GetUsage(t.Context(), 42)
	require.NoError(t, err)
	require.Equal(t, calls+2, fetcher.calls)
}

func TestClaudePassivePerWindowObservationAndRollover(t *testing.T) {
	old := time.Now().UTC().Add(-time.Hour)
	weeklyReset := time.Now().Add(24 * time.Hour).Unix()
	repo := &sessionWindowMockRepo{}
	svc := newRateLimitServiceForTest(repo)
	account := &Account{ID: 1, Extra: map[string]any{"passive_usage_7d_utilization": 0.8, "passive_usage_7d_reset": weeklyReset, "passive_usage_7d_utilization_observed_at": old.Format(time.RFC3339Nano)}}
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.25")
	svc.UpdateSessionWindow(t.Context(), account, headers)
	for _, call := range repo.updateExtraCalls {
		require.NotContains(t, call.Updates, "passive_usage_7d_utilization", "5h rollover cannot clear weekly data")
		require.NotContains(t, call.Updates, "passive_usage_7d_utilization_observed_at", "5h observation cannot refresh weekly timestamp")
	}
	weekly := buildPassiveUsageWindow(account.Extra, "passive_usage_7d_utilization", "passive_usage_7d_reset")
	require.Equal(t, old, *weekly.ObservedAt)
	require.True(t, weekly.Stale)

	// Weekly-only headers are sampled, and a reset-only rollover cannot reuse old usage.
	headers = http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(weeklyReset+3600, 10))
	svc.UpdateSessionWindow(t.Context(), account, headers)
	last := repo.updateExtraCalls[len(repo.updateExtraCalls)-1].Updates
	require.Contains(t, last, "passive_usage_7d_utilization")
	require.Nil(t, last["passive_usage_7d_utilization"])
}

type claudeBlockingPersistenceRepo struct {
	AccountRepository
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *claudeBlockingPersistenceRepo) UpdateExtra(context.Context, int64, map[string]any) error {
	r.once.Do(func() { close(r.started); <-r.release })
	return nil
}
func TestClaudeForcedRefreshCannotJoinOlderPersistence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &claudeBlockingPersistenceRepo{started: make(chan struct{}), release: make(chan struct{})}
		fetcher := &claudeAccuracyFetcher{response: &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(25)}}}
		account := &Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "synthetic"}}
		svc := &AccountUsageService{accountRepo: repo, usageLogRepo: &usageBatchLogRepoStub{}, usageFetcher: fetcher, cache: &UsageCache{}}
		first := make(chan error, 1)
		go func() { _, err := svc.getUsageForAccount(t.Context(), account, false); first <- err }()
		<-repo.started
		time.Sleep(time.Nanosecond)
		requestStarted := time.Now()
		forced := make(chan *UsageInfo, 1)
		errors := make(chan error, 1)
		go func() { info, err := svc.getUsageForAccount(t.Context(), account, true); forced <- info; errors <- err }()
		// Advance the synthetic clock past jitter so the refresh joins the blocked flight.
		time.Sleep(apiQueryMaxJitter + time.Second)
		synctest.Wait()
		close(repo.release)
		require.NoError(t, <-first)
		info := <-forced
		require.NoError(t, <-errors)
		require.Equal(t, 2, fetcher.calls, "forced refresh must fetch after joining an older completed observation")
		require.False(t, info.UpdatedAt.Before(requestStarted))
	})
}
