//go:build integration

package repository

import (
	"context"
	"fmt"
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

func TestOpenAIAutoWarmupRepositoryDormantClaimIgnoresSlidingResetDrift(t *testing.T) {
	ctx := context.Background()
	var accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, status, schedulable, credentials, extra)
		VALUES ($1, 'openai', 'oauth', 'active', TRUE, '{}'::jsonb, '{}'::jsonb)
		RETURNING id`, "auto-warmup-dormant-race-"+time.Now().UTC().Format("20060102150405.000000000"),
	).Scan(&accountID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
	})

	repositories := []*openAIAutoWarmupRepository{{db: integrationDB}, {db: integrationDB}}
	start := make(chan struct{})
	type dormantClaimResult struct {
		claimed bool
		err     error
	}
	results := make(chan dormantClaimResult, len(repositories))
	var wg sync.WaitGroup
	baseReset := time.Now().UTC().Truncate(time.Second).Add(5 * time.Hour)
	for index, repo := range repositories {
		wg.Add(1)
		go func(repo *openAIAutoWarmupRepository, resetAt time.Time) {
			defer wg.Done()
			<-start
			_, claimed, err := repo.ClaimDormant(ctx, accountID, "5h", resetAt, 5*time.Hour)
			results <- dormantClaimResult{claimed: claimed, err: err}
		}(repo, baseReset.Add(time.Duration(index)*time.Minute))
	}
	close(start)
	wg.Wait()
	close(results)

	claimed := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.claimed {
			claimed++
		}
	}
	require.Equal(t, 1, claimed)
	_, claimedAgain, err := repositories[0].ClaimDormant(ctx, accountID, "5h", baseReset.Add(2*time.Minute), 5*time.Hour)
	require.NoError(t, err)
	require.False(t, claimedAgain)
}

func TestOpenAIAutoWarmupRepositoryRetriesOnlyKnownPreflightFailures(t *testing.T) {
	ctx := context.Background()
	var accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, status, schedulable, credentials, extra)
		VALUES ('warmup-preflight-retry', 'openai', 'oauth', 'active', TRUE, '{}', '{}') RETURNING id`).Scan(&accountID))
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", accountID) })
	repo := &openAIAutoWarmupRepository{db: integrationDB}
	resetAt := time.Now().UTC().Truncate(time.Second).Add(5 * time.Hour)
	attempt, claimed, err := repo.Claim(ctx, accountID, "5h", resetAt)
	require.NoError(t, err)
	require.True(t, claimed)
	for _, status := range []string{"pending", "succeeded", "failed"} {
		_, err := integrationDB.ExecContext(ctx, `UPDATE openai_auto_warmup_attempts SET status=$2,
		 attempted_at=NOW()-INTERVAL '11 minutes', error_code=NULL WHERE id=$1`, attempt.ID, status)
		require.NoError(t, err)
		_, claimed, err := repo.Claim(ctx, accountID, "5h", resetAt)
		require.NoError(t, err)
		require.False(t, claimed)
	}
	for _, code := range []string{"OPENAI_AUTO_WARMUP_UPSTREAM_FAILED", "OPENAI_AUTO_WARMUP_RESPONSE_FAILED", "OPENAI_AUTO_WARMUP_AUTH_FAILED"} {
		_, err := integrationDB.ExecContext(ctx, `UPDATE openai_auto_warmup_attempts SET error_code=$2 WHERE id=$1`, attempt.ID, code)
		require.NoError(t, err)
		_, claimed, err := repo.ClaimDormant(ctx, accountID, "5h", resetAt.Add(time.Minute), 5*time.Hour)
		require.NoError(t, err)
		require.False(t, claimed)
	}
	for _, dormant := range []bool{false, true} {
		_, err := integrationDB.ExecContext(ctx, `UPDATE openai_auto_warmup_attempts SET status='failed',
		 attempted_at=NOW(), error_code='OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED' WHERE id=$1`, attempt.ID)
		require.NoError(t, err)
		_, claimed, err := repo.Claim(ctx, accountID, "5h", resetAt)
		require.NoError(t, err)
		require.False(t, claimed)
		_, err = integrationDB.ExecContext(ctx, `UPDATE openai_auto_warmup_attempts SET attempted_at=NOW()-INTERVAL '11 minutes' WHERE id=$1`, attempt.ID)
		require.NoError(t, err)
		results := make(chan bool, 8)
		errors := make(chan error, 8)
		for range 8 {
			go func() {
				restarted := &openAIAutoWarmupRepository{db: integrationDB}
				var got *service.OpenAIAutoWarmupAttempt
				var claimed bool
				var err error
				if dormant {
					got, claimed, err = restarted.ClaimDormant(ctx, accountID, "5h", resetAt.Add(time.Minute), 5*time.Hour)
				} else {
					got, claimed, err = restarted.Claim(ctx, accountID, "5h", resetAt)
				}
				if got != nil && got.ID != attempt.ID {
					errors <- fmt.Errorf("claim changed row ID")
					results <- false
					return
				}
				errors <- err
				results <- claimed
			}()
		}
		winners := 0
		for range 8 {
			require.NoError(t, <-errors)
			if <-results {
				winners++
			}
		}
		require.Equal(t, 1, winners)
	}
}
