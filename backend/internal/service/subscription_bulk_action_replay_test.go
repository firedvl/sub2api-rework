package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/require"
)

type rejectedBulkResultRepository struct {
	*inMemoryIdempotencyRepo
	savedInsideTransaction bool
}

func (repo *rejectedBulkResultRepository) MarkSucceeded(ctx context.Context, _ int64, _ int, _ string, _ time.Time) error {
	repo.savedInsideTransaction = dbent.TxFromContext(ctx) != nil
	return errors.New("aggregate replay storage failed")
}

func TestBulkSubscriptionAction_ReplayStorageFailureDoesNotCommitMutation(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = database.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, database)))
	expiresAt := time.Now().AddDate(0, 0, 30)
	subscriptions := &transactionalBulkSubscriptionRepo{
		committed: UserSubscription{ID: 1, UserID: 7, GroupID: 9, Status: SubscriptionStatusActive, ExpiresAt: expiresAt},
		pending:   make(map[*dbent.Tx]*UserSubscription), reads: make(map[*dbent.Tx]int),
	}
	subscriptionService := &SubscriptionService{userSubRepo: subscriptions, entClient: client}
	resultRepository := &rejectedBulkResultRepository{inMemoryIdempotencyRepo: newInMemoryIdempotencyRepo()}
	coordinator := NewIdempotencyCoordinator(resultRepository, DefaultIdempotencyConfig())
	mock.ExpectBegin()
	mock.ExpectExec("SAVEPOINT subscription_bulk_item").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT subscription_bulk_item").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	_, err = coordinator.Execute(context.Background(), IdempotencyExecuteOptions{
		Scope: "admin.subscriptions.bulk-action", Method: "POST", Route: "/bulk-action",
		IdempotencyKey: "storage-failure", Payload: "extend-seven-days", RequireKey: true, ExecutionTimeout: time.Second,
		RunInTransaction: subscriptionService.RunBulkSubscriptionTransaction,
	}, func(ctx context.Context) (any, error) {
		return subscriptionService.BulkSubscriptionAction(ctx, &BulkSubscriptionActionInput{
			SubscriptionIDs: []int64{1}, Action: "extend", Days: 7,
		})
	})
	require.ErrorIs(t, err, ErrIdempotencyStoreUnavail)
	require.Equal(t, expiresAt, subscriptions.committed.ExpiresAt, "a missing durable replay result must not leave a committed extension")
	require.True(t, resultRepository.savedInsideTransaction)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkSubscriptionAction_CachesChangeOnlyAfterConfirmedCommit(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(map[bool]string{true: "commit", false: "rollback"}[commit], func(t *testing.T) {
			database, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = database.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, database)))
			cache, err := ristretto.NewCache(&ristretto.Config{NumCounters: 1000, MaxCost: 100, BufferItems: 64})
			require.NoError(t, err)
			defer cache.Close()
			key := subCacheKey(7, 9)
			require.True(t, cache.Set(key, &UserSubscription{ID: 1}, 1))
			cache.Wait()
			mock.ExpectBegin()
			transaction, err := client.Tx(context.Background())
			require.NoError(t, err)
			subscriptions := &SubscriptionService{subCacheL1: cache}
			require.NoError(t, subscriptions.invalidateSubscriptionCachesAfterCommit(dbent.NewTxContext(context.Background(), transaction), 7, 9))
			_, cached := cache.Get(key)
			require.True(t, cached)
			if commit {
				mock.ExpectCommit()
				require.NoError(t, transaction.Commit())
			} else {
				mock.ExpectRollback()
				require.NoError(t, transaction.Rollback())
			}
			_, cached = cache.Get(key)
			require.Equal(t, !commit, cached)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
