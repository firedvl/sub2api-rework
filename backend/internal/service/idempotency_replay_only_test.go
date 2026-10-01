package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type replayOnlyRepository struct {
	*inMemoryIdempotencyRepo
	claims int
}

func (repo *replayOnlyRepository) CreateProcessing(ctx context.Context, record *IdempotencyRecord) (bool, error) {
	repo.claims++
	return repo.inMemoryIdempotencyRepo.CreateProcessing(ctx, record)
}

func TestIdempotencyCoordinator_ReplayOnlyDoesNotCreateMissingOperation(t *testing.T) {
	repo := &replayOnlyRepository{inMemoryIdempotencyRepo: newInMemoryIdempotencyRepo()}
	coordinator := NewIdempotencyCoordinator(repo, DefaultIdempotencyConfig())
	executions := 0
	_, err := coordinator.Execute(context.Background(), IdempotencyExecuteOptions{
		Scope: "admin.bulk", Method: "POST", Route: "/bulk", IdempotencyKey: "uncertain-key",
		Payload: "original-request", ReplayOnly: true,
	}, func(context.Context) (any, error) {
		executions++
		return "mutated", nil
	})
	require.ErrorIs(t, err, ErrIdempotencyReplayUnavailable)
	require.Zero(t, executions)
	require.Zero(t, repo.claims)
	require.Empty(t, repo.data)
}

func TestIdempotencyCoordinator_ReplayOnlyDoesNotReexecuteExpiredOperation(t *testing.T) {
	repo := &replayOnlyRepository{inMemoryIdempotencyRepo: newInMemoryIdempotencyRepo()}
	coordinator := NewIdempotencyCoordinator(repo, DefaultIdempotencyConfig())
	opts := IdempotencyExecuteOptions{Scope: "admin.bulk", Method: "POST", Route: "/bulk", IdempotencyKey: "expired-key", Payload: "original-request"}
	executions := 0
	execute := func(context.Context) (any, error) {
		executions++
		return map[string]int{"success_count": 1}, nil
	}
	_, err := coordinator.Execute(context.Background(), opts, execute)
	require.NoError(t, err)
	opts.ReplayOnly = true
	replay, err := coordinator.Execute(context.Background(), opts, execute)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	for _, record := range repo.data {
		record.ExpiresAt = time.Now().Add(-time.Second)
	}
	_, err = coordinator.Execute(context.Background(), opts, execute)
	require.ErrorIs(t, err, ErrIdempotencyReplayUnavailable)
	require.Equal(t, 1, executions)
	require.Equal(t, 1, repo.claims)
	for _, record := range repo.data {
		require.Equal(t, IdempotencyStatusSucceeded, record.Status)
	}
}
