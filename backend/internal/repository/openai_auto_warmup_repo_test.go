package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoWarmupRepositoryClaim(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("existing jittered reset blocks the claim", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		mock.ExpectBegin()
		mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("(?s)SELECT id.*reset_at BETWEEN \\$3 AND \\$4").
			WithArgs(int64(42), "5h", now.Add(-time.Minute), now.Add(time.Minute)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
		mock.ExpectCommit()

		attempt, claimed, err := (&openAIAutoWarmupRepository{db: db}).Claim(context.Background(), 42, "5h", now)
		require.NoError(t, err)
		require.False(t, claimed)
		require.Nil(t, attempt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("new reset is claimed before sender IO", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		mock.ExpectBegin()
		mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("(?s)SELECT id.*reset_at BETWEEN \\$3 AND \\$4").
			WithArgs(int64(42), "5h", now.Add(-time.Minute), now.Add(time.Minute)).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery("(?s)INSERT INTO openai_auto_warmup_attempts.*RETURNING id, attempted_at").
			WithArgs(int64(42), "5h", now).
			WillReturnRows(sqlmock.NewRows([]string{"id", "attempted_at"}).AddRow(8, now))
		mock.ExpectCommit()

		attempt, claimed, err := (&openAIAutoWarmupRepository{db: db}).Claim(context.Background(), 42, "5h", now)
		require.NoError(t, err)
		require.True(t, claimed)
		require.Equal(t, int64(8), attempt.ID)
		require.Equal(t, service.OpenAIAutoWarmupStatusPending, attempt.Status)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOpenAIAutoWarmupRepositoryCompletePersistsOutcomeAndUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	completion := service.OpenAIAutoWarmupCompletion{
		Status: service.OpenAIAutoWarmupStatusSucceeded, Model: "gpt-test",
		RequestID: "req-test", ResponseID: "resp-test", LatencyMS: 12,
		InputTokens: 5, OutputTokens: 1, CacheCreationTokens: 2, CacheReadTokens: 3,
	}
	mock.ExpectExec("(?s)UPDATE openai_auto_warmup_attempts.*WHERE id = \\$1 AND status = 'pending'").
		WithArgs(int64(9), completion.Status, "", completion.Model, completion.RequestID, completion.ResponseID,
			completion.LatencyMS, completion.InputTokens, completion.OutputTokens, completion.CacheCreationTokens, completion.CacheReadTokens).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, (&openAIAutoWarmupRepository{db: db}).Complete(context.Background(), 9, completion))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIAutoWarmupMigrationContract(t *testing.T) {
	content, err := dbmigrations.FS.ReadFile("232_openai_auto_warmup_attempts.sql")
	require.NoError(t, err)
	sqlText := string(content)
	for _, required := range []string{
		"REFERENCES accounts(id) ON DELETE CASCADE",
		"UNIQUE (account_id, window_type, reset_at)",
		"CHECK (status IN ('pending', 'succeeded', 'failed'))",
		"source VARCHAR(32) NOT NULL DEFAULT 'auto_warmup'",
		"request_kind VARCHAR(32) NOT NULL DEFAULT 'warmup'",
	} {
		require.Contains(t, sqlText, required)
	}
}
