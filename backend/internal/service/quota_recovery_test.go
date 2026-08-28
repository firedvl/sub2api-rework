package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestAssessOpenAIQuotaRecovery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("exhausted quota remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(100, 20), now)
		require.False(t, recovered)
	})

	t.Run("future stale reset recovers on meaningful capacity", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(4*time.Hour), "5h")
		_, decision, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(40, 20), now)
		require.True(t, recovered)
		require.Equal(t, 60.0, decision.FreshRemaining["5h"])
	})

	t.Run("scheduled reset recovers", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(-time.Minute), "5h")
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(0, 20), now)
		require.True(t, recovered)
	})

	t.Run("near exhaustion jitter remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(99, 20), now)
		require.False(t, recovered)
	})

	t.Run("unknown quota remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		_, _, recovered := assessOpenAIQuotaRecovery(account, &OpenAIQuotaUsage{}, now)
		require.False(t, recovered)
	})

	t.Run("partial quota window remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "global")
		var usage OpenAIQuotaUsage
		require.NoError(t, json.Unmarshal([]byte(`{"rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"limit_window_seconds":18000}}}`), &usage))
		_, _, recovered := assessOpenAIQuotaRecovery(account, &usage, now)
		require.False(t, recovered)
	})

	t.Run("missing provider decision remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		var usage OpenAIQuotaUsage
		require.NoError(t, json.Unmarshal([]byte(`{"rate_limit":{"allowed":true,"primary_window":{"used_percent":20,"limit_window_seconds":18000}}}`), &usage))
		_, _, recovered := assessOpenAIQuotaRecovery(account, &usage, now)
		require.False(t, recovered)
	})

	t.Run("every limiting window must recover", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h", "7d")
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(20, 100), now)
		require.False(t, recovered)
	})

	t.Run("missing limiting window remains blocked", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "7d")
		usage := quotaRecoveryTestUsage(20, 20)
		usage.RateLimit.SecondaryWindow = nil
		_, _, recovered := assessOpenAIQuotaRecovery(account, usage, now)
		require.False(t, recovered)
	})

	t.Run("unchanged future reset still permits replenishment", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour)
		account, _ := quotaRecoveryTestAccount(t, now, resetAt, "5h")
		usage := quotaRecoveryTestUsage(25, 20)
		usage.RateLimit.PrimaryWindow.ResetAfterSeconds = int64(resetAt.Sub(now).Seconds())
		_, _, recovered := assessOpenAIQuotaRecovery(account, usage, now)
		require.True(t, recovered)
	})

	t.Run("provider must explicitly allow usage", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		usage := quotaRecoveryTestUsage(20, 20)
		usage.RateLimit.Allowed = false
		_, _, recovered := assessOpenAIQuotaRecovery(account, usage, now)
		require.False(t, recovered)
		usage.RateLimit.Allowed = true
		usage.RateLimit.LimitReached = true
		_, _, recovered = assessOpenAIQuotaRecovery(account, usage, now)
		require.False(t, recovered)
	})

	t.Run("minimum post-block floor prevents flapping", func(t *testing.T) {
		account, block := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		block.BlockedAt = now.Add(-10 * time.Second)
		setQuotaRecoveryTestBlock(t, account, block)
		account.RateLimitedAt = &block.BlockedAt
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(20, 20), now)
		require.False(t, recovered)
	})

	t.Run("auth and manual state cannot recover", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		account.Status = StatusError
		_, _, recovered := assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(20, 20), now)
		require.False(t, recovered)

		account.Status = StatusActive
		account.Schedulable = false
		_, _, recovered = assessOpenAIQuotaRecovery(account, quotaRecoveryTestUsage(20, 20), now)
		require.False(t, recovered)
	})
}

