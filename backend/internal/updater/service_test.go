//go:build unit

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

const (
	testTargetDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testSourceDigest = "weishaw/sub2api@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

type fakeManifestFetcher struct {
	data []byte
	err  error
}

func (f *fakeManifestFetcher) Fetch(context.Context, string) ([]byte, error) {
	return f.data, f.err
}

type fakeRunner struct {
	mu                   sync.Mutex
	calls                []string
	composeConfig        string
	runtimeComposeConfig string
	volumeInspect        string
	migration            int
	digestMismatch       bool
	tagDigestMismatch    bool
	failDatabase         bool
	redisAuthRequired    bool
	redisClientAuthValid bool
	failRedis            bool
	redisReply           string
	redisSecret          string
	failBackup           bool
	failRestore          bool
	failMigration        bool
	failStopAt           int
	failHealthOnTarget   bool
	healthFail           bool
	upCount              int
	stopCount            int
	cancelOnMigration    context.CancelFunc
	forwardCanceled      bool
	recoveryLive         bool
	recoveryBounded      bool
}

func (f *fakeRunner) Run(ctx context.Context, _ io.Reader, stdout io.Writer, _ string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forwardCanceled {
		f.recoveryLive = ctx.Err() == nil
		_, f.recoveryBounded = ctx.Deadline()
	}
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, joined)
	switch {
	case strings.Contains(joined, "config --services"):
		_, _ = io.WriteString(stdout, "sub2api\npostgres\nredis\n")
	case strings.Contains(joined, "config --no-interpolate --format json"):
		_, _ = io.WriteString(stdout, f.composeConfig)
	case strings.Contains(joined, "config --format json"):
		if f.runtimeComposeConfig == "" {
			_, _ = io.WriteString(stdout, f.composeConfig)
		} else {
			_, _ = io.WriteString(stdout, f.runtimeComposeConfig)
		}
	case strings.Contains(joined, "volume inspect --format {{json .}}"):
		_, _ = io.WriteString(stdout, f.volumeInspect)
	case strings.Contains(joined, "pg_isready"):
		if f.failDatabase {
			return fmt.Errorf("database unavailable")
		}
	case strings.Contains(joined, "redis-cli --raw ping"):
		if f.failRedis {
			if f.redisSecret != "" {
				return fmt.Errorf("redis unavailable: %s", f.redisSecret)
			}
			return fmt.Errorf("redis unavailable")
		}
		if f.redisReply != "" {
			_, _ = io.WriteString(stdout, f.redisReply+"\n")
			break
		}
		withoutAuth := strings.Contains(joined, "env -u REDISCLI_AUTH")
		switch {
		case f.redisAuthRequired && withoutAuth:
			_, _ = io.WriteString(stdout, "NOAUTH Authentication required.\n")
		case f.redisAuthRequired && f.redisClientAuthValid:
			_, _ = io.WriteString(stdout, "PONG\n")
		case f.redisAuthRequired:
			_, _ = io.WriteString(stdout, "NOAUTH Authentication required.\n")
		default:
			_, _ = io.WriteString(stdout, "PONG\n")
		}
	case strings.Contains(joined, " psql "):
		_, _ = fmt.Fprintf(stdout, "%d\n", f.migration)
	case strings.Contains(joined, "image inspect"):
		image := args[len(args)-1]
		var digests []string
		if strings.Contains(image, "ghcr.io/firedvl/sub2api-rework") {
			digest := "ghcr.io/firedvl/sub2api-rework@" + testTargetDigest
			if f.digestMismatch || (f.tagDigestMismatch && !strings.Contains(image, "@")) {
				digest = "ghcr.io/firedvl/sub2api-rework@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
			}
			digests = []string{digest}
		} else {
			digests = []string{testSourceDigest}
		}
		data, _ := json.Marshal(digests)
		_, _ = stdout.Write(data)
	case strings.Contains(joined, " pg_dump "):
		if f.failBackup {
			return fmt.Errorf("backup failed")
		}
		_, _ = io.WriteString(stdout, "synthetic-database-backup")
	case strings.Contains(joined, "/app/sub2api --migrate"):
		if f.cancelOnMigration != nil {
			f.forwardCanceled = true
			f.cancelOnMigration()
			return ctx.Err()
		}
		if f.failMigration {
			return fmt.Errorf("migration failed")
		}
		f.migration = 233
	case strings.Contains(joined, " stop sub2api"):
		f.stopCount++
		if f.failStopAt == f.stopCount {
			return fmt.Errorf("stop failed")
		}
	case strings.Contains(joined, " pg_restore "):
		if f.failRestore {
			return fmt.Errorf("restore failed")
		}
		f.migration = 232
	case strings.Contains(joined, " up -d --no-deps sub2api"):
		f.upCount++
		if f.upCount == 1 && f.failHealthOnTarget {
			f.healthFail = true
		} else if f.upCount > 1 {
			f.healthFail = false
		}
	}
	return nil
}

