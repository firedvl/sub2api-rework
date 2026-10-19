//go:build integration

package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestUsageObservationMigrationPreservesHistoricalRecords(t *testing.T) {
	tx := testTx(t)
	_, err := tx.ExecContext(t.Context(), `CREATE TEMP TABLE usage_logs (id bigint PRIMARY KEY, input_tokens integer, actual_cost numeric(20,10)) ON COMMIT DROP`)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), `INSERT INTO usage_logs VALUES (1, 123, 0.0000043210)`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("252_usage_observation_completeness.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(t.Context(), string(migration))
		require.NoError(t, err)
	}
	var tokens int
	var cost string
	var complete sql.NullBool
	err = tx.QueryRowContext(t.Context(), `SELECT input_tokens, actual_cost::text, usage_observation_complete FROM pg_temp.usage_logs WHERE id=1`).Scan(&tokens, &cost, &complete)
	require.NoError(t, err)
	require.Equal(t, 123, tokens)
	require.Equal(t, "0.0000043210", cost)
	require.False(t, complete.Valid, "historical charges must not be marked complete")
	var nullable bool
	var defaultValue sql.NullString
	err = tx.QueryRowContext(t.Context(), `SELECT NOT a.attnotnull, pg_get_expr(d.adbin,d.adrelid) FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid='pg_temp.usage_logs'::regclass AND a.attname='usage_observation_complete'`).Scan(&nullable, &defaultValue)
	require.NoError(t, err)
	require.True(t, nullable)
	require.False(t, defaultValue.Valid, "future uninstrumented writes must remain unknown")
}
