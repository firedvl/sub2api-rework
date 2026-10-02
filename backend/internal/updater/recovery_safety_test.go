//go:build unit

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

type safetyRunner struct {
	*fakeRunner
	before func(string, io.Writer) error
	after  func(string, io.Writer) error
}

func (r *safetyRunner) Run(ctx context.Context, in io.Reader, out io.Writer, name string, args ...string) error {
	command := strings.Join(args, " ")
	if r.before != nil {
		if err := r.before(command, out); err != nil {
			return err
		}
	}
	if err := r.fakeRunner.Run(ctx, in, out, name, args...); err != nil {
		return err
	}
	if r.after != nil {
		return r.after(command, out)
	}
	return nil
}

func installSafetyFixture(t *testing.T) (*Service, *fakeRunner) {
	t.Helper()
	runner := &fakeRunner{}
	svc, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, svc)
	_, err := svc.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
	runner.databaseContents = "source-and-post-success-sentinel"
	return svc, runner
}

func TestOldRecoveryRejectedAndRescueRetainsPostSuccessWrite(t *testing.T) {
	svc, runner := installSafetyFixture(t)
	calls := runner.callsSnapshot()
	_, err := svc.Start(updatecontract.OperationRecover, recoverRequest())
	require.Error(t, err)
	require.Equal(t, calls, runner.callsSnapshot())
	require.Equal(t, "source-and-post-success-sentinel", runner.databaseContents)
	prepareRecovery(t, svc)
	state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	require.Equal(t, exposurePossible, state.Exposure)
	require.Equal(t, 233, state.Recovery.Rescue.SourceMigration)
	rescueBytes, err := os.ReadFile(state.Recovery.Rescue.DatabaseBackup)
	require.NoError(t, err)
	require.Contains(t, string(rescueBytes), "post-success-sentinel")
	ack := recoverRequest(svc)
	_, err = svc.Start(updatecontract.OperationRecover, ack)
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateSucceeded, waitForUpdater(t, svc, 5*time.Second).State)
	require.NotContains(t, runner.databaseContents, "post-success-sentinel")
	require.FileExists(t, state.Backup.DatabaseBackup)
	require.FileExists(t, state.Recovery.Rescue.DatabaseBackup)
	require.NoError(t, svc.validateRecoveryBackup(state.Recovery.Rescue))
	_, err = svc.Start(updatecontract.OperationRecover, ack)
	require.Error(t, err, "completed consent cannot be replayed")
}

func TestLaunchFailurePersistsExposureBeforeCommandAndPreservesCurrentDatabase(t *testing.T) {
	runner := &fakeRunner{}
	svc, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, svc)
	exposureObserved := false
	svc.runner = &safetyRunner{fakeRunner: runner, before: func(command string, _ io.Writer) error {
		if strings.Contains(command, " up -d --no-deps sub2api") {
			state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
			if err != nil {
				return err
			}
			exposureObserved = state.Exposure == exposurePossible
			runner.databaseContents = "launch-may-have-written"
			return fmt.Errorf("synthetic launch failure")
		}
		return nil
	}}
	_, err := svc.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, svc, 5*time.Second)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.True(t, exposureObserved)
	require.False(t, runner.hasCall(" pg_restore -U"))
	require.Zero(t, runner.upCount)
	require.Equal(t, 233, runner.migration)
	require.Equal(t, "launch-may-have-written", runner.databaseContents)
	require.True(t, runner.applicationStopped)
	restarted, err := NewService(svc.policy, runner, svc.fetcher)
	require.NoError(t, err)
	state, err := restarted.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	require.Equal(t, exposurePossible, state.Exposure)
	_, err = restarted.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{Version: recoverRequest().Version, Actor: "admin:1", Confirmation: "ROLLBACK " + recoverRequest().Version})
	require.Error(t, err)
}

func TestRescuePreparationFailureNeverRestoresDatabase(t *testing.T) {
	for _, failure := range []string{"dump", "disk", "checksum", "metadata", "archive", "state"} {
		t.Run(failure, func(t *testing.T) {
			svc, runner := installSafetyFixture(t)
			svc.runner = &safetyRunner{fakeRunner: runner, after: func(command string, output io.Writer) error {
				if failure == "archive" && strings.Contains(command, "pg_restore --list") {
					return fmt.Errorf("invalid archive")
				}
				if !strings.Contains(command, " pg_dump ") {
					return nil
				}
				dump := output.(*os.File)
				switch failure {
				case "dump":
					return fmt.Errorf("dump failed")
				case "disk":
					return dump.Close()
				case "checksum":
					return os.Chmod(dump.Name(), 0644)
				case "metadata":
					return os.Mkdir(filepath.Join(filepath.Dir(dump.Name()), "metadata.json"), 0700)
				case "state":
					if err := os.Remove(svc.store.path); err != nil {
						return err
					}
					return os.Mkdir(svc.store.path, 0700)
				}
				return nil
			}}
			_, err := svc.Start(updatecontract.OperationPrepareRecovery, updatecontract.OperationRequest{Version: recoverRequest().Version, Actor: "admin:1", Confirmation: "PREPARE RECOVERY " + recoverRequest().Version})
			require.NoError(t, err)
			if failure == "state" {
				require.Eventually(t, func() bool {
					svc.statusMu.RLock()
					visible := svc.statusErr != nil
					svc.statusMu.RUnlock()
					return visible
				}, 5*time.Second, 10*time.Millisecond)
			}
			status := waitForUpdater(t, svc, 5*time.Second)
			require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
			require.Equal(t, "failed", status.LastAttempt.Result)
			require.False(t, runner.hasCall(" pg_restore -U"))
			require.False(t, runner.hasCall("DROP DATABASE IF EXISTS"))
			require.Equal(t, "source-and-post-success-sentinel", runner.databaseContents)
		})
	}
}