func (f *fakeRunner) hasCall(fragment string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if strings.Contains(call, fragment) {
			return true
		}
	}
	return false
}

func (f *fakeRunner) isHealthFailing() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthFail
}

func (f *fakeRunner) callIndex(fragment string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index, call := range f.calls {
		if strings.Contains(call, fragment) {
			return index
		}
	}
	return -1
}

func (f *fakeRunner) recoveryContextState() (live, bounded bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recoveryLive, f.recoveryBounded
}

func (f *fakeRunner) callsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func validUpdaterManifest(t *testing.T) []byte {
	t.Helper()
	manifest := updatecontract.Manifest{
		SchemaVersion: 1, ReworkVersion: "0.1.184-rework.1", UpstreamVersion: "v0.1.184",
		GitSHA: strings.Repeat("a", 40), Image: "ghcr.io/firedvl/sub2api-rework:0.1.184-rework.1",
		ImageDigest: testTargetDigest, MigrationMin: 232, MigrationMax: 233,
		ReleaseDate: "2026-08-28T12:00:00Z", Compatibility: updatecontract.CompatibilityApproved,
		MinimumUpdaterVersion: Version, ReleaseNotes: updatecontract.ReleaseNotes{Rollback: "Automatic restore uses the pre-update backup."},
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	return data
}

func newUpdaterTestService(t *testing.T, runner *fakeRunner) (*Service, Policy) {
	return newUpdaterTestServiceWithPolicy(t, runner, nil)
}

func newUpdaterTestServiceWithPolicy(t *testing.T, runner *fakeRunner, mutate func(*Policy)) (*Service, Policy) {
	t.Helper()
	root := t.TempDir()
	policy := testPolicy(root)
	if mutate != nil {
		mutate(&policy)
	}
	writeUpdaterTestDeployment(t, policy)
	runner.composeConfig = validRenderedComposeConfig(policy)
	runner.migration = 232
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if runner.isHealthFailing() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(healthServer.Close)
	policy.HealthBaseURL = healthServer.URL
	policy.HealthTimeoutSeconds = 5
	service, err := NewService(policy, runner, &fakeManifestFetcher{data: validUpdaterManifest(t)})
	require.NoError(t, err)
	return service, policy
}

func writeUpdaterTestDeployment(t *testing.T, policy Policy) {
	t.Helper()
	require.NoError(t, os.MkdirAll(policy.DeploymentDirectory, 0700))
	require.NoError(t, os.WriteFile(policy.ComposeFiles[0], []byte(`services:
  sub2api:
    image: ${SUB2API_IMAGE:-weishaw/sub2api:latest}
    environment:
      - SUB2API_UPDATER_SOCKET=`+policy.SocketPath+`
      - SUB2API_UPDATER_GID=1234
    group_add:
      - "1234"
    volumes:
      - type: bind
        source: `+filepath.Dir(policy.SocketPath)+`
        target: `+filepath.Dir(policy.SocketPath)+`
  postgres:
    image: postgres:18-alpine
  redis:
    image: redis:8-alpine
`), 0600))
	for _, path := range policy.ComposeFiles[1:] {
		require.NoError(t, os.WriteFile(path, []byte("services: {}\n"), 0600))
	}
	require.NoError(t, os.WriteFile(policy.EnvironmentFile, []byte("POSTGRES_PASSWORD=synthetic\n"), 0600))
}

func validRenderedComposeConfig(policy Policy) string {
	data, _ := json.Marshal(map[string]any{"services": map[string]any{
		policy.ApplicationService: map[string]any{
			"image": "${SUB2API_IMAGE:-weishaw/sub2api:latest}",
			"environment": map[string]string{
				"SUB2API_UPDATER_SOCKET": policy.SocketPath,
				"SUB2API_UPDATER_GID":    "1234",
			},
			"group_add": []string{"1234"},
			"volumes": []map[string]string{{
				"type": "bind", "source": filepath.Dir(policy.SocketPath), "target": filepath.Dir(policy.SocketPath),
			}},
		},
		policy.DatabaseService: map[string]any{"image": "postgres:18-alpine"},
		policy.RedisService:    map[string]any{"image": "redis:8-alpine"},
	}})
	return string(data)
}

func waitForUpdater(t *testing.T, service *Service, timeout time.Duration) updatecontract.UpdaterStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := service.Status()
		require.NoError(t, err)
		if !status.Busy {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("updater operation did not finish")
	return updatecontract.UpdaterStatus{}
}

func prepareUpdater(t *testing.T, service *Service) {
	t.Helper()
	_, err := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStatePrepared, status.State)
}

