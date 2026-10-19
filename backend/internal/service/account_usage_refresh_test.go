package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type usageRefreshRepo struct {
	stubOpenAIAccountRepo
	mu         sync.Mutex
	writes     map[int64]map[string]any
	persistErr error
}

func (r *usageRefreshRepo) UpdateExtra(_ context.Context, id int64, extra map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writes == nil {
		r.writes = make(map[int64]map[string]any)
	}
	r.writes[id] = extra
	return r.persistErr
}

type usageRefreshFetcher struct {
	mu        sync.Mutex
	calls     map[int64]int
	failID    int64
	err       error
	started   chan struct{}
	release   chan struct{}
	once      sync.Once
	active    int
	maxActive int
}

func (f *usageRefreshFetcher) FetchUsage(context.Context, string, string) (*ClaudeUsageResponse, error) {
	panic("use account options")
}
func (f *usageRefreshFetcher) FetchUsageWithOptions(ctx context.Context, opts *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error) {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = make(map[int64]int)
	}
	f.calls[opts.AccountID]++
	f.active++
	f.maxActive = max(f.maxActive, f.active)
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()
	if f.started != nil {
		f.once.Do(func() { close(f.started) })
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if opts.AccountID == f.failID {
		return nil, f.err
	}
	return &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(float64(opts.AccountID))}}, nil
}

func refreshTestService(accounts []Account, fetcher *usageRefreshFetcher) (*AccountUsageService, *usageRefreshRepo) {
	r := &usageRefreshRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: accounts}}
	return &AccountUsageService{accountRepo: r, usageLogRepo: &usageBatchLogRepoStub{}, usageFetcher: fetcher, cache: NewUsageCache()}, r
}

func refreshTestAccount(id int64) Account {
	return Account{ID: id, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"access_token": "fixture-only"}}
}

func TestClaudeBatchManualAndDueRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &usageRefreshFetcher{}
		s, repo := refreshTestService([]Account{refreshTestAccount(1), refreshTestAccount(2)}, f)
		usage, failures, err := s.GetUsageBatch(t.Context(), []int64{1, 2, 1}, true)
		require.NoError(t, err)
		require.Empty(t, failures)
		require.Len(t, usage, 2)
		require.Equal(t, 1.0, usage[1].FiveHour.Utilization)
		require.Equal(t, 2.0, usage[2].FiveHour.Utilization)
		require.Equal(t, "active", usage[1].Source)
		require.Len(t, repo.writes, 2)
		observed := *usage[1].FiveHour.ObservedAt
		for range 12 {
			time.Sleep(5 * time.Second)
			usage, failures, err = s.GetUsageBatch(t.Context(), []int64{1, 2}, false)
			require.NoError(t, err)
			require.Empty(t, failures)
			require.True(t, usage[1].Cached)
			require.Equal(t, observed, *usage[1].FiveHour.ObservedAt)
		}
		require.Equal(t, map[int64]int{1: 1, 2: 1}, f.calls, "five-second timer must reuse three-minute observations")
		time.Sleep(apiCacheTTL)
		_, failures, err = s.GetUsageBatch(t.Context(), []int64{1, 2}, false)
		require.NoError(t, err)
		require.Empty(t, failures)
		require.Equal(t, map[int64]int{1: 2, 2: 2}, f.calls)
		_, failures, err = s.GetUsageBatch(t.Context(), []int64{1, 2}, true)
		require.NoError(t, err)
		require.Empty(t, failures)
		require.Equal(t, map[int64]int{1: 3, 2: 3}, f.calls)
	})
}

