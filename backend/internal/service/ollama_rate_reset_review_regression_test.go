//go:build unit

package service

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reviewOllamaRepository struct {
	*ollama429Repo
	clearBeforeReread bool
	conflictOnce      bool
	conflicts         int
	recordCalls       int
	clearOnConflict   bool
	swapOnConflict    bool
	afterCAS          func()
}

func (repository *reviewOllamaRepository) RecordOllamaCloudUsage429(ctx context.Context, observed *Account, resetAt *time.Time) (*AccountRateLimitGeneration, error) {
	repository.recordCalls++
	if repository.conflicts > 0 {
		repository.conflicts--
		repository.mutate(observed.ID, func(account *Account) {
			if repository.clearOnConflict {
				account.RateLimitedAt, account.RateLimitResetAt = nil, nil
			}
			if repository.swapOnConflict {
				account.Credentials["api_key"] = "replacement-fixture"
			}
		})
		return nil, nil
	}
	if repository.conflictOnce {
		repository.conflictOnce = false
		short := time.Now().Add(5 * time.Second)
		_, err := repository.ollama429Repo.RecordOllamaCloudUsage429(ctx, observed, &short)
		return nil, err
	}
	updated, err := repository.ollama429Repo.RecordOllamaCloudUsage429(ctx, observed, resetAt)
	if updated != nil && repository.clearBeforeReread {
		repository.mutate(observed.ID, func(account *Account) { account.RateLimitedAt, account.RateLimitResetAt = nil, nil })
	}
	return updated, err
}

func TestOllama429ConflictRetriesAreBoundedAndRejectClearOrIdentityChange(t *testing.T) {
	for _, scenario := range []string{"contention", "clear", "key swap"} {
		t.Run(scenario, func(t *testing.T) {
			account := ollama429Account(995, PlatformOpenAI)
			account.RateLimitedAt = ollama429TimePtr(time.Now())
			account.RateLimitResetAt = ollama429TimePtr(time.Now().Add(5 * time.Second))
			repository := &reviewOllamaRepository{ollama429Repo: newOllama429Repo(account), conflicts: 10, clearOnConflict: scenario == "clear", swapOnConflict: scenario == "key swap"}
			scheduler := newOllama429SchedulerStub(true)
			service := NewRateLimitService(repository, nil, nil, nil, nil)
			service.SetOllamaCloudUsageProbeScheduler(scheduler)
			service.handle429(context.Background(), account, http.Header{}, nil)
			expected := 1
			if scenario == "contention" {
				expected = 3
			}
			require.Equal(t, expected, repository.recordCalls)
			require.Zero(t, scheduler.count())
		})
	}
}

func (repository *reviewOllamaRepository) SetRateLimitedIfUnchanged(ctx context.Context, id int64, version time.Time, limitedAt, resetAt *time.Time, next time.Time) (bool, error) {
	updated, err := repository.ollama429Repo.SetRateLimitedIfUnchanged(ctx, id, version, limitedAt, resetAt, next)
	if updated && repository.afterCAS != nil {
		repository.afterCAS()
	}
	return updated, err
}

func TestOllama429ClearBetweenRecordAndRereadDoesNotAdoptClearedGeneration(t *testing.T) {
	account := ollama429Account(991, PlatformOpenAI)
	repository := &reviewOllamaRepository{ollama429Repo: newOllama429Repo(account), clearBeforeReread: true}
	scheduler := newOllama429SchedulerStub(true)
	service := NewRateLimitService(repository, nil, nil, nil, nil)
	service.SetOllamaCloudUsageProbeScheduler(scheduler)
	service.handle429(context.Background(), account, http.Header{}, nil)
	require.Zero(t, scheduler.count(), "the stamped event must not adopt a cleared row during its reread")
	require.Nil(t, repository.currentReset(account.ID))
}

func TestOllama429BenignConcurrentEventConflictRetainsLongerFloor(t *testing.T) {
	account := ollama429Account(992, PlatformOpenAI)
	repository := &reviewOllamaRepository{ollama429Repo: newOllama429Repo(account), conflictOnce: true}
	scheduler := newOllama429SchedulerStub(true)
	service := NewRateLimitService(repository, nil, nil, nil, nil)
	service.SetOllamaCloudUsageProbeScheduler(scheduler)
	before := time.Now()
	service.handle429(context.Background(), account, http.Header{"Retry-After": {"60"}}, nil)
	reset := repository.currentReset(account.ID)
	require.NotNil(t, reset)
	require.False(t, reset.Before(before.Add(time.Minute)), "a competing shorter event must not discard this event's longer floor")
	require.Equal(t, 1, scheduler.count())
}