func installRequest() updatecontract.OperationRequest {
	return updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Confirmation: "INSTALL 0.1.184-rework.1", Actor: "admin:1",
	}
}

func TestCheckRedisClearsStaleClientAuthForNoAuthServer(t *testing.T) {
	policy := testPolicy(t.TempDir())
	runner := &fakeRunner{}
	service := &Service{policy: policy, runner: runner}

	require.NoError(t, service.checkRedis(context.Background()))
	require.True(t, runner.hasCall("env -u REDISCLI_AUTH redis-cli --raw ping"))
	require.False(t, runner.hasCall(" redis redis-cli --raw ping"))
}

func TestCheckRedisRequiresPongForAuthenticatedAndUnavailableServers(t *testing.T) {
	const secret = "redis-auth-secret"
	tests := []struct {
		name   string
		runner *fakeRunner
		pass   bool
	}{
		{"authenticated", &fakeRunner{redisAuthRequired: true, redisClientAuthValid: true}, true},
		{"wrong password", &fakeRunner{redisAuthRequired: true, redisSecret: secret}, false},
		{"missing password", &fakeRunner{redisAuthRequired: true}, false},
		{"unavailable", &fakeRunner{failRedis: true}, false},
		{"non-PONG reply", &fakeRunner{redisReply: "LOADING Redis is loading"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := testPolicy(t.TempDir())
			service := &Service{policy: policy, runner: test.runner}

			err := service.checkRedis(context.Background())

			if test.pass {
				require.NoError(t, err)
				require.True(t, test.runner.hasCall("env -u REDISCLI_AUTH redis-cli --raw ping"))
				require.True(t, test.runner.hasCall(" redis redis-cli --raw ping"))
			} else {
				require.EqualError(t, err, "redis ping failed")
				require.NotContains(t, err.Error(), secret)
			}
			for _, call := range test.runner.callsSnapshot() {
				require.NotContains(t, call, secret)
			}
		})
	}
}

func TestRedisAuthFailureIsRedactedFromStateAndAudit(t *testing.T) {
	const secret = "redis-secret-that-must-not-leak"
	runner := &fakeRunner{failRedis: true, redisSecret: secret}
	service, policy := newUpdaterTestService(t, runner)

	_, err := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Equal(t, "redis is unavailable", status.LastError)
	audit, err := os.ReadFile(policy.AuditPath)
	require.NoError(t, err)
	require.NotContains(t, string(audit), secret)
	require.NotContains(t, status.LastError, secret)
}

func TestUpdater111PreparesRework4FromRework3(t *testing.T) {
	runner := &fakeRunner{}
	service, _ := newUpdaterTestServiceWithPolicy(t, runner, func(policy *Policy) {
		policy.InitialInstalledVersion = "0.1.183-rework.3"
	})
	var manifest updatecontract.Manifest
	require.NoError(t, json.Unmarshal(validUpdaterManifest(t), &manifest))
	manifest.ReworkVersion = "0.1.183-rework.4"
	manifest.Image = "ghcr.io/firedvl/sub2api-rework:0.1.183-rework.4"
	manifest.MinimumUpdaterVersion = "1.1.1"
	manifest.MigrationMax = 232
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	service.fetcher = &fakeManifestFetcher{data: data}

	_, err = service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: manifest.ReworkVersion, Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStatePrepared, status.State)
	require.Equal(t, "0.1.183-rework.3", status.InstalledVersion)
	require.Equal(t, manifest.ReworkVersion, status.PreparedVersion)
	require.Equal(t, 232, status.CurrentMigration)
}

