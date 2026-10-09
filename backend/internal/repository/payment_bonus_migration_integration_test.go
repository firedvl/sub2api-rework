//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestPaymentBonusMigrationPreservesLegacyCredit(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	var precision, scale int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT numeric_precision, numeric_scale FROM information_schema.columns WHERE table_name='payment_orders' AND column_name='bonus_amount'`).Scan(&precision, &scale))
	require.Equal(t, 20, precision)
	require.Equal(t, 2, scale)
	// A disposable pre-promotion schema verifies default backfill and repeat application.
	db, err := sql.Open("postgres", integrationPostgresDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `CREATE TEMP TABLE payment_orders (id BIGINT PRIMARY KEY, amount DECIMAL(20,2), pay_amount DECIMAL(20,2)); INSERT INTO payment_orders VALUES (1, 14, 100)`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("251_add_payment_order_bonus_amount.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = conn.ExecContext(ctx, string(content))
		require.NoError(t, err)
	}
	var amount, pay, bonus float64
	require.NoError(t, conn.QueryRowContext(ctx, `SELECT amount, pay_amount, bonus_amount FROM payment_orders WHERE id=1`).Scan(&amount, &pay, &bonus))
	require.Equal(t, 14.0, amount)
	require.Equal(t, 100.0, pay)
	require.Zero(t, bonus)
}