func TestOllamaProbeClearAfterCASDoesNotResurrectRuntimeBlock(t *testing.T) {
	account := ollama429Account(993, PlatformOpenAI)
	repository := &reviewOllamaRepository{ollama429Repo: newOllama429Repo(account)}
	scheduler := newOllama429SchedulerStub(true)
	gateway := &OpenAIGatewayService{}
	service := NewRateLimitService(repository, nil, nil, nil, nil)
	service.SetOllamaCloudUsageProbeScheduler(scheduler)
	service.SetAccountRuntimeBlocker(gateway)
	service.handle429(context.Background(), account, http.Header{}, nil)
	repository.afterCAS = func() {
		repository.mutate(account.ID, func(current *Account) { current.RateLimitedAt, current.RateLimitResetAt = nil, nil })
		gateway.ClearAccountSchedulingBlock(account.ID)
	}
	scheduler.fire(account.ID, time.Now().Add(time.Hour))
	_, blocked := gateway.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, blocked, "an administrative runtime clear must fence a delayed publisher")
	require.Nil(t, repository.currentReset(account.ID))
}

func TestOllamaRefreshJoiningFlightHonorsCallerCancellation(t *testing.T) {
	account := ollamaUsageAccount(994)
	account.Extra[OllamaCloudUsageSessionExtraKey] = "cipher:wos-session=fixture"
	repository := &ollamaUsageTestRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	service := newOllamaUsageTestService(t, repository, &ollamaUsageHTTPStub{}, &upstreamBillingProbeSettingRepo{}, true)
	key, valid := ollamaCloudUsageGroupFingerprint(account)
	require.True(t, valid)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	ownerDone := make(chan struct{})
	go func() {
		_, _, _ = service.refreshGroup.Do(key, func() (any, error) { close(started); <-release; return nil, nil })
		close(ownerDone)
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := service.refreshAccount(ctx, account.ID, nil, false); finished <- err }()
	require.Eventually(t, func() bool { return repository.getByIDCalls.Load() > 0 }, time.Second, time.Millisecond)
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		once.Do(func() { close(release) })
		<-ownerDone
		<-finished
		t.Fatal("cancelled caller remained blocked behind another flight owner")
	}
	once.Do(func() { close(release) })
	<-ownerDone
}

func TestOllamaProbeStopDoesNotWaitForAnotherFlightOwner(t *testing.T) {
	account := ollamaUsageAccount(996)
	account.Extra[OllamaCloudUsageSessionExtraKey] = "cipher:wos-session=fixture"
	repository := &ollamaUsageTestRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	service := ollamaCloudProbeFixture(t, repository, &ollamaUsageHTTPStub{}, time.Now())
	key, valid := ollamaCloudUsageGroupFingerprint(account)
	require.True(t, valid)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	ownerDone := make(chan struct{})
	go func() {
		_, _, _ = service.refreshGroup.Do(key, func() (any, error) { close(started); <-release; return nil, nil })
		close(ownerDone)
	}()
	<-started
	before := repository.getByIDCalls.Load()
	callbacks := make(chan struct{}, 1)
	require.True(t, service.ScheduleOllamaCloudUsageRateLimitProbe(account.ID, func(int64, time.Time) { callbacks <- struct{}{} }))
	require.Eventually(t, func() bool { return repository.getByIDCalls.Load() >= before+2 }, time.Second, time.Millisecond)
	stopped := make(chan struct{})
	go func() { service.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		once.Do(func() { close(release) })
		<-stopped
		<-ownerDone
		t.Fatal("probe shutdown waited for an unrelated flight owner")
	}
	require.Empty(t, callbacks)
	once.Do(func() { close(release) })
	<-ownerDone
}

type deadlineOllamaRepository struct {
	*ollamaUsageTestRepo
	deadline time.Time
}

func (repository *deadlineOllamaRepository) GetByID(ctx context.Context, id int64) (*Account, error) {
	repository.deadline, _ = ctx.Deadline()
	return repository.ollamaUsageTestRepo.GetByID(ctx, id)
}

func TestOllamaRefreshOwnerIsBoundedBeforeRepositoryWork(t *testing.T) {
	account := ollamaUsageAccount(997)
	repository := &deadlineOllamaRepository{ollamaUsageTestRepo: &ollamaUsageTestRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}}}
	service := NewOllamaCloudUsageService(repository, &ollamaUsageHTTPStub{}, nil, ollamaUsageTestEncryptor{}, true)
	defer service.Stop()
	before := time.Now()
	_, err := service.refreshAccount(context.Background(), account.ID, nil, false)
	require.ErrorIs(t, err, ErrOllamaCloudUsageSessionRequired)
	require.False(t, repository.deadline.IsZero())
	require.WithinDuration(t, before.Add(ollamaCloudUsageProbeTimeout), repository.deadline, time.Second)
}