func TestPrepareRejectsDigestMismatch(t *testing.T) {
	runner := &fakeRunner{digestMismatch: true}
	service, policy := newUpdaterTestService(t, runner)
	_, err := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: "0.1.184-rework.1", Actor: "admin:1"})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Contains(t, status.LastError, "digest")
	environment, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	require.NotContains(t, string(environment), "SUB2API_IMAGE")
}

func TestPrepareRejectsDigestFromDifferentTag(t *testing.T) {
	runner := &fakeRunner{tagDigestMismatch: true}
	service, _ := newUpdaterTestService(t, runner)
	_, err := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: "0.1.184-rework.1", Actor: "admin:1"})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Contains(t, status.LastError, "digest")
}

func TestInstallPreflightFailureDoesNotMutateDeployment(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	runner.failDatabase = true
	before, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	_, err = service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	after, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.False(t, runner.hasCall(" pg_dump "))
	require.False(t, runner.hasCall("/app/sub2api --migrate"))
}

func TestBackupFailureAbortsBeforeEnvironmentMutation(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	runner.failBackup = true
	before, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	_, err = service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Contains(t, status.LastError, "backup")
	after, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.False(t, runner.hasCall("/app/sub2api --migrate"))
}

func TestInstallSuccessPinsDigestAndRecordsRollback(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 5*time.Second)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, status.State)
	require.Equal(t, "0.1.184-rework.1", status.InstalledVersion)
	require.Equal(t, "0.1.183-rework.1", status.RollbackVersion)
	require.Equal(t, 233, status.CurrentMigration)
	environment, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	require.Contains(t, string(environment), "SUB2API_IMAGE=ghcr.io/firedvl/sub2api-rework:0.1.184-rework.1@"+testTargetDigest)
	backups, err := os.ReadDir(policy.BackupDirectory)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.FileExists(t, filepath.Join(policy.BackupDirectory, backups[0].Name(), "postgres.dump"))
	require.Less(t, runner.callIndex(" stop sub2api"), runner.callIndex("/app/sub2api --migrate"))
}

func TestAutomaticRollbackUsesFreshBoundedContext(t *testing.T) {
	runner := &fakeRunner{}
	service, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	ctx, cancel := context.WithCancel(context.Background())
	runner.cancelOnMigration = cancel
	state, err := service.store.load(service.policy.InitialInstalledVersion, service.policy.InitialMigration, Version)
	require.NoError(t, err)
	summary := updatecontract.OperationSummary{OperationID: "upd-context", Action: updatecontract.OperationInstall}

	err = service.install(ctx, "0.1.184-rework.1", &summary, &state)

	require.ErrorContains(t, err, "automatic rollback succeeded")
	require.True(t, runner.hasCall(" pg_restore "))
	live, bounded := runner.recoveryContextState()
	require.True(t, live)
	require.True(t, bounded)
}

func TestFailedMigrationForcesDatabaseRestore(t *testing.T) {
	runner := &fakeRunner{failMigration: true}
	service, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, "succeeded", status.LastAttempt.RollbackResult)
	require.True(t, runner.hasCall(" pg_restore "))
}

func TestRestoreStopFailureSkipsDatabaseRestore(t *testing.T) {
	runner := &fakeRunner{failMigration: true, failStopAt: 2}
	service, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.False(t, runner.hasCall(" pg_restore "))
}

