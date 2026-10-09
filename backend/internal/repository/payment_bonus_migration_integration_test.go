//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestPaymentBonusUpgrade249Through251(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	files, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	stages := map[int]fstest.MapFS{249: {}, 250: {}, 251: {}}
	var has250 bool
	for _, name := range files {
		var number int
		_, err := fmt.Sscanf(name, "%d_", &number)
		require.NoError(t, err)
		if number == 250 {
			has250 = true
		}
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		for stage, subset := range stages {
			if number <= stage {
				subset[name] = &fstest.MapFile{Data: data}
			}
		}
	}
	require.True(t, has250, "upgrade qualification requires the actual integrated schema250 migration")
	dbName := fmt.Sprintf("sub2api_payment_upgrade_%d", time.Now().UnixNano())
	_, err = integrationDB.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err)
	var upgradeDB *sql.DB
	t.Cleanup(func() {
		if upgradeDB != nil {
			_ = upgradeDB.Close()
		}
		dropCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := integrationDB.ExecContext(dropCtx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		require.NoError(t, err)
	})
	dsn, err := url.Parse(integrationPostgresDSN)
	require.NoError(t, err)
	dsn.Path, dsn.RawPath = "/"+dbName, ""
	upgradeDB, err = openSQLWithRetry(ctx, dsn.String(), 30*time.Second)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[249]))
	_, err = upgradeDB.ExecContext(ctx, `INSERT INTO payment_orders (user_id, user_email, amount, pay_amount, status, expires_at, out_trade_no) VALUES (1, 'legacy@example.test', 14, 100, 'COMPLETED', NOW(), 'legacy-promotion-fixture')`)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[250]))
	var exists bool
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='payment_orders' AND column_name='bonus_amount')`).Scan(&exists))
	require.False(t, exists)
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[251]))
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[251]))
	var amount, pay, bonus float64
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT amount, pay_amount, bonus_amount FROM payment_orders WHERE out_trade_no='legacy-promotion-fixture'`).Scan(&amount, &pay, &bonus))
	require.Equal(t, 14.0, amount)
	require.Equal(t, 100.0, pay)
	require.Zero(t, bonus)
}

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
