//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeQuotaEntCreateUpdatePreservesAllExistingPlatforms(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	ctx = dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	userID := mustCreateUserForQuota(t, client)
	platforms := []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go", "typesafe"}
	for _, platform := range platforms {
		quota, err := client.UserPlatformQuota.Create().SetUserID(userID).SetPlatform(platform).SetDailyLimitUsd(12).SetDailyUsageUsd(3).Save(ctx)
		require.NoError(t, err, platform)
		updated, err := quota.Update().SetPlatform(platform).SetWeeklyLimitUsd(24).Save(ctx)
		require.NoError(t, err, platform)
		require.Equal(t, platform, updated.Platform)
		require.Equal(t, 3.0, updated.DailyUsageUsd)
	}
	quotas, err := NewUserPlatformQuotaRepository(client).ListByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, quotas, len(platforms))
	for _, quota := range quotas {
		require.Equal(t, 12.0, *quota.DailyLimitUSD)
		require.Equal(t, 24.0, *quota.WeeklyLimitUSD)
	}
	_, err = client.UserPlatformQuota.Create().SetUserID(userID).SetPlatform("unsupported").Save(ctx)
	require.ErrorContains(t, err, "not allowed")
}

func TestMigration250TypeSafeFreshAnd249Upgrade(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(fmt.Sprintf("upgrade_249_%t", upgrade), func(t *testing.T) {
			ctx := context.Background()
			db := newGroupModelAllowlistMigrationDB(t)
			if upgrade {
				require.NoError(t, applyMigrationsFS(ctx, db, migrationsThrough(t, "249_opencode_go_platform.sql")))
			} else {
				require.NoError(t, ApplyMigrations(ctx, db))
			}
			var userID, groupID int64
			require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash, role, status) VALUES ('typesafe-migration@example.test', 'fixture', 'user', 'active') RETURNING id`).Scan(&userID))
			require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups (name, platform, rate_multiplier, status) VALUES ('typesafe-migration', 'composite', 1, 'active') RETURNING id`).Scan(&groupID))
			legacy := []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go"}
			for _, platform := range legacy {
				_, err := db.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd, daily_usage_usd) VALUES ($1, $2, 12, 3)`, userID, platform)
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `INSERT INTO composite_model_routes (group_id, public_model, target_platform) VALUES ($1, $2, $2)`, groupID, platform)
				require.NoError(t, err)
			}
			monitorChecks := make(map[string]string)
			for _, table := range []string{"channel_monitors", "channel_monitor_request_templates"} {
				var definition string
				require.NoError(t, db.QueryRowContext(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2`, table, table+"_provider_check").Scan(&definition))
				monitorChecks[table] = definition
			}
			if upgrade {
				_, err := db.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform) VALUES ($1, 'typesafe')`, userID)
				require.ErrorContains(t, err, "user_platform_quotas_platform_check")
			}
			require.NoError(t, ApplyMigrations(ctx, db))
			require.NoError(t, ApplyMigrations(ctx, db), "canonical runner remains restart-safe")
			_, err := db.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd) VALUES ($1, 'typesafe', 4)`, userID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `INSERT INTO composite_model_routes (group_id, public_model, target_platform) VALUES ($1, 'jev-latest', 'typesafe')`, groupID)
			require.NoError(t, err)
			var retained, routes, applied int
			require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM user_platform_quotas WHERE user_id=$1 AND platform<>'typesafe' AND daily_limit_usd=12 AND daily_usage_usd=3`, userID).Scan(&retained))
			require.Equal(t, len(legacy), retained)
			require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM composite_model_routes WHERE group_id=$1`, groupID).Scan(&routes))
			require.Equal(t, len(legacy)+1, routes)
			require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE filename='250_add_typesafe_platform.sql'`).Scan(&applied))
			require.Equal(t, 1, applied)
			_, err = db.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform) VALUES ($1, 'unsupported')`, userID)
			require.ErrorContains(t, err, "user_platform_quotas_platform_check")
			_, err = db.ExecContext(ctx, `INSERT INTO composite_model_routes (group_id, public_model, target_platform) VALUES ($1, 'unsupported', 'unsupported')`, groupID)
			require.ErrorContains(t, err, "composite_model_routes_target_platform_check")
			for table, before := range monitorChecks {
				var after string
				require.NoError(t, db.QueryRowContext(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2`, table, table+"_provider_check").Scan(&after))
				require.Equal(t, before, after)
				require.NotContains(t, after, "typesafe")
			}
		})
	}
}
