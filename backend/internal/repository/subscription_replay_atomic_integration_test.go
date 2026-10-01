//go:build integration

package repository

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func subscriptionReplayFixture(t *testing.T, count int) ([]int64, time.Time, string) {
	t.Helper()
	ctx := context.Background()
	group, err := integrationEntClient.Group.Create().SetName(hashedTestValue(t, "replay-group")).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, integrationEntClient.Group.DeleteOneID(group.ID).Exec(mixins.SkipSoftDelete(context.Background())))
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiry := now.AddDate(0, 0, 30)
	ids := make([]int64, 0, count)
	for index := 0; index < count; index++ {
		user, err := integrationEntClient.User.Create().SetEmail(hashedTestValue(t, "replay-user-"+strconv.Itoa(index)) + "@example.test").
			SetPasswordHash("fixture-hash").SetRole(service.RoleUser).SetStatus(service.StatusActive).Save(ctx)
		require.NoError(t, err)
		subscription, err := integrationEntClient.UserSubscription.Create().SetUserID(user.ID).SetGroupID(group.ID).
			SetStartsAt(now).SetExpiresAt(expiry).SetStatus(service.SubscriptionStatusActive).SetAssignedAt(now).
			SetDailyUsageUsd(7).SetWeeklyUsageUsd(11).Save(ctx)
		require.NoError(t, err)
		ids = append(ids, subscription.ID)
		t.Cleanup(func() {
			cleanupCtx := mixins.SkipSoftDelete(context.Background())
			require.NoError(t, integrationEntClient.UserSubscription.DeleteOneID(subscription.ID).Exec(cleanupCtx))
			require.NoError(t, integrationEntClient.User.DeleteOneID(user.ID).Exec(cleanupCtx))
		})
	}
	scope := hashedTestValue(t, "subscription-replay")
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM idempotency_records WHERE scope = $1", scope)
		require.NoError(t, err)
	})
	return ids, expiry, scope
}

type failedSubscriptionReplayStore struct {
	service.IdempotencyRepository
	fail bool
}

func (repo *failedSubscriptionReplayStore) MarkSucceeded(ctx context.Context, id int64, status int, body string, expiry time.Time) error {
	if repo.fail {
		return errors.New("aggregate response persistence unavailable")
	}
	return repo.IdempotencyRepository.MarkSucceeded(ctx, id, status, body, expiry)
}

func TestSubscriptionReplayAtomic_StoreFailureRollsBackAndLeaseRecoveryDoesNotDuplicate(t *testing.T) {
	for _, action := range []string{"extend", "reset_quota"} {
		t.Run(action, func(t *testing.T) {
			ids, expiry, scope := subscriptionReplayFixture(t, 1)
			subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
			subscriptions := service.NewSubscriptionService(nil, subscriptionRepo, nil, integrationEntClient, nil)
			t.Cleanup(subscriptions.Stop)
			store := &failedSubscriptionReplayStore{IdempotencyRepository: NewIdempotencyRepository(integrationEntClient, integrationDB), fail: true}
			coordinator := service.NewIdempotencyCoordinator(store, service.DefaultIdempotencyConfig())
			input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: action, Days: 7, Daily: true}
			opts := service.IdempotencyExecuteOptions{Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "original-operation",
				Payload: input, RequireKey: true, ExecutionTimeout: 5 * time.Second, RunInTransaction: subscriptions.RunBulkSubscriptionTransaction}
			executions := 0
			execute := func(ctx context.Context) (any, error) {
				executions++
				return subscriptions.BulkSubscriptionAction(ctx, input)
			}
			_, err := coordinator.Execute(context.Background(), opts, execute)
			require.ErrorIs(t, err, service.ErrIdempotencyStoreUnavail)
			unchanged, err := subscriptionRepo.GetByID(context.Background(), ids[0])
			require.NoError(t, err)
			require.Equal(t, expiry, unchanged.ExpiresAt)
			require.Equal(t, float64(7), unchanged.DailyUsageUSD)
			store.fail = false
			_, err = integrationDB.ExecContext(context.Background(), "UPDATE idempotency_records SET locked_until = NOW() - INTERVAL '1 second' WHERE scope = $1", scope)
			require.NoError(t, err)
			opts.ReplayOnly = true
			result, err := coordinator.Execute(context.Background(), opts, execute)
			require.NoError(t, err)
			require.False(t, result.Replayed)
			changed, err := subscriptionRepo.GetByID(context.Background(), ids[0])
			require.NoError(t, err)
			if action == "extend" {
				require.Equal(t, expiry.AddDate(0, 0, 7), changed.ExpiresAt)
			} else {
				require.Zero(t, changed.DailyUsageUSD)
				require.Equal(t, float64(11), changed.WeeklyUsageUSD)
			}
			replay, err := coordinator.Execute(context.Background(), opts, execute)
			require.NoError(t, err)
			require.True(t, replay.Replayed)
			require.Equal(t, 2, executions)
		})
	}
}

