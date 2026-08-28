package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const openAIAutoWarmupResetJitter = time.Minute

type openAIAutoWarmupRepository struct {
	db *sql.DB
}

func NewOpenAIAutoWarmupAttemptRepository(db *sql.DB) service.OpenAIAutoWarmupAttemptRepository {
	return &openAIAutoWarmupRepository{db: db}
}

func (r *openAIAutoWarmupRepository) Claim(ctx context.Context, accountID int64, windowType string, resetAt time.Time) (*service.OpenAIAutoWarmupAttempt, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('openai_auto_warmup:' || $1::text, 0))", accountID); err != nil {
		return nil, false, err
	}
	var existingID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM openai_auto_warmup_attempts
		WHERE account_id = $1
		  AND window_type = $2
		  AND reset_at BETWEEN $3 AND $4
		LIMIT 1`,
		accountID, windowType, resetAt.Add(-openAIAutoWarmupResetJitter), resetAt.Add(openAIAutoWarmupResetJitter),
	).Scan(&existingID)
	if err == nil {
		if err = tx.Commit(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	attempt := &service.OpenAIAutoWarmupAttempt{
		AccountID: accountID, WindowType: windowType, ResetAt: resetAt.UTC(),
		Status: service.OpenAIAutoWarmupStatusPending,
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO openai_auto_warmup_attempts (account_id, window_type, reset_at)
		VALUES ($1, $2, $3)
		RETURNING id, attempted_at`, accountID, windowType, resetAt.UTC(),
	).Scan(&attempt.ID, &attempt.AttemptedAt)
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return attempt, true, nil
}

func (r *openAIAutoWarmupRepository) Complete(ctx context.Context, attemptID int64, completion service.OpenAIAutoWarmupCompletion) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE openai_auto_warmup_attempts
		SET status = $2,
		    completed_at = NOW(),
		    error_code = NULLIF($3, ''),
		    model = NULLIF($4, ''),
		    request_id = NULLIF($5, ''),
		    response_id = NULLIF($6, ''),
		    latency_ms = $7,
		    input_tokens = $8,
		    output_tokens = $9,
		    cache_creation_tokens = $10,
		    cache_read_tokens = $11
		WHERE id = $1 AND status = 'pending'`,
		attemptID, completion.Status, completion.ErrorCode, completion.Model,
		completion.RequestID, completion.ResponseID, completion.LatencyMS,
		completion.InputTokens, completion.OutputTokens,
		completion.CacheCreationTokens, completion.CacheReadTokens,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return sql.ErrNoRows
	}
	return nil
}
