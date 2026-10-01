//go:build integration

package repository

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountRepositorySetRateLimitedIfUnchanged(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	makeAcct := func(name string) *service.Account {
		return mustCreateAccount(t, tx.Client(), &service.Account{
			Name: name, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Credentials: map[string]any{"base_url": "https://www.ollama.com", "api_key": name},
		})
	}

	acct := makeAcct("cas-match")
	base := time.Now().Add(5 * time.Second)
	require.NoError(t, repo.SetRateLimited(ctx, acct.ID, base))

	cur, err := repo.GetByID(ctx, acct.ID)
	require.NoError(t, err)
	require.NotNil(t, cur)
	require.NotNil(t, cur.RateLimitedAt)
	require.NotNil(t, cur.RateLimitResetAt)
	expUpdated, expLimited, expReset := cur.UpdatedAt, cur.RateLimitedAt, cur.RateLimitResetAt

	newReset := time.Now().Add(2 * time.Hour)
	updated, err := repo.SetRateLimitedIfUnchanged(ctx, acct.ID, expUpdated, expLimited, expReset, newReset)
	require.NoError(t, err)
	require.True(t, updated, "a write matching the observed generation must apply")

	after, err := repo.GetByID(ctx, acct.ID)
	require.NoError(t, err)
	require.NotNil(t, after.RateLimitResetAt)
	require.WithinDuration(t, newReset, *after.RateLimitResetAt, 2*time.Second)

	stale := time.Now().Add(3 * time.Hour)
	updated, err = repo.SetRateLimitedIfUnchanged(ctx, acct.ID, expUpdated, expLimited, expReset, stale)
	require.NoError(t, err)
	require.False(t, updated, "a write whose row version moved on must not apply")

	updated, err = repo.SetRateLimitedIfUnchanged(ctx, acct.ID, after.UpdatedAt, expLimited, expReset, stale)
	require.NoError(t, err)
	require.False(t, updated, "re-armed limited/reset generation must not accept the old one even with the current row version")

	clearedReset := time.Now().Add(30 * time.Minute)
	updated, err = repo.SetRateLimitedIfUnchanged(ctx, acct.ID, after.UpdatedAt, nil, nil, clearedReset)
	require.NoError(t, err)
	require.False(t, updated, "nil-limited expectation must not match a set generation")

	updated, err = repo.SetRateLimitedIfUnchanged(ctx, acct.ID, expUpdated, nil, nil, clearedReset)
	require.NoError(t, err)
	require.False(t, updated, "stale row version must not apply even with nil limited/reset")

	fresh := makeAcct("cas-fresh")
	freshCur, err := repo.GetByID(ctx, fresh.ID)
	require.NoError(t, err)
	require.Nil(t, freshCur.RateLimitedAt)
	require.Nil(t, freshCur.RateLimitResetAt)
	firstReset := time.Now().Add(time.Hour)
	updated, err = repo.SetRateLimitedIfUnchanged(ctx, fresh.ID, freshCur.UpdatedAt, nil, nil, firstReset)
	require.NoError(t, err)
	require.True(t, updated, "nil generation CAS must apply to a never-limited account")

	require.NoError(t, repo.ClearRateLimit(ctx, fresh.ID))
	afterClear, err := repo.GetByID(ctx, fresh.ID)
	require.NoError(t, err)
	updated, err = repo.SetRateLimitedIfUnchanged(ctx, fresh.ID, afterClear.UpdatedAt, nil, nil, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.True(t, updated, "cleared account with nil generation accepts a fresh write")
}

func TestRecordOllama429PreservesFloorAndSupersedesEveryGeneration(t *testing.T) {
	ctx := context.Background()
	transaction := testEntTx(t)
	repository := newAccountRepositoryWithSQL(transaction.Client(), transaction, nil)
	account := mustCreateAccount(t, transaction.Client(), &service.Account{
		Name: "ollama-generation-fixture", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://ollama.com", "api_key": "fixture"},
	})
	observed, err := repository.GetByID(ctx, account.ID)
	require.NoError(t, err)
	updated, err := repository.RecordOllamaCloudUsage429(ctx, observed, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	first, err := repository.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, first.RateLimitedAt)
	require.Nil(t, first.RateLimitResetAt, "a disabled fallback must not invent a recovery deadline")
	updated, err = repository.RecordOllamaCloudUsage429(ctx, observed, nil)
	require.NoError(t, err)
	require.Nil(t, updated, "a stale trigger must not replace the row version")
	longFloor := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	updated, err = repository.RecordOllamaCloudUsage429(ctx, first, &longFloor)
	require.NoError(t, err)
	require.NotNil(t, updated)
	long, err := repository.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, long.RateLimitedAt.After(*first.RateLimitedAt))
	shortFloor := time.Now().Add(time.Minute)
	updated, err = repository.RecordOllamaCloudUsage429(ctx, long, &shortFloor)
	require.NoError(t, err)
	require.NotNil(t, updated)
	latest, err := repository.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, longFloor, *latest.RateLimitResetAt)
	require.True(t, latest.RateLimitedAt.After(*long.RateLimitedAt))
	applied, err := repository.SetRateLimitedIfUnchanged(ctx, latest.ID, latest.UpdatedAt, long.RateLimitedAt, long.RateLimitResetAt, longFloor.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, applied, "the old event must remain stale even when its deadline floor did not move")
	require.NoError(t, repository.ClearRateLimit(ctx, latest.ID))
	cleared, err := repository.GetByID(ctx, latest.ID)
	require.NoError(t, err)
	clearGeneration, ok := cleared.Extra[service.OllamaRateLimitClearGenerationExtraKey].(string)
	require.True(t, ok)
	require.NotEmpty(t, clearGeneration)
	require.NoError(t, repository.ClearRateLimit(ctx, latest.ID))
	clearedAgain, err := repository.GetByID(ctx, latest.ID)
	require.NoError(t, err)
	require.NotEqual(t, clearGeneration, clearedAgain.Extra[service.OllamaRateLimitClearGenerationExtraKey])
	applied, err = repository.SetRateLimitedIfUnchanged(ctx, latest.ID, cleared.UpdatedAt, first.RateLimitedAt, nil, longFloor)
	require.NoError(t, err)
	require.False(t, applied, "an administratively cleared unknown-deadline generation must stay cleared")
}