func TestClaudeBatchEligibilityAndPartialFailure(t *testing.T) {
	accounts := []Account{refreshTestAccount(1), refreshTestAccount(2), refreshTestAccount(3), refreshTestAccount(4), refreshTestAccount(5), refreshTestAccount(6), refreshTestAccount(7), refreshTestAccount(8)}
	accounts[2].Type = AccountTypeSetupToken
	accounts[3].Type = AccountTypeAPIKey
	accounts[4].Credentials = nil
	accounts[5].Status = StatusDisabled
	accounts[6].Credentials["scope"] = "user:inference"
	accounts[7].Credentials["scope"] = "user:inference user:profile"
	accounts[1].Extra = map[string]any{"passive_usage_7d_utilization": 0.45, "passive_usage_7d_utilization_observed_at": time.Now().Add(-time.Hour).Format(time.RFC3339Nano)}
	f := &usageRefreshFetcher{failID: 2, err: errors.New("fixture usage unavailable")}
	s, _ := refreshTestService(accounts, f)
	usage, failures, err := s.GetUsageBatch(t.Context(), []int64{1, 2, 3, 4, 5, 6, 7, 8, 99}, true)
	require.NoError(t, err)
	require.Equal(t, map[int64]int{1: 1, 2: 1, 8: 1}, f.calls)
	require.NotNil(t, usage[1])
	require.Equal(t, "fixture usage unavailable", failures[2])
	require.Equal(t, 45.0, usage[2].SevenDay.Utilization)
	require.True(t, usage[2].SevenDay.Stale)
	require.Contains(t, failures[4], "does not support usage query")
	require.Contains(t, failures, int64(99))
	for _, id := range []int64{3, 5, 6, 7} {
		require.NotNil(t, usage[id])
		require.NotContains(t, failures, id)
	}
}

func TestClaudeRefreshHonorsRetryAfterAndBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &usageRefreshFetcher{failID: 1, err: NewClaudeUsageRateLimitError(http.Header{"Retry-After": []string{"600"}})}
		s, _ := refreshTestService([]Account{refreshTestAccount(1)}, f)
		for range 2 {
			_, _, err := s.GetUsageBatch(t.Context(), []int64{1}, true)
			require.NoError(t, err)
		}
		require.Equal(t, 1, f.calls[1], "force must not bypass 429 cooldown")
		time.Sleep(5 * time.Minute)
		_, _, err := s.GetUsageBatch(t.Context(), []int64{1}, false)
		require.NoError(t, err)
		require.Equal(t, 1, f.calls[1])
		time.Sleep(5 * time.Minute)
		_, _, err = s.GetUsageBatch(t.Context(), []int64{1}, false)
		require.NoError(t, err)
		require.Equal(t, 2, f.calls[1])
		entry, _ := s.cache.apiCache.Load(int64(1))
		cached, ok := entry.(*apiUsageCache)
		require.True(t, ok)
		require.NotNil(t, cached)
		require.GreaterOrEqual(t, time.Until(cached.retryAt), 2*time.Minute)
	})
}

func TestClaudeConcurrentForcedRefreshDeduplicatesAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &usageRefreshFetcher{started: make(chan struct{}), release: make(chan struct{})}
		s, _ := refreshTestService([]Account{refreshTestAccount(1)}, f)
		ctx, cancel := context.WithCancel(t.Context())
		canceled := make(chan error, 1)
		go func() { _, err := s.GetUsage(ctx, 1, true); canceled <- err }()
		<-f.started
		var wg sync.WaitGroup
		for range 12 {
			wg.Go(func() { _, err := s.GetUsage(t.Context(), 1, true); require.NoError(t, err) })
		}
		time.Sleep(apiQueryMaxJitter + time.Second)
		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-canceled, context.Canceled)
		close(f.release)
		wg.Wait()
		require.Equal(t, 1, f.calls[1], "concurrent batch/query callers share one account fetch")
	})
}

func TestClaudePartialRefreshPreservesWeeklyObservation(t *testing.T) {
	old := time.Now().UTC().Add(-time.Hour)
	a := refreshTestAccount(1)
	a.Extra = map[string]any{"passive_usage_7d_utilization": 0.72, "passive_usage_7d_utilization_source": "active", "passive_usage_7d_utilization_observed_at": old.Format(time.RFC3339Nano)}
	s, repo := refreshTestService([]Account{a}, &usageRefreshFetcher{})
	usage, failures, err := s.GetUsageBatch(t.Context(), []int64{1}, true)
	require.NoError(t, err)
	require.Empty(t, failures)
	require.Equal(t, 72.0, usage[1].SevenDay.Utilization)
	require.Equal(t, old, *usage[1].SevenDay.ObservedAt)
	require.True(t, usage[1].SevenDay.Stale)
	require.NotContains(t, repo.writes[1], "passive_usage_7d_utilization_observed_at")
	require.NotContains(t, repo.writes[1], "passive_usage_7d_utilization")
	require.NotContains(t, repo.writes[1], "passive_usage_sampled_at", "active windows must not renew the legacy timestamp for unrelated windows")
}