func TestOpenAIQuotaRecoveryCASAndRestart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	usage := quotaRecoveryTestUsage(20, 10)

	t.Run("lost compare and set keeps runtime block", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		repo := &quotaRecoveryAccountRepoStub{account: account, clearResult: false}
		recoverer := &quotaRecoveryRuntimeStub{}
		service := &OpenAIQuotaAutoResetService{accountRepo: repo, recoverer: recoverer}

		recovered, err := service.tryRecoverQuotaBlock(context.Background(), account, usage, now)
		require.NoError(t, err)
		require.False(t, recovered)
		require.Equal(t, 1, repo.clearCalls)
		require.Empty(t, recoverer.clearedAccountIDs)
		require.NotNil(t, repo.account.RateLimitResetAt)
	})

	t.Run("new service instance recovers persisted marker", func(t *testing.T) {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(4*time.Hour), "5h")
		repo := &quotaRecoveryAccountRepoStub{account: account, clearResult: true}
		quota := &quotaRecoveryQuotaStub{usage: usage}
		recoverer := &quotaRecoveryRuntimeStub{}
		service := NewOpenAIQuotaAutoResetService(repo, quota, recoverer, nil, nil, nil, nil)

		require.NoError(t, service.evaluateAccount(context.Background(), account.ID))
		require.Equal(t, 1, quota.queryCalls)
		require.Equal(t, 1, repo.clearCalls)
		require.Contains(t, repo.updatedExtra, "codex_5h_used_percent")
		require.Nil(t, repo.account.RateLimitedAt)
		require.Nil(t, repo.account.RateLimitResetAt)
		require.NotContains(t, repo.account.Extra, QuotaRateLimitBlockExtraKey)
		require.Equal(t, []int64{account.ID}, recoverer.clearedAccountIDs)
	})
}

func TestOpenAIQuotaRecoveryScannerIncludesPersistedBlocks(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	recoverable, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
	recoverable.ID = 11
	generic := *recoverable
	generic.ID = 12
	generic.Extra = map[string]any{}
	manual := *recoverable
	manual.ID = 13
	manual.Schedulable = false

	repo := &quotaRecoveryAccountRepoStub{recoveryCandidates: []Account{*recoverable, generic, manual}}
	service := NewOpenAIQuotaAutoResetService(repo, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, nil)
	defer service.cancel()
	for i := range cap(service.queue) {
		service.queue <- int64(1000 + i)
	}

	service.scanEligibleAccounts(context.Background())
	select {
	case accountID := <-service.recoveryQueue:
		require.Equal(t, recoverable.ID, accountID)
	default:
		t.Fatal("persisted quota block was not scheduled after restart")
	}
	select {
	case accountID := <-service.recoveryQueue:
		t.Fatalf("unexpected account %d scheduled for quota recovery", accountID)
	default:
	}
}

func TestOpenAIQuotaRecoveryScannerRotatesPastFullQueue(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	candidates := make([]Account, openAIAutoResetBatchSize+1)
	for i := range candidates {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		account.ID = int64(i + 1)
		candidates[i] = *account
	}
	repo := &quotaRecoveryAccountRepoStub{recoveryCandidates: candidates}
	service := NewOpenAIQuotaAutoResetService(repo, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, nil)
	defer service.cancel()
	for i := range cap(service.recoveryQueue) {
		id := int64(10_000 + i)
		service.pending.Store(id, struct{}{})
		service.recoveryQueue <- id
	}

	runScan := func() {
		done := make(chan struct{})
		go func() {
			service.scanQuotaRecoveryAccounts(context.Background(), false)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("quota recovery scan blocked on a full queue")
		}
	}

	runScan()
	require.Equal(t, int64(openAIAutoResetBatchSize), service.recoveryScanAfterID)
	drained := <-service.recoveryQueue
	service.pending.Delete(drained)
	runScan()
	_, queued := service.pending.Load(int64(openAIAutoResetBatchSize + 1))
	require.True(t, queued, "the next keyset batch should use the newly available slot")
	require.Zero(t, service.recoveryScanAfterID)
}

func TestOpenAIQuotaRecoveryScannerSharesCursorAcrossReplicas(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	candidates := make([]Account, openAIAutoResetBatchSize+1)
	for i := range candidates {
		account, _ := quotaRecoveryTestAccount(t, now, now.Add(time.Hour), "5h")
		account.ID = int64(i + 1)
		candidates[i] = *account
	}
	repo := &quotaRecoveryAccountRepoStub{recoveryCandidates: candidates}
	lock := &quotaRecoveryCursorLeaderLock{fakeLeaderLockCache: &fakeLeaderLockCache{}}
	first := NewOpenAIQuotaAutoResetService(repo, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, lock)
	second := NewOpenAIQuotaAutoResetService(repo, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, lock)
	defer first.cancel()
	defer second.cancel()

	first.scanEligibleAccounts(context.Background())
	require.Len(t, first.recoveryQueue, openAIAutoResetBatchSize)
	second.scanEligibleAccounts(context.Background())

	require.Equal(t, int64(openAIAutoResetBatchSize+1), <-second.recoveryQueue)
}