func TestHealthFailureRestoresDatabaseAndPreviousImage(t *testing.T) {
	runner := &fakeRunner{failHealthOnTarget: true}
	service, policy := newUpdaterTestServiceWithPolicy(t, runner, func(policy *Policy) {
		policy.ComposeFiles = append(policy.ComposeFiles, filepath.Join(policy.DeploymentDirectory, "docker-compose.updater.yml"))
	})
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 10*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Equal(t, "succeeded", status.LastAttempt.RollbackResult)
	require.Equal(t, "0.1.183-rework.1", status.InstalledVersion)
	require.Equal(t, 232, status.CurrentMigration)
	require.True(t, runner.hasCall(" pg_restore "))
	databaseReset := runner.callIndex("DROP DATABASE IF EXISTS")
	require.NotEqual(t, -1, databaseReset)
	require.Equal(t, databaseReset, runner.callIndex("CREATE DATABASE"))
	require.Less(t, databaseReset, runner.callIndex(" pg_restore "))
	environment, err := os.ReadFile(policy.EnvironmentFile)
	require.NoError(t, err)
	require.Contains(t, string(environment), "SUB2API_IMAGE="+testSourceDigest)
	prefix := strings.Join((&Service{policy: policy}).composeArgs(), " ")
	upCalls := 0
	for _, call := range runner.callsSnapshot() {
		if strings.Contains(call, " up -d --no-deps sub2api") {
			upCalls++
			require.True(t, strings.HasPrefix(call, prefix), call)
		}
	}
	require.Equal(t, 2, upCalls)
}

func TestRollbackFailureLeavesCriticalState(t *testing.T) {
	runner := &fakeRunner{failHealthOnTarget: true, failRestore: true}
	service, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, service, 10*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.Equal(t, "failed", status.LastAttempt.RollbackResult)
}

func TestCriticalStateRejectsPrepareAndInstallWithoutChangingRecoveryMetadata(t *testing.T) {
	runner := &fakeRunner{}
	service, _ := newUpdaterTestService(t, runner)
	state, err := service.store.load(service.policy.InitialInstalledVersion, service.policy.InitialMigration, Version)
	require.NoError(t, err)
	state.Status.State = updatecontract.UpdaterStateCritical
	state.Status.RollbackVersion = "0.1.182-rework.1"
	state.Backup = &backupMetadata{UpdateID: "preserve-me", SourceVersion: "0.1.182-rework.1"}
	require.NoError(t, service.store.save(state))

	_, prepareErr := service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Actor: "admin:1",
	})
	_, installErr := service.Start(updatecontract.OperationInstall, installRequest())

	require.ErrorContains(t, prepareErr, "requires recovery")
	require.ErrorContains(t, installErr, "requires recovery")
	stored, err := service.store.load(service.policy.InitialInstalledVersion, service.policy.InitialMigration, Version)
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateCritical, stored.Status.State)
	require.Equal(t, "preserve-me", stored.Backup.UpdateID)
	require.Equal(t, "0.1.182-rework.1", stored.Status.RollbackVersion)
}

func TestCriticalStateAllowsRecordedRollback(t *testing.T) {
	runner := &fakeRunner{}
	service, policy := newUpdaterTestService(t, runner)
	seedCriticalRollbackState(t, service, policy)

	_, err := service.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{
		Version: "0.1.183-rework.1", Confirmation: "ROLLBACK 0.1.183-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, status.State)
	require.Equal(t, "0.1.183-rework.1", status.InstalledVersion)
}

func TestFailedCriticalRollbackKeepsRecoveryState(t *testing.T) {
	runner := &fakeRunner{failDatabase: true}
	service, policy := newUpdaterTestService(t, runner)
	seedCriticalRollbackState(t, service, policy)

	_, err := service.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{
		Version: "0.1.183-rework.1", Confirmation: "ROLLBACK 0.1.183-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)

	_, err = service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
		Version: "0.1.184-rework.1", Actor: "admin:1",
	})
	require.ErrorContains(t, err, "requires recovery")
	stored, err := service.store.load(service.policy.InitialInstalledVersion, service.policy.InitialMigration, Version)
	require.NoError(t, err)
	require.Equal(t, "recorded", stored.Backup.UpdateID)
	require.Equal(t, "0.1.183-rework.1", stored.Status.RollbackVersion)
}