type abortedBulkItemRepository struct {
	service.UserSubscriptionRepository
	rejectedID int64
}

func (repo *abortedBulkItemRepository) GetByID(ctx context.Context, id int64) (*service.UserSubscription, error) {
	if id == repo.rejectedID {
		executor, supported := dbent.TxFromContext(ctx).Client().Driver().(sqlExecutor)
		if !supported {
			return nil, errors.New("fixture transaction driver does not support SQL execution")
		}
		_, err := executor.ExecContext(ctx, "SELECT nonexistent_replay_fixture_column FROM user_subscriptions")
		return nil, err
	}
	return repo.UserSubscriptionRepository.GetByID(ctx, id)
}

func TestSubscriptionReplayAtomic_SQLAbortedItemRollsBackToSavepointAndContinues(t *testing.T) {
	ids, expiry, scope := subscriptionReplayFixture(t, 3)
	subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
	subscriptions := service.NewSubscriptionService(nil, &abortedBulkItemRepository{UserSubscriptionRepository: subscriptionRepo, rejectedID: ids[1]}, nil, integrationEntClient, nil)
	t.Cleanup(subscriptions.Stop)
	coordinator := service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), service.DefaultIdempotencyConfig())
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: "extend", Days: 7}
	result, err := coordinator.Execute(context.Background(), service.IdempotencyExecuteOptions{
		Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "partial-operation", Payload: input,
		RequireKey: true, ExecutionTimeout: 5 * time.Second, RunInTransaction: subscriptions.RunBulkSubscriptionTransaction,
	}, func(ctx context.Context) (any, error) { return subscriptions.BulkSubscriptionAction(ctx, input) })
	require.NoError(t, err)
	outcome := result.Data.(*service.BulkSubscriptionActionResult)
	require.Equal(t, 2, outcome.SuccessCount)
	require.Equal(t, 1, outcome.FailedCount)
	for index, id := range ids {
		subscription, err := subscriptionRepo.GetByID(context.Background(), id)
		require.NoError(t, err)
		expected := expiry.AddDate(0, 0, 7)
		if index == 1 {
			expected = expiry
		}
		require.Equal(t, expected, subscription.ExpiresAt)
	}
}

func TestSubscriptionReplayAtomic_LostCommitAcknowledgmentReplaysCommittedOutcome(t *testing.T) {
	ids, expiry, scope := subscriptionReplayFixture(t, 1)
	subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
	subscriptions := service.NewSubscriptionService(nil, subscriptionRepo, nil, integrationEntClient, nil)
	t.Cleanup(subscriptions.Stop)
	coordinator := service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), service.DefaultIdempotencyConfig())
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: "extend", Days: 7}
	opts := service.IdempotencyExecuteOptions{Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "uncertain-commit",
		Payload: input, RequireKey: true, ExecutionTimeout: 5 * time.Second,
		RunInTransaction: func(ctx context.Context, execute func(context.Context) error) error {
			if err := subscriptions.RunBulkSubscriptionTransaction(ctx, execute); err != nil {
				return err
			}
			return errors.New("commit acknowledgment lost")
		}}
	executions := 0
	execute := func(ctx context.Context) (any, error) {
		executions++
		return subscriptions.BulkSubscriptionAction(ctx, input)
	}
	_, err := coordinator.Execute(context.Background(), opts, execute)
	require.Error(t, err)
	opts.ReplayOnly = true
	replay, err := coordinator.Execute(context.Background(), opts, execute)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, 1, executions)
	subscription, err := subscriptionRepo.GetByID(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, expiry.AddDate(0, 0, 7), subscription.ExpiresAt)
}

func TestSubscriptionReplayAtomic_UnreplayableOversizedResponseRollsBack(t *testing.T) {
	ids, expiry, scope := subscriptionReplayFixture(t, 1)
	subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
	subscriptions := service.NewSubscriptionService(nil, subscriptionRepo, nil, integrationEntClient, nil)
	t.Cleanup(subscriptions.Stop)
	config := service.DefaultIdempotencyConfig()
	config.MaxStoredResponseLen = 20
	coordinator := service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), config)
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: "extend", Days: 7}
	_, err := coordinator.Execute(context.Background(), service.IdempotencyExecuteOptions{
		Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "oversized-response", Payload: input,
		RequireKey: true, ExecutionTimeout: 5 * time.Second, RunInTransaction: subscriptions.RunBulkSubscriptionTransaction,
	}, func(ctx context.Context) (any, error) { return subscriptions.BulkSubscriptionAction(ctx, input) })
	require.ErrorIs(t, err, service.ErrIdempotencyStoreUnavail)
	subscription, err := subscriptionRepo.GetByID(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, expiry, subscription.ExpiresAt)
}

