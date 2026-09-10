//go:build unit

package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

type recoveryFailureRunner struct {
	*fakeRunner
	failCommand string
	entered     chan struct{}
	resume      chan struct{}
}

func (r *recoveryFailureRunner) Run(ctx context.Context, in io.Reader, out io.Writer, name string, args ...string) error {
	if strings.Contains(strings.Join(args, " "), r.failCommand) {
		if r.entered != nil {
			close(r.entered)
			select {
			case <-r.resume:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return fmt.Errorf("synthetic recovery command failure")
	}
	return r.fakeRunner.Run(ctx, in, out, name, args...)
}

func installedRecoveryFixture(t *testing.T) (*Service, *fakeRunner, persistedState) {
	t.Helper()
	runner := &fakeRunner{}
	svc, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, svc)
	_, err := svc.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
	state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	return svc, runner, state
}

func TestRecoveryRejectsInvalidRecordsAndConfirmation(t *testing.T) {
	for _, scenario := range []string{"missing dump", "metadata mismatch", "environment checksum", "live compose drift", "live environment drift", "wrong version", "ordinary confirmation"} {
		t.Run(scenario, func(t *testing.T) {
			svc, runner, state := installedRecoveryFixture(t)
			request := recoverRequest()
			switch scenario {
			case "missing dump":
				require.NoError(t, os.Remove(state.Backup.DatabaseBackup))
			case "metadata mismatch":
				require.NoError(t, os.WriteFile(filepath.Join(state.Backup.Directory, "metadata.json"), []byte(`{}`), 0600))
			case "environment checksum":
				require.NoError(t, os.WriteFile(state.Backup.EnvironmentCopy, []byte("tampered"), 0600))
			case "live compose drift":
				require.NoError(t, os.WriteFile(svc.policy.ComposeFiles[0], []byte("services: {}\n"), 0600))
			case "live environment drift":
				data, err := os.ReadFile(svc.policy.EnvironmentFile)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(svc.policy.EnvironmentFile, append(data, []byte("COMPOSE_PROJECT_NAME=another-deployment\n")...), 0600))
			case "wrong version":
				request.Version = "0.1.182-rework.1"
				request.Confirmation = "RESTORE DATABASE AND ROLLBACK " + request.Version
			case "ordinary confirmation":
				request.Confirmation = "ROLLBACK " + request.Version
			}
			_, err := svc.Start(updatecontract.OperationRecover, request)
			require.Error(t, err)
			require.False(t, runner.hasCall(" pg_restore "))
			current, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
			require.NoError(t, err)
			require.Equal(t, state, current, "rejected recovery must not rewrite recorded state")
		})
	}
}

func TestRecoveryCommandFailuresRetainTargetAndPermitRetry(t *testing.T) {
	for _, command := range []string{" pg_restore ", " up -d --no-deps sub2api", "inspect --format {{.Config.Image}}"} {
		t.Run(command, func(t *testing.T) {
			svc, runner, before := installedRecoveryFixture(t)
			svc.runner = &recoveryFailureRunner{fakeRunner: runner, failCommand: command}
			_, err := svc.Start(updatecontract.OperationRecover, recoverRequest())
			require.NoError(t, err)
			status := waitForUpdater(t, svc, 5*time.Second)
			require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
			require.Equal(t, before.Backup.SourceVersion, status.RollbackVersion)
			require.Equal(t, "failed", status.LastRollback.Result)
			require.FileExists(t, before.Backup.DatabaseBackup)
			_, err = svc.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: installRequest().Version, Actor: "admin:1"})
			require.Error(t, err)
			svc.runner = runner
			_, err = svc.Start(updatecontract.OperationRecover, recoverRequest())
			require.NoError(t, err)
			require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
		})
	}
}

func TestRecoveryRejectsConcurrentOperation(t *testing.T) {
	svc, runner, _ := installedRecoveryFixture(t)
	entered, resume := make(chan struct{}), make(chan struct{})
	svc.runner = &recoveryFailureRunner{fakeRunner: runner, failCommand: " pg_restore ", entered: entered, resume: resume}
	_, err := svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(resume)
		t.Fatal("recovery did not enter restore")
	}
	_, busyErr := svc.Start(updatecontract.OperationRecover, recoverRequest())
	close(resume)
	require.ErrorIs(t, busyErr, ErrOperationBusy)
	require.Equal(t, updatecontract.UpdaterStateCritical, waitForUpdater(t, svc, 5*time.Second).State)
}

func TestRecoveryInterruptedStateCanResumeRecordedBackup(t *testing.T) {
	svc, runner, state := installedRecoveryFixture(t)
	state.Status.Busy = true
	state.Status.State = updatecontract.UpdaterStateCritical
	state.Status.LastAttempt.Action = updatecontract.OperationRecover
	require.NoError(t, svc.store.save(state))
	restarted, err := NewService(svc.policy, runner, svc.fetcher)
	require.NoError(t, err)
	status, err := restarted.Status()
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.False(t, status.Busy)
	_, err = restarted.Start(updatecontract.OperationInstall, installRequest())
	require.Error(t, err)
	_, err = restarted.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, restarted, 5*time.Second).State)
}

func TestRecoveryHealthFailureRemainsCritical(t *testing.T) {
	svc, _, before := installedRecoveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	svc.policy.HealthBaseURL = server.URL
	svc.policy.HealthTimeoutSeconds = 1
	_, err := svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, svc, 5*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.Equal(t, before.Backup.SourceVersion, status.RollbackVersion)
	require.Equal(t, "failed", status.LastRollback.Result)
	require.FileExists(t, before.Backup.DatabaseBackup)
}

func TestRecoveryAuditFailurePreservesRetryTarget(t *testing.T) {
	svc, _, before := installedRecoveryFixture(t)
	require.NoError(t, os.Remove(svc.store.auditPath))
	require.NoError(t, os.Mkdir(svc.store.auditPath, 0700))
	_, err := svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, svc, 5*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.Equal(t, before.Backup.SourceVersion, status.RollbackVersion)
	require.Contains(t, status.LastError, "audit persistence failed")
	require.Equal(t, "failed", status.LastRollback.Result)
	_, err = svc.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: installRequest().Version, Actor: "admin:1"})
	require.Error(t, err)
	require.NoError(t, os.Remove(svc.store.auditPath))
	_, err = svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
}

func TestIncompleteRecoveryAtSourceSchemaCannotUseOrdinaryRollback(t *testing.T) {
	svc, runner, state := installedRecoveryFixture(t)
	state.Status.State = updatecontract.UpdaterStateCritical
	state.Status.LastAttempt.Action = updatecontract.OperationRecover
	state.Status.LastAttempt.Result = "failed"
	require.NoError(t, svc.store.save(state))
	runner.migration = state.Backup.SourceMigration
	_, err := svc.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{
		Version: recoverRequest().Version, Actor: "admin:1", Confirmation: "ROLLBACK " + recoverRequest().Version,
	})
	require.Error(t, err, "schema number alone does not prove a complete restored database")
	_, err = svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
}

func TestRecoveryUsesMaintenanceDatabaseOutsideTarget(t *testing.T) {
	svc, runner, _ := installedRecoveryFixture(t)
	svc.policy.DatabaseName = "postgres"
	require.NoError(t, svc.checkPostgresMaintenance(context.Background()))
	require.True(t, runner.hasCall("-d template1"))
	require.NoError(t, svc.recreateDatabase(context.Background()))
	require.True(t, runner.hasCall(`DROP DATABASE IF EXISTS "postgres"`))
}