func TestInterruptedMutatingOperationRequiresRecovery(t *testing.T) {
	for _, interruptedState := range []updatecontract.UpdaterState{
		updatecontract.UpdaterStateInstalling,
		updatecontract.UpdaterStateRollingBack,
	} {
		t.Run(string(interruptedState), func(t *testing.T) {
			root := t.TempDir()
			policy := testPolicy(root)
			writeUpdaterTestDeployment(t, policy)
			state := persistedState{SchemaVersion: 2, Status: updatecontract.UpdaterStatus{
				SchemaVersion: 1, UpdaterVersion: Version, Healthy: true, Busy: true,
				State: interruptedState, InstalledVersion: policy.InitialInstalledVersion,
				CurrentMigration: policy.InitialMigration,
			}}
			require.NoError(t, newStateStore(policy).save(state))

			service, err := NewService(policy, &fakeRunner{}, &fakeManifestFetcher{data: validUpdaterManifest(t)})
			require.NoError(t, err)
			status, err := service.Status()
			require.NoError(t, err)
			require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
			require.False(t, status.Busy)
			_, err = service.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{
				Version: "0.1.184-rework.1", Actor: "admin:1",
			})
			require.ErrorContains(t, err, "requires recovery")
		})
	}
}

func seedCriticalRollbackState(t *testing.T, service *Service, policy Policy) {
	t.Helper()
	backupDir := filepath.Join(policy.BackupDirectory, "recorded")
	require.NoError(t, os.MkdirAll(backupDir, 0700))
	environmentCopy := filepath.Join(backupDir, "deployment.env")
	composeCopy := filepath.Join(backupDir, "compose-000.yaml")
	databaseBackup := filepath.Join(backupDir, "postgres.dump")
	require.NoError(t, os.WriteFile(environmentCopy, []byte("SUB2API_IMAGE="+testSourceDigest+"\n"), 0600))
	require.NoError(t, os.WriteFile(composeCopy, []byte("services: {}\n"), 0600))
	require.NoError(t, os.WriteFile(databaseBackup, []byte("synthetic backup"), 0600))
	state, err := service.store.load(policy.InitialInstalledVersion, policy.InitialMigration, Version)
	require.NoError(t, err)
	state.Status.State = updatecontract.UpdaterStateCritical
	state.Status.InstalledVersion = "0.1.184-rework.1"
	state.Status.RollbackVersion = "0.1.183-rework.1"
	state.Backup = &backupMetadata{
		UpdateID: "recorded", Directory: backupDir, EnvironmentCopy: environmentCopy,
		ComposeFiles: []backupComposeFile{{
			OriginalPath: policy.ComposeFiles[0], BackupPath: composeCopy,
			SHA256: composeChecksum([]byte("services: {}\n")),
		}}, DatabaseBackup: databaseBackup,
		SourceVersion: "0.1.183-rework.1", TargetVersion: "0.1.184-rework.1",
		SourceDigest: testSourceDigest, SourceMigration: 232,
	}
	metadata, err := json.MarshalIndent(state.Backup, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "metadata.json"), append(metadata, '\n'), 0600))
	require.NoError(t, service.store.save(state))
}

func TestManualRollbackBlocksAfterMigrationAdvance(t *testing.T) {
	runner := &fakeRunner{}
	service, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, service)
	_, err := service.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, service, 5*time.Second).State)
	_, err = service.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{
		Version: "0.1.183-rework.1", Confirmation: "ROLLBACK 0.1.183-rework.1", Actor: "admin:1",
	})
	require.NoError(t, err)
	status := waitForUpdater(t, service, 3*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Contains(t, status.LastError, "migrations changed")
}

func TestFinalStateSaveFailureIsVisibleAsCritical(t *testing.T) {
	runner := &fakeRunner{}
	service, _ := newUpdaterTestService(t, runner)
	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blockedParent, []byte("blocked"), 0600))
	service.store.path = filepath.Join(blockedParent, "state.json")
	state := persistedState{SchemaVersion: 2, Status: updatecontract.UpdaterStatus{
		SchemaVersion: 1, Healthy: true, Busy: true, State: updatecontract.UpdaterStatePreparing,
		InstalledVersion: "0.1.183-rework.1", CurrentMigration: 232,
	}}
	service.finish(updatecontract.OperationSummary{
		OperationID: "upd-test", Action: updatecontract.OperationPrepare, Actor: "admin:1",
		SourceVersion: "0.1.183-rework.1", TargetVersion: "0.1.184-rework.1",
	}, nil, &state)
	status, err := service.Status()
	require.NoError(t, err)
	require.False(t, status.Healthy)
	require.False(t, status.Busy)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.Equal(t, "Updater state persistence failed.", status.LastError)
}