type expiredBulkItemRepository struct {
	service.UserSubscriptionRepository
	expiredID int64
}

func (repo *expiredBulkItemRepository) GetByIDForUpdate(ctx context.Context, id int64) (*service.UserSubscription, error) {
	if id == repo.expiredID {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return repo.UserSubscriptionRepository.GetByIDForUpdate(ctx, id)
}

func TestSubscriptionReplayAtomic_ItemDeadlinePreservesEarlierSuccessAndDurableResult(t *testing.T) {
	ids, expiry, scope := subscriptionReplayFixture(t, 2)
	subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
	subscriptions := service.NewSubscriptionService(nil, &expiredBulkItemRepository{UserSubscriptionRepository: subscriptionRepo, expiredID: ids[1]}, nil, integrationEntClient, nil)
	t.Cleanup(subscriptions.Stop)
	coordinator := service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), service.DefaultIdempotencyConfig())
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: "extend", Days: 7}
	opts := service.IdempotencyExecuteOptions{Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "item-deadline", Payload: input,
		RequireKey: true, ExecutionTimeout: time.Second, RunInTransaction: subscriptions.RunBulkSubscriptionTransaction}
	executions := 0
	execute := func(ctx context.Context) (any, error) {
		executions++
		return subscriptions.BulkSubscriptionAction(ctx, input)
	}
	result, err := coordinator.Execute(context.Background(), opts, execute)
	require.NoError(t, err)
	outcome := result.Data.(*service.BulkSubscriptionActionResult)
	require.Equal(t, 1, outcome.SuccessCount)
	require.Equal(t, 1, outcome.FailedCount)
	for index, id := range ids {
		subscription, err := subscriptionRepo.GetByID(context.Background(), id)
		require.NoError(t, err)
		expected := expiry
		if index == 0 {
			expected = expiry.AddDate(0, 0, 7)
		}
		require.Equal(t, expected, subscription.ExpiresAt)
	}
	opts.ReplayOnly = true
	replay, err := coordinator.Execute(context.Background(), opts, execute)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, 1, executions)
}

func TestSubscriptionReplayAtomic_ExpiredLeaseCannotOvertakeLiveTransaction(t *testing.T) {
	ids, expiry, scope := subscriptionReplayFixture(t, 1)
	subscriptionRepo := NewUserSubscriptionRepository(integrationEntClient)
	subscriptions := service.NewSubscriptionService(nil, subscriptionRepo, nil, integrationEntClient, nil)
	t.Cleanup(subscriptions.Stop)
	config := service.DefaultIdempotencyConfig()
	config.ProcessingTimeout = time.Second
	coordinator := service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), config)
	input := &service.BulkSubscriptionActionInput{SubscriptionIDs: ids, Action: "extend", Days: 7}
	opts := service.IdempotencyExecuteOptions{Scope: scope, Method: "POST", Route: "/bulk-action", IdempotencyKey: "competing-owner",
		Payload: input, RequireKey: true, ExecutionTimeout: 5 * time.Second, RunInTransaction: subscriptions.RunBulkSubscriptionTransaction}
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	ownerDone, retryDone := make(chan error, 1), make(chan error, 1)
	var executions atomic.Int32
	execute := func(ctx context.Context) (any, error) {
		executions.Add(1)
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return subscriptions.BulkSubscriptionAction(ctx, input)
	}
	go func() { _, err := coordinator.Execute(context.Background(), opts, execute); ownerDone <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("transaction owner did not start")
	}
	var deadline time.Time
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT locked_until FROM idempotency_records WHERE scope = $1", scope).Scan(&deadline))
	time.Sleep(time.Until(deadline) + 10*time.Millisecond)
	retryOptions := opts
	retryOptions.ReplayOnly = true
	go func() { _, err := coordinator.Execute(context.Background(), retryOptions, execute); retryDone <- err }()
	require.Eventually(t, func() bool {
		var waiting int
		err := integrationDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE '%UPDATE idempotency_records%' AND pid <> pg_backend_pid()").Scan(&waiting)
		return err == nil && waiting > 0
	}, 2*time.Second, 20*time.Millisecond)
	require.EqualValues(t, 1, executions.Load())
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, <-ownerDone)
	require.ErrorIs(t, <-retryDone, service.ErrIdempotencyInProgress)
	replay, err := coordinator.Execute(context.Background(), retryOptions, execute)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.EqualValues(t, 1, executions.Load())
	subscription, err := subscriptionRepo.GetByID(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, expiry.AddDate(0, 0, 7), subscription.ExpiresAt)
}
