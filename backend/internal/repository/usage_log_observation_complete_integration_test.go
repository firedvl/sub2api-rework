//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsageLogObservationCompleteRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUsageLogRepositoryWithSQL(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{Email: "observation-" + uuid.NewString() + "@example.com"})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id=$1", user.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE user_id=$1", user.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id=$1", user.ID)
		require.NoError(t, err)
	})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-observation-" + uuid.NewString(), Name: "observation"})
	account := mustCreateAccount(t, client, &service.Account{Name: "observation-" + uuid.NewString()})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE account_id=$1", account.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		require.NoError(t, err)
	})
	complete, incomplete := true, false
	values := []*bool{nil, &incomplete, &complete}
	for _, path := range []string{"single", "batch", "best_effort"} {
		t.Run(path, func(t *testing.T) {
			logs := make([]*service.UsageLog, len(values))
			errs := make([]error, len(values))
			var wg sync.WaitGroup
			for i, value := range values {
				logs[i] = &service.UsageLog{
					UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID,
					RequestID: uuid.NewString(), Model: "claude-sonnet-4",
					InputTokens: 10 + i, OutputTokens: 20 + i,
					TotalCost: 0.4, ActualCost: 0.3, RateMultiplier: 1,
					UsageObservationComplete: value,
				}
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					switch path {
					case "single":
						_, errs[i] = repo.createSingle(ctx, integrationDB, logs[i])
					case "batch":
						_, errs[i] = repo.Create(ctx, logs[i])
					case "best_effort":
						errs[i] = repo.CreateBestEffort(ctx, logs[i])
					}
				}(i)
			}
			wg.Wait()
			for i, log := range logs {
				require.NoError(t, errs[i])
				var id int64
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", log.RequestID, key.ID).Scan(&id))
				got, err := repo.GetByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, values[i], got.UsageObservationComplete, fmt.Sprintf("%s state %d", path, i))
				require.Equal(t, log.UserID, got.UserID)
				require.Equal(t, log.APIKeyID, got.APIKeyID)
				require.Equal(t, log.AccountID, got.AccountID)
				require.Equal(t, log.InputTokens, got.InputTokens)
				require.Equal(t, log.OutputTokens, got.OutputTokens)
				require.Equal(t, log.TotalCost, got.TotalCost)
				require.Equal(t, log.ActualCost, got.ActualCost)
				// Deduplication must preserve the original observation and cost.
				duplicate := *log
				duplicate.UsageObservationComplete = &complete
				duplicate.TotalCost = 999
				inserted, err := repo.Create(ctx, &duplicate)
				require.NoError(t, err)
				require.False(t, inserted)
				got, err = repo.GetByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, values[i], got.UsageObservationComplete)
				require.Equal(t, log.TotalCost, got.TotalCost)
			}
		})
	}
}