func TestOpenAIQuotaRecoverySharesInflightDedupe(t *testing.T) {
	service := NewOpenAIQuotaAutoResetService(&quotaRecoveryAccountRepoStub{}, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, nil)
	defer service.cancel()

	service.Notify(31)
	require.Equal(t, int64(31), <-service.queue)
	service.NotifyQuotaRecovery(31)
	select {
	case accountID := <-service.recoveryQueue:
		t.Fatalf("account %d was queued twice", accountID)
	default:
	}
}

func TestQuotaBlockPersistenceRenotifiesAfterCommit(t *testing.T) {
	account := &Account{ID: 21, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	repo := &quotaRecoveryAccountRepoStub{}
	notifier := NewOpenAIQuotaAutoResetService(repo, &quotaRecoveryQuotaStub{}, &quotaRecoveryRuntimeStub{}, nil, nil, nil, nil)
	setOpenAIAutoResetNotifier(notifier)
	defer func() {
		clearOpenAIAutoResetNotifier(notifier)
		notifier.cancel()
	}()

	notifyOpenAIAutoReset(account.ID)
	repo.onQuotaSet = func(accountID int64) {
		require.Equal(t, account.ID, <-notifier.queue)
		notifier.pending.Delete(accountID)
	}

	service := NewRateLimitService(repo, nil, nil, nil, nil)
	err := service.setOpenAIQuotaRateLimited(context.Background(), account, time.Now().Add(time.Hour), newOpenAIQuotaRateLimitBlock("test", map[string]float64{"5h": 100}), 0)
	require.NoError(t, err)
	require.Equal(t, account.ID, <-notifier.recoveryQueue)
}

type quotaRecoveryAccountRepoStub struct {
	AccountRepository
	account            *Account
	recoveryCandidates []Account
	clearResult        bool
	clearCalls         int
	observed           QuotaRateLimitBlock
	updatedExtra       map[string]any
	onQuotaSet         func(int64)
}

type quotaRecoveryCursorLeaderLock struct {
	*fakeLeaderLockCache
	cursorMu sync.Mutex
	cursors  map[string]int64
}

func (l *quotaRecoveryCursorLeaderLock) LoadScanCursor(_ context.Context, key string) (int64, error) {
	l.cursorMu.Lock()
	defer l.cursorMu.Unlock()
	return l.cursors[key], nil
}

func (l *quotaRecoveryCursorLeaderLock) StoreScanCursorIfLeader(_ context.Context, key, leaderKey, owner string, cursor int64) (bool, error) {
	if l.heldBy(leaderKey) != owner {
		return false, nil
	}
	l.cursorMu.Lock()
	defer l.cursorMu.Unlock()
	if l.cursors == nil {
		l.cursors = make(map[string]int64)
	}
	l.cursors[key] = cursor
	return true, nil
}

func (r *quotaRecoveryAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	if r.account == nil {
		return nil, nil
	}
	copy := *r.account
	copy.Extra = cloneOpenAIAutoResetExtra(r.account.Extra)
	return &copy, nil
}

func (r *quotaRecoveryAccountRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Page: 1, Pages: 1}, nil
}

