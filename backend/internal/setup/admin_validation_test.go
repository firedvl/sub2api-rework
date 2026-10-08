package setup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdminValidationMatchesLoginAndBcrypt(t *testing.T) {
	for _, email := range []string{"a@b", "Owner <owner@example.com>", "<owner@example.com>", "invalid"} {
		require.False(t, validateEmail(email), "setup cannot create an identity login rejects")
	}
	require.True(t, validateEmail("owner@example.com"))
	for _, password := range []string{"1234567", strings.Repeat("a", 73), strings.Repeat("\u00e9", 37), " ", strings.Repeat(" ", 8), strings.Repeat(" ", 100)} {
		require.Error(t, validatePassword(password))
	}
	require.NoError(t, validatePassword("12345678"))
	require.NoError(t, validatePassword(strings.Repeat("\u00e9", 36)))
	require.NoError(t, validatePassword(" valid-synthetic-password "))
}

func TestBootstrapAdminPreservesExistingUsers(t *testing.T) {
	for _, adminUsers := range []int{0, 1} {
		t.Run(map[int]string{0: "users-without-admin", 1: "existing-admin"}[adminUsers], func(t *testing.T) {
			t.Setenv("DATA_DIR", t.TempDir())
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users WHERE role").WithArgs(service.RoleAdmin).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(adminUsers))
			created, _, err := bootstrapAdminUser(context.Background(), db, &SetupConfig{Admin: AdminConfig{Email: "invalid", Password: "weak"}})
			require.NoError(t, err)
			require.False(t, created)
			_, err = os.Stat(filepath.Join(GetDataDir(), bootstrapCredentialsFile))
			require.True(t, os.IsNotExist(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBootstrapAdminCredentialFile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		admin     AdminConfig
		block     string
		wantError bool
	}{
		{name: "random-defaults"},
		{name: "provided", admin: AdminConfig{Email: "  owner@example.com\n", Password: "synthetic-valid-password"}},
		{name: "only-random-email", admin: AdminConfig{Password: "synthetic-valid-password"}},
		{name: "weak", admin: AdminConfig{Email: "owner@example.com", Password: "weak"}, wantError: true},
		{name: "explicit-whitespace", admin: AdminConfig{Email: "owner@example.com", Password: " "}, wantError: true},
		{name: "bad-email", admin: AdminConfig{Email: "a@b", Password: "synthetic-valid-password"}, wantError: true},
		{name: "existing-file", block: "file", wantError: true},
		{name: "symlink", block: "symlink", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATA_DIR", t.TempDir())
			t.Setenv("RUN_MODE", "simple")
			credentialPath := filepath.Join(GetDataDir(), bootstrapCredentialsFile)
			if tc.block != "" {
				if tc.block == "file" {
					require.NoError(t, os.WriteFile(credentialPath, []byte("existing synthetic data"), 0644))
				} else {
					require.NoError(t, os.Symlink(filepath.Join(GetDataDir(), "nonexistent-target"), credentialPath))
				}
			}
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users WHERE role").WithArgs(service.RoleAdmin).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			if !tc.wantError {
				mock.ExpectExec("INSERT INTO users").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), service.RoleAdmin, float64(0), simpleModeAdminConcurrency, service.StatusActive, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
			}
			cfg := &SetupConfig{Admin: tc.admin}
			created, _, err := bootstrapAdminUser(context.Background(), db, cfg)
			if tc.wantError {
				require.Error(t, err)
				require.False(t, created)
			} else {
				require.NoError(t, err)
				require.True(t, created)
				if tc.name == "provided" {
					_, err = os.Stat(credentialPath)
					require.True(t, os.IsNotExist(err), "provided credentials should not be persisted")
				} else {
					info, err := os.Stat(credentialPath)
					require.NoError(t, err)
					require.Equal(t, os.FileMode(0600), info.Mode().Perm())
					data, err := os.ReadFile(credentialPath)
					require.NoError(t, err)
					var saved AdminConfig
					require.NoError(t, json.Unmarshal(data, &saved))
					require.True(t, regexp.MustCompile(`^admin-[0-9a-f]{12}@sub2api\.local$`).MatchString(saved.Email))
					if tc.name == "only-random-email" {
						require.Empty(t, saved.Password, "do not duplicate an explicitly supplied secret")
					} else {
						require.Len(t, saved.Password, 32)
					}
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBootstrapPreservesCredentialsAfterUnknownInsertOutcome(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM users WHERE role").WithArgs(service.RoleAdmin).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO users").WillReturnError(context.DeadlineExceeded)
	created, _, err := bootstrapAdminUser(context.Background(), db, &SetupConfig{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, created)
	credentialPath := filepath.Join(GetDataDir(), bootstrapCredentialsFile)
	info, err := os.Stat(credentialPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Positive(t, info.Size())
	require.Error(t, saveBootstrapCredentials(AdminConfig{}), "automatic retry must not overwrite the saved identity")
	require.NoError(t, mock.ExpectationsWereMet())
}
