package setup

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestDatabaseConnectionTargetAndBootstrapPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		code pq.ErrorCode
		want []string
		fail bool
	}{
		{"existing", "", []string{"customdb"}, false},
		{"missing", "3D000", []string{"customdb", "postgres", "customdb"}, false},
		{"authentication", "28P01", []string{"customdb"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, targetMock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = target.Close() }()
			bootstrap, bootstrapMock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = bootstrap.Close() }()
			if tc.code == "3D000" {
				bootstrapMock.ExpectQuery(`SELECT EXISTS`).WithArgs("customdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				bootstrapMock.ExpectExec(`CREATE DATABASE "customdb"`).WillReturnResult(sqlmock.NewResult(0, 1))
				bootstrapMock.ExpectClose()
			}
			if !tc.fail {
				targetMock.ExpectClose()
			}
			var opened []string
			open := func(_ *DatabaseConfig, name string) (*sql.DB, error) {
				opened = append(opened, name)
				if len(opened) == 1 && tc.code != "" {
					return nil, fmt.Errorf("wrapped: %w", &pq.Error{Code: tc.code})
				}
				if name == "postgres" {
					return bootstrap, nil
				}
				return target, nil
			}
			err = testDatabaseConnection(&DatabaseConfig{DBName: "customdb"}, open)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, opened)
			require.NoError(t, targetMock.ExpectationsWereMet())
			require.NoError(t, bootstrapMock.ExpectationsWereMet())
		})
	}
}
