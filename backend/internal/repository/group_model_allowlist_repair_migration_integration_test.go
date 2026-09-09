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

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration244UpgradesVerified239WithoutCopyingDisplayConfig(t *testing.T) {
	ctx := context.Background()
	db := newGroupModelAllowlistMigrationDB(t)
	require.NoError(t, applyMigrationsFS(ctx, db, migrationsThrough(t, "239_group_free_openai_fast.sql")))

	var groupID int64
	require.NoError(t, db.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status, models_list_config)
VALUES ('migration-244-display', 'openai', 1, 'active', '{"enabled":true,"models":["display-only-model"]}'::jsonb)
RETURNING id
`).Scan(&groupID))

	require.NoError(t, ApplyMigrations(ctx, db))

	var display, allowlist string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT models_list_config::text, model_allowlist::text FROM groups WHERE id = $1
`, groupID).Scan(&display, &allowlist))
	require.JSONEq(t, `{"enabled":true,"models":["display-only-model"]}`, display)
	require.JSONEq(t, `{}`, allowlist, "enforcement starts disabled instead of inheriting display configuration")

	var applied int
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM schema_migrations
WHERE filename IN (
  '240_add_usage_log_upstream_request_id.sql',
  '241_add_usage_log_upstream_request_id_index_notx.sql',
  '242_channel_max_reasoning_effort_multiplier.sql',
  '243_group_codex_models_manifest_config.sql',
  '244_group_model_allowlist.sql'
)
`).Scan(&applied))
	require.Equal(t, 5, applied)
	require.NoError(t, ApplyMigrations(ctx, db), "recorded migration remains restart-safe")
}

func TestMigration244FailsClosedForPartialRestoreAndUnknownProvenance(t *testing.T) {
	ctx := context.Background()
	db := newGroupModelAllowlistMigrationDB(t)
	require.NoError(t, applyMigrationsFS(ctx, db, migrationsThrough(t, "239_group_free_openai_fast.sql")))

	_, err := db.ExecContext(ctx, `UPDATE schema_migrations SET checksum = 'unknown' WHERE filename = '143_group_models_list_config.sql'`)
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "checksum mismatch")

	_, err = db.ExecContext(ctx, `UPDATE schema_migrations SET checksum = $1 WHERE filename = '143_group_models_list_config.sql'`, "b0a2cac2567db903a8967456fff348f59530c2633b4dae363c32a0e3b6503cb3")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "ALTER TABLE groups DROP COLUMN models_list_config")
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "display-only models_list_config is missing")

	_, err = db.ExecContext(ctx, "ALTER TABLE groups ADD COLUMN models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "ALTER TABLE groups ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "preexisting enforcement policy has unknown provenance")
}

func TestMigrationsRunnerGroupModelAllowlistStartupGuard(t *testing.T) {
	ctx := context.Background()
	db := newGroupModelAllowlistMigrationDB(t)
	require.NoError(t, ApplyMigrations(ctx, db))

	var groupID int64
	require.NoError(t, db.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status)
VALUES ('migration-244-guard', 'openai', 1, 'active') RETURNING id
`).Scan(&groupID))

	// A fresh install records 244 with a disabled enforcement policy, and startup checks it again.
	require.NoError(t, ApplyMigrations(ctx, db))

	_, err := db.ExecContext(ctx, "ALTER TABLE groups ALTER COLUMN model_allowlist DROP NOT NULL")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE groups SET model_allowlist = NULL WHERE id = $1", groupID)
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "both present and non-null")

	_, err = db.ExecContext(ctx, "UPDATE groups SET model_allowlist = '{}'::jsonb WHERE id = $1", groupID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "ALTER TABLE groups ALTER COLUMN model_allowlist SET NOT NULL")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE groups SET model_allowlist = '[]'::jsonb WHERE id = $1", groupID)
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "invalid policy shape")

	_, err = db.ExecContext(ctx, `UPDATE groups SET model_allowlist = '{"enabled":true,"models":["custom-model"]}'::jsonb WHERE id = $1`, groupID)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db), "valid custom policies survive the startup guard")
	for _, policy := range []string{`null`, `{"enabled":"true"}`, `{"models":null}`, `{"models":{}}`, `{"models":[1]}`, `{"enabled":true}`, `{"enabled":true,"models":[]}`} {
		_, err = db.ExecContext(ctx, "UPDATE groups SET model_allowlist = $1::jsonb WHERE id = $2", policy, groupID)
		require.NoError(t, err)
		err = ApplyMigrations(ctx, db)
		require.Error(t, err, "malformed enforcement must block startup")
		require.NotContains(t, err.Error(), policy, "diagnostics must not contain model policy data")
		var retained string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT model_allowlist::text FROM groups WHERE id = $1", groupID).Scan(&retained))
		require.JSONEq(t, policy, retained)
	}
	_, err = db.ExecContext(ctx, `UPDATE groups SET model_allowlist = '{"enabled":true,"models":["allowed"]}', models_list_config = '{"enabled":true,"models":["display"]}' WHERE id=$1`, groupID)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))
	_, err = db.ExecContext(ctx, "ALTER TABLE groups ALTER COLUMN model_allowlist DROP DEFAULT")
	require.NoError(t, err)
	require.Error(t, ApplyMigrations(ctx, db))
	_, err = db.ExecContext(ctx, "ALTER TABLE groups ALTER COLUMN model_allowlist SET DEFAULT '{}'::jsonb")
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))

	_, err = db.ExecContext(ctx, "ALTER TABLE groups DROP COLUMN model_allowlist")
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "both present and non-null")
}

func newGroupModelAllowlistMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dbName := fmt.Sprintf("sub2api_group_allowlist_%d", time.Now().UnixNano())
	_, err := integrationDB.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err)

	dsn, err := url.Parse(integrationPostgresDSN)
	require.NoError(t, err)
	dsn.Path = "/" + dbName
	dsn.RawPath = ""
	db, err := openSQLWithRetry(ctx, dsn.String(), 30*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		if _, dropErr := integrationDB.ExecContext(dropCtx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); dropErr != nil {
			t.Errorf("drop migration rehearsal database: %v", dropErr)
		}
	})
	return db
}

func TestMigration244ResolvesGroupsThroughSearchPath(t *testing.T) {
	ctx := context.Background()
	db := newGroupModelAllowlistMigrationDB(t)
	db.SetMaxOpenConns(1)
	require.NoError(t, applyMigrationsFS(ctx, db, migrationsThrough(t, "239_group_free_openai_fast.sql")))
	_, err := db.ExecContext(ctx, `CREATE SCHEMA policy_test; ALTER TABLE groups SET SCHEMA policy_test; ALTER TABLE schema_migrations SET SCHEMA policy_test; ALTER TABLE atlas_schema_revisions SET SCHEMA policy_test; SET search_path TO policy_test, public`)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db))
	var schema string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.oid='groups'::regclass`).Scan(&schema))
	require.Equal(t, "policy_test", schema)
}

func TestMigration241RebuildsInterruptedRequestIDIndex(t *testing.T) {
	ctx := context.Background()
	db := newGroupModelAllowlistMigrationDB(t)
	_, err := db.ExecContext(ctx, `CREATE TABLE usage_logs (upstream_request_id TEXT, created_at TIMESTAMPTZ); INSERT INTO usage_logs VALUES ('duplicate', NOW()), ('duplicate', NOW())`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE UNIQUE INDEX CONCURRENTLY idx_usage_logs_upstream_request_id ON usage_logs (upstream_request_id)`)
	require.Error(t, err, "failed concurrent creation must leave an invalid index")
	invalid, err := indexIsInvalid(ctx, db, usageLogsUpstreamRequestIDIndex)
	require.NoError(t, err)
	require.True(t, invalid)
	data, err := dbmigrations.FS.ReadFile(usageLogsUpstreamRequestIDIndexMigration)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, db, fstest.MapFS{usageLogsUpstreamRequestIDIndexMigration: &fstest.MapFile{Data: data}}))
	invalid, err = indexIsInvalid(ctx, db, usageLogsUpstreamRequestIDIndex)
	require.NoError(t, err)
	require.False(t, invalid)
	var unique bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT indisunique FROM pg_index WHERE indexrelid='idx_usage_logs_upstream_request_id'::regclass`).Scan(&unique))
	require.False(t, unique)
}

func migrationsThrough(t *testing.T, last string) fstest.MapFS {
	t.Helper()
	files, err := fs.Glob(dbmigrations.FS, "*.sql")
	require.NoError(t, err)
	selected := fstest.MapFS{}
	for _, name := range files {
		if name > last {
			continue
		}
		data, readErr := dbmigrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		selected[name] = &fstest.MapFile{Data: data}
	}
	return selected
}
