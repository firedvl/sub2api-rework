//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoWarmupRepositoryClaimsOnceAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	var accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, status, schedulable, credentials, extra)
		VALUES ($1, 'openai', 'oauth', 'active', TRUE, '{}'::jsonb, '{}'::jsonb)
		RETURNING id`, "auto-warmup-race-"+time.Now().UTC().Format("20060102150405.000000000"),
	).Scan(&accountID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
	})

	resetAt := time.Now().UTC().Truncate(time.Second).Add(5 * time.Hour)
	repositories := []*openAIAutoWarmupRepository{{db: integrationDB}, {db: integrationDB}}
	start := make(chan struct{})
	type claimResult struct {
		attempt *service.OpenAIAutoWarmupAttempt
		claimed bool
		err     error
	}
	results := make(chan claimResult, len(repositories))
	var wg sync.WaitGroup
	for _, repo := range repositories {
		wg.Add(1)
		go func(repo *openAIAutoWarmupRepository) {
			defer wg.Done()
			<-start
			attempt, claimed, err := repo.Claim(ctx, accountID, "5h", resetAt)
			results <- claimResult{attempt: attempt, claimed: claimed, err: err}
		}(repo)
	}
	close(start)
	wg.Wait()
	close(results)

	claimed := 0
	var winning *service.OpenAIAutoWarmupAttempt
	for result := range results {
		require.NoError(t, result.err)
		if result.claimed {
			claimed++
			winning = result.attempt
		}
	}
	require.Equal(t, 1, claimed)
	require.NotNil(t, winning)

	_, jitterClaimed, err := repositories[0].Claim(ctx, accountID, "5h", resetAt.Add(30*time.Second))
	require.NoError(t, err)
	require.False(t, jitterClaimed)
	_, newWindowClaimed, err := repositories[1].Claim(ctx, accountID, "5h", resetAt.Add(5*time.Hour))
	require.NoError(t, err)
	require.True(t, newWindowClaimed)

	require.NoError(t, repositories[0].Complete(ctx, winning.ID, service.OpenAIAutoWarmupCompletion{
		Status: service.OpenAIAutoWarmupStatusSucceeded, Model: "gpt-test", RequestID: "req-test",
		InputTokens: 5, OutputTokens: 1,
	}))
	var status, source, requestKind string
	var inputTokens, outputTokens int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, source, request_kind, input_tokens, output_tokens
		FROM openai_auto_warmup_attempts WHERE id = $1`, winning.ID,
	).Scan(&status, &source, &requestKind, &inputTokens, &outputTokens))
	require.Equal(t, service.OpenAIAutoWarmupStatusSucceeded, status)
	require.Equal(t, "auto_warmup", source)
	require.Equal(t, "warmup", requestKind)
	require.Equal(t, 5, inputTokens)
	require.Equal(t, 1, outputTokens)
}