type concurrentOllamaTriggerRepository struct {
	service.AccountRepository
	store   *accountRepository
	reads   atomic.Int64
	arrived sync.WaitGroup
}

func (repository *concurrentOllamaTriggerRepository) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	account, err := repository.store.GetByID(ctx, id)
	if repository.reads.Add(1) <= 2 {
		repository.arrived.Done()
		repository.arrived.Wait()
	}
	return account, err
}

func (repository *concurrentOllamaTriggerRepository) RecordOllamaCloudUsage429(ctx context.Context, observed *service.Account, resetAt *time.Time) (*service.AccountRateLimitGeneration, error) {
	return repository.store.RecordOllamaCloudUsage429(ctx, observed, resetAt)
}

func (repository *concurrentOllamaTriggerRepository) SetRateLimitedIfUnchanged(ctx context.Context, id int64, version time.Time, limitedAt, resetAt *time.Time, next time.Time) (bool, error) {
	return repository.store.SetRateLimitedIfUnchanged(ctx, id, version, limitedAt, resetAt, next)
}

func TestConcurrentOllama429RetainsMaximumFloorWithSharedObservedVersion(t *testing.T) {
	ctx := context.Background()
	store := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	account := mustCreateAccount(t, integrationEntClient, &service.Account{
		Name: "ollama-concurrent-floor", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"base_url": "https://ollama.com", "api_key": "fixture"},
	})
	t.Cleanup(func() {
		require.NoError(t, integrationEntClient.Account.DeleteOneID(account.ID).Exec(mixins.SkipSoftDelete(context.Background())))
	})
	caller, err := store.GetByID(ctx, account.ID)
	require.NoError(t, err)
	repository := &concurrentOllamaTriggerRepository{AccountRepository: store, store: store}
	repository.arrived.Add(2)
	limiter := service.NewRateLimitService(repository, nil, nil, nil, nil)
	var finished sync.WaitGroup
	finished.Add(2)
	before := time.Now()
	for _, duration := range []string{"5", "60"} {
		go func(seconds string) {
			defer finished.Done()
			limiter.HandleUpstreamError(ctx, caller, http.StatusTooManyRequests, http.Header{"Retry-After": {seconds}}, nil)
		}(duration)
	}
	finished.Wait()
	latest, err := store.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, latest.RateLimitResetAt)
	require.False(t, latest.RateLimitResetAt.Before(before.Add(time.Minute)))
}