func TestClaudeRefreshPersistenceFailureRemainsVisible(t *testing.T) {
	s, repo := refreshTestService([]Account{refreshTestAccount(1)}, &usageRefreshFetcher{})
	repo.persistErr = errors.New("fixture DB unavailable")
	_, failures, err := s.GetUsageBatch(t.Context(), []int64{1}, true)
	require.NoError(t, err)
	require.Equal(t, "persist Claude usage observation failed", failures[1])
}

func TestClaudePartialRefreshRetainsFiveHourStats(t *testing.T) {
	a := refreshTestAccount(1)
	old := time.Now().UTC().Add(-time.Hour)
	a.Extra = map[string]any{"session_window_utilization": 0.3, "session_window_utilization_source": "active", "session_window_utilization_observed_at": old.Format(time.RFC3339Nano)}
	r := &usageRefreshRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{a}}}
	f := &claudeAccuracyFetcher{response: &ClaudeUsageResponse{SevenDay: ClaudeUsageWindow{Utilization: claudeTestPercent(50)}}}
	s := &AccountUsageService{accountRepo: r, usageLogRepo: &usageBatchLogRepoStub{}, usageFetcher: f, cache: NewUsageCache()}
	s.cache.windowStatsCache.Store(int64(1), &windowStatsCache{stats: &WindowStats{Requests: 7}, timestamp: time.Now()})
	usage, failures, err := s.GetUsageBatch(t.Context(), []int64{1}, true)
	require.NoError(t, err)
	require.Empty(t, failures)
	require.Equal(t, old, *usage[1].FiveHour.ObservedAt)
	require.NotNil(t, usage[1].FiveHour.WindowStats)
	require.Equal(t, int64(7), usage[1].FiveHour.WindowStats.Requests)
}

func TestClaudePartialRefreshDoesNotRenewLegacyWeeklyTimestamp(t *testing.T) {
	old := time.Now().UTC().Add(-time.Hour)
	a := refreshTestAccount(1)
	a.Extra = map[string]any{"passive_usage_7d_utilization": 0.72, "passive_usage_sampled_at": old.Format(time.RFC3339Nano)}
	r := &claudeAccuracyRepo{account: &a}
	f := &claudeAccuracyFetcher{response: &ClaudeUsageResponse{FiveHour: ClaudeUsageWindow{Utilization: claudeTestPercent(30)}}}
	s := &AccountUsageService{accountRepo: r, usageLogRepo: &usageBatchLogRepoStub{}, usageFetcher: f, cache: NewUsageCache()}
	_, err := s.GetUsage(t.Context(), 1, true)
	require.NoError(t, err)
	passive, err := s.GetPassiveUsage(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, old, *passive.SevenDay.ObservedAt)
	require.True(t, passive.SevenDay.Stale)
}

func TestClaudeFleetRefreshBoundsConcurrency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var accounts []Account
		var ids []int64
		for id := int64(1); id <= 20; id++ {
			accounts = append(accounts, refreshTestAccount(id))
			ids = append(ids, id)
		}
		f := &usageRefreshFetcher{release: make(chan struct{})}
		s, _ := refreshTestService(accounts, f)
		done := make(chan struct{})
		go func() {
			defer close(done)
			usage, failures, err := s.GetUsageBatch(t.Context(), ids, true)
			require.NoError(t, err)
			require.Empty(t, failures)
			require.Len(t, usage, 20)
		}()
		time.Sleep(apiQueryMaxJitter + time.Second)
		synctest.Wait()
		f.mu.Lock()
		active := f.active
		f.mu.Unlock()
		require.Equal(t, 6, active)
		close(f.release)
		<-done
		require.LessOrEqual(t, f.maxActive, 6)
	})
}