func (r *quotaRecoveryAccountRepoStub) ListQuotaRecoveryCandidates(_ context.Context, afterID int64, limit int) ([]Account, error) {
	result := make([]Account, 0, limit)
	for _, account := range r.recoveryCandidates {
		if account.ID <= afterID {
			continue
		}
		result = append(result, account)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (r *quotaRecoveryAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updatedExtra == nil {
		r.updatedExtra = make(map[string]any)
	}
	for key, value := range updates {
		r.updatedExtra[key] = value
		if r.account != nil {
			if r.account.Extra == nil {
				r.account.Extra = make(map[string]any)
			}
			r.account.Extra[key] = value
		}
	}
	return nil
}

func (r *quotaRecoveryAccountRepoStub) SetQuotaRateLimited(_ context.Context, id int64, _ time.Time, _ QuotaRateLimitBlock) (bool, error) {
	if r.onQuotaSet != nil {
		r.onQuotaSet(id)
	}
	return true, nil
}

func (r *quotaRecoveryAccountRepoStub) ClearQuotaRateLimitIfObserved(_ context.Context, _ int64, block QuotaRateLimitBlock) (bool, error) {
	r.clearCalls++
	r.observed = block
	if r.clearResult && r.account != nil {
		r.account.RateLimitedAt = nil
		r.account.RateLimitResetAt = nil
		delete(r.account.Extra, QuotaRateLimitBlockExtraKey)
	}
	return r.clearResult, nil
}

type quotaRecoveryQuotaStub struct {
	usage      *OpenAIQuotaUsage
	queryCalls int
}

func (q *quotaRecoveryQuotaStub) QueryUsage(context.Context, int64) (*OpenAIQuotaUsage, error) {
	q.queryCalls++
	return q.usage, nil
}

func (q *quotaRecoveryQuotaStub) CacheResetCreditsSnapshot(context.Context, int64, *OpenAIRateLimitResetCredits) error {
	return nil
}

func (q *quotaRecoveryQuotaStub) ResetCreditTargeted(context.Context, int64, string, string) (*OpenAIQuotaResetResult, error) {
	return nil, nil
}

type quotaRecoveryRuntimeStub struct {
	clearedAccountIDs []int64
}

func (r *quotaRecoveryRuntimeStub) RecoverAccountState(context.Context, int64, AccountRecoveryOptions) (*SuccessfulTestRecoveryResult, error) {
	return &SuccessfulTestRecoveryResult{}, nil
}

func (r *quotaRecoveryRuntimeStub) ClearQuotaRecoveryRuntimeBlock(accountID int64) {
	r.clearedAccountIDs = append(r.clearedAccountIDs, accountID)
}

func quotaRecoveryTestAccount(t *testing.T, now, resetAt time.Time, windows ...string) (*Account, QuotaRateLimitBlock) {
	t.Helper()
	blockedAt := now.Add(-time.Minute)
	utilization := make(map[string]float64, len(windows))
	for _, window := range windows {
		utilization[window] = 100
	}
	block := newOpenAIQuotaRateLimitBlock("test", utilization)
	block.BlockedAt = blockedAt
	block.ResetAt = resetAt
	account := &Account{
		ID:               1,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		Schedulable:      true,
		RateLimitedAt:    &block.BlockedAt,
		RateLimitResetAt: &block.ResetAt,
		Extra:            map[string]any{},
	}
	setQuotaRecoveryTestBlock(t, account, block)
	return account, block
}

func setQuotaRecoveryTestBlock(t *testing.T, account *Account, block QuotaRateLimitBlock) {
	t.Helper()
	raw, err := json.Marshal(block)
	require.NoError(t, err)
	var persisted map[string]any
	require.NoError(t, json.Unmarshal(raw, &persisted))
	account.Extra[QuotaRateLimitBlockExtraKey] = persisted
}

func quotaRecoveryTestUsage(used5h, used7d float64) *OpenAIQuotaUsage {
	return &OpenAIQuotaUsage{
		FetchedAt: time.Now().Unix(),
		RateLimit: &OpenAIRateLimit{
			Allowed:             true,
			LimitReached:        false,
			allowedPresent:      true,
			limitReachedPresent: true,
			PrimaryWindow: &OpenAIRateLimitWindow{
				UsedPercent: used5h, LimitWindowSeconds: int64((5 * time.Hour).Seconds()), ResetAfterSeconds: int64(time.Hour.Seconds()), usedPercentPresent: true,
			},
			SecondaryWindow: &OpenAIRateLimitWindow{
				UsedPercent: used7d, LimitWindowSeconds: int64((7 * 24 * time.Hour).Seconds()), ResetAfterSeconds: int64((24 * time.Hour).Seconds()), usedPercentPresent: true,
			},
		},
	}
}
