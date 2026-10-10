//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
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
		_, err := fmt.Sscanf(name, "%d", &number)
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
	_, err = upgradeDB.ExecContext(ctx, `INSERT INTO payment_orders (user_id, user_email, amount, pay_amount, status, expires_at, out_trade_no, refund_amount) VALUES
		(1, 'legacy@example.test', 14, 100, 'COMPLETED', NOW(), 'legacy-promotion-fixture', 0),
		(1, 'pending@example.test', 0.01, 0.01, 'PENDING', NOW(), 'legacy-pending-fixture', 0),
		(1, 'refunded@example.test', 14, 100, 'REFUNDED', NOW(), 'legacy-refund-fixture', 7),
		(1, 'maximum@example.test', 999999999999999999.99, 999999999999999999.99, 'COMPLETED', NOW(), 'legacy-maximum-fixture', 0)`)
	require.NoError(t, err)
	var before string
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM payment_orders p`).Scan(&before))
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[250]))
	var exists bool
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='payment_orders' AND column_name='bonus_amount')`).Scan(&exists))
	require.False(t, exists)
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[251]))
	require.NoError(t, applyMigrationsFS(ctx, upgradeDB, stages[251]))
	var preserved bool
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT jsonb_agg(to_jsonb(p) - 'bonus_amount' ORDER BY id) = $1::jsonb FROM payment_orders p`, before).Scan(&preserved))
	require.True(t, preserved, "every legacy field and exact numeric value must survive the upgrade")
	var backfilled, applied int
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT count(*) FROM payment_orders WHERE bonus_amount=0`).Scan(&backfilled))
	require.Equal(t, 4, backfilled)
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied))
	require.Equal(t, len(stages[251]), applied)
	for name, file := range stages[251] {
		var checksum string
		require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE filename=$1`, name).Scan(&checksum))
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(file.Data))))), checksum, name)
	}
	var amount, pay, bonus float64
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT amount, pay_amount, bonus_amount FROM payment_orders WHERE out_trade_no='legacy-promotion-fixture'`).Scan(&amount, &pay, &bonus))
	require.Equal(t, 14.0, amount)
	require.Equal(t, 100.0, pay)
	require.Zero(t, bonus)
	var precision, scale int
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT numeric_precision, numeric_scale FROM information_schema.columns WHERE table_schema='public' AND table_name='payment_orders' AND column_name='pay_amount'`).Scan(&precision, &scale))
	require.Equal(t, 21, precision)
	require.Equal(t, 3, scale)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, upgradeDB)))
	var legacyID int64
	require.NoError(t, upgradeDB.QueryRowContext(ctx, `SELECT id FROM payment_orders WHERE out_trade_no='legacy-promotion-fixture'`).Scan(&legacyID))
	legacy, err := client.PaymentOrder.Get(ctx, legacyID)
	require.NoError(t, err)
	require.Equal(t, 14.0, legacy.Amount)
	require.Zero(t, legacy.BonusAmount)
	_, err = legacy.Update().SetBonusAmount(2.8).SetPayAmount(9.999).Save(ctx)
	require.NoError(t, err)
	updated, err := client.PaymentOrder.Get(ctx, legacyID)
	require.NoError(t, err)
	require.Equal(t, 14.0, updated.Amount)
	require.Equal(t, 2.8, updated.BonusAmount)
	require.Equal(t, 9.999, updated.PayAmount)
}

func TestPaymentBonusMigrationPreservesLegacyCredit(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	var precision, scale int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT numeric_precision, numeric_scale FROM information_schema.columns WHERE table_name='payment_orders' AND column_name='bonus_amount'`).Scan(&precision, &scale))
	require.Equal(t, 20, precision)
	require.Equal(t, 2, scale)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT numeric_precision, numeric_scale FROM information_schema.columns WHERE table_schema='public' AND table_name='payment_orders' AND column_name='pay_amount'`).Scan(&precision, &scale))
	require.Equal(t, 21, precision)
	require.Equal(t, 3, scale)
	requireColumn(t, tx, "payment_orders", "bonus_amount", "numeric", 0, false)
	requireColumnDefaultContains(t, tx, "payment_orders", "bonus_amount", "0")
	requireColumn(t, tx, "payment_orders", "pay_amount", "numeric", 0, false)
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
	_, err = conn.ExecContext(ctx, `INSERT INTO payment_orders (id, amount, pay_amount) VALUES (2, 16.8, 9.999)`)
	require.NoError(t, err)
	var exactPay string
	require.NoError(t, conn.QueryRowContext(ctx, `SELECT pay_amount::text, bonus_amount FROM payment_orders WHERE id=2`).Scan(&exactPay, &bonus))
	require.Equal(t, "9.999", exactPay)
	require.Zero(t, bonus, "new rows receive the database default")
	_, err = conn.ExecContext(ctx, `UPDATE payment_orders SET bonus_amount=NULL WHERE id=2`)
	require.ErrorContains(t, err, "null value")
}
