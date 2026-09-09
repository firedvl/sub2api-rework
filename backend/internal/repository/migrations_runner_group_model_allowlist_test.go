package repository

import (
	"context"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnsureGroupModelAllowlistSchemaRejectsRecordedDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM schema_migrations").
		WithArgs(groupModelAllowlistMigration).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("FROM pg_attribute a").
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "attnotnull", "default_expr"}).
			AddRow("models_list_config", "jsonb", true, "'{}'::jsonb"))

	err = ensureGroupModelAllowlistSchema(context.Background(), db, fstest.MapFS{
		groupModelAllowlistMigration: &fstest.MapFile{Data: []byte("SELECT 1")},
	})
	require.ErrorContains(t, err, "both present and non-null")
	require.NoError(t, mock.ExpectationsWereMet())
}