func TestRecoveryRejectsTamperingStaleGenerationAndResumedApplication(t *testing.T) {
	for _, scenario := range []string{"rescue", "original", "generation", "application restart"} {
		t.Run(scenario, func(t *testing.T) {
			svc, runner := installSafetyFixture(t)
			prepareRecovery(t, svc)
			ack := recoverRequest(svc)
			state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
			require.NoError(t, err)
			switch scenario {
			case "rescue":
				require.NoError(t, os.WriteFile(state.Recovery.Rescue.DatabaseBackup, []byte("tampered"), 0600))
			case "original":
				require.NoError(t, os.WriteFile(state.Backup.DatabaseBackup, []byte("tampered"), 0600))
			case "generation":
				prepareRecovery(t, svc)
			case "application restart":
				runner.applicationStartedAt = "2026-10-02T00:00:00Z"
			}
			_, err = svc.Start(updatecontract.OperationRecover, ack)
			if scenario == "application restart" {
				require.NoError(t, err)
				require.Equal(t, updatecontract.UpdaterStateCritical, waitForUpdater(t, svc, 5*time.Second).State)
			} else {
				require.Error(t, err)
			}
			require.False(t, runner.hasCall(" pg_restore -U"))
			require.Equal(t, "source-and-post-success-sentinel", runner.databaseContents)
		})
	}
}

func TestLegacyStateUnknownFieldsAndRecoveryMetadataFailClosed(t *testing.T) {
	svc, runner := installSafetyFixture(t)
	state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	state.Exposure = ""
	state.Status.UpdaterVersion = "1.1.4"
	require.NoError(t, svc.store.save(state))
	restarted, err := NewService(svc.policy, runner, svc.fetcher)
	require.NoError(t, err)
	_, err = restarted.Start(updatecontract.OperationRecover, recoverRequest())
	require.Error(t, err)
	prepareRecovery(t, restarted)
	prepared, err := restarted.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	for _, mutate := range []func(map[string]any){
		func(value map[string]any) { value["unknown"] = true },
		func(value map[string]any) { value["schema_version"] = 3 },
		func(value map[string]any) { value["exposure"] = "safe" },
		func(value map[string]any) { value["recovery"].(map[string]any)["source_update_id"] = "different" },
		func(value map[string]any) { delete(value["recovery"].(map[string]any), "quiesced") },
	} {
		data, err := json.Marshal(prepared)
		require.NoError(t, err)
		var value map[string]any
		require.NoError(t, json.Unmarshal(data, &value))
		mutate(value)
		data, err = json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(svc.store.path, data, 0600))
		_, err = NewService(svc.policy, runner, svc.fetcher)
		require.Error(t, err)
	}
}

func TestFailedOrInterruptedRepreparationInvalidatesEarlierConsent(t *testing.T) {
	svc, runner := installSafetyFixture(t)
	prepareRecovery(t, svc)
	old := recoverRequest(svc)
	runner.failBackup = true
	_, err := svc.Start(updatecontract.OperationPrepareRecovery, updatecontract.OperationRequest{Version: old.Version, Actor: "admin:1", Confirmation: "PREPARE RECOVERY " + old.Version})
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateCritical, waitForUpdater(t, svc, 5*time.Second).State)
	restarted, err := NewService(svc.policy, runner, svc.fetcher)
	require.NoError(t, err)
	_, err = restarted.Start(updatecontract.OperationRecover, old)
	require.Error(t, err)
	status, err := restarted.RecoveryStatus()
	require.NoError(t, err)
	require.Empty(t, status.Confirmation)
	require.False(t, runner.hasCall(" pg_restore -U"))
}

func TestSourceDot6CannotStartAgainstSchema249EvenWithMisrecordedBackup(t *testing.T) {
	svc, runner := installSafetyFixture(t)
	runner.migration = 249
	require.Error(t, svc.restoreApplication(context.Background(), &backupMetadata{SourceVersion: "0.2.3-rework.6", SourceMigration: 249}))
	require.Equal(t, 1, runner.upCount)
}

func TestRescueVersionUsesActiveImageInsteadOfFuturePreparedRelease(t *testing.T) {
	svc, _ := installSafetyFixture(t)
	var manifest updatecontract.Manifest
	require.NoError(t, json.Unmarshal(validUpdaterManifest(t), &manifest))
	manifest.ReworkVersion = "0.1.185-rework.1"
	manifest.Image = "ghcr.io/firedvl/sub2api-rework:" + manifest.ReworkVersion
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	svc.fetcher = &fakeManifestFetcher{data: data}
	_, err = svc.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: manifest.ReworkVersion, Actor: "admin:1"})
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStatePrepared, waitForUpdater(t, svc, 5*time.Second).State)
	status, err := svc.RecoveryStatus()
	require.NoError(t, err)
	require.Equal(t, installRequest().Version, status.CurrentVersion)
	prepareRecovery(t, svc)
	status, err = svc.RecoveryStatus()
	require.NoError(t, err)
	require.Equal(t, installRequest().Version, status.CurrentVersion)
}
