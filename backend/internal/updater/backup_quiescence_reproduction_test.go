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

func TestPreExposureRestoreRetainsAcknowledgedWriteBeforeQuiescence(t *testing.T) {
	runner := &fakeRunner{failMigration: true, databaseContents: "source-snapshot"}
	svc, _ := newUpdaterTestService(t, runner)
	prepareUpdater(t, svc)
	writeAcknowledgedBeforeQuiescence := false
	dumpAfterQuiescence := false
	svc.runner = &safetyRunner{fakeRunner: runner, after: func(command string, _ io.Writer) error {
		if command == "update --restart=no "+testContainerID && !writeAcknowledgedBeforeQuiescence {
			writeAcknowledgedBeforeQuiescence = !runner.applicationStopped
			runner.databaseContents = "acknowledged-before-quiescence"
		}
		if strings.Contains(command, " pg_dump ") {
			dumpAfterQuiescence = runner.applicationStopped
		}
		return nil
	}}
	_, err := svc.Start(updatecontract.OperationInstall, installRequest())
	require.NoError(t, err)
	status := waitForUpdater(t, svc, 5*time.Second)
	require.Equal(t, updatecontract.UpdaterStateFailed, status.State)
	require.Equal(t, "succeeded", status.LastAttempt.RollbackResult)
	require.True(t, writeAcknowledgedBeforeQuiescence)
	require.True(t, dumpAfterQuiescence)
	require.Less(t, runner.callIndex("stop "+testContainerID), runner.callIndex(" pg_dump "))
	require.Equal(t, "acknowledged-before-quiescence", runner.databaseContents,
		"automatic pre-exposure restore must retain writes acknowledged before application quiescence")
}

func TestInstallSnapshotFailuresResumeSourceWithoutDatabaseRestore(t *testing.T) {
	for _, failure := range []string{"stop", "dump", "checksum", "metadata", "archive", "cancel", "cancel_after_snapshot", "state"} {
		t.Run(failure, func(t *testing.T) {
			runner := &fakeRunner{databaseContents: "acknowledged-source-write"}
			svc, _ := newUpdaterTestService(t, runner)
			prepareUpdater(t, svc)
			state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
			require.NoError(t, err)
			environment, err := os.ReadFile(svc.policy.EnvironmentFile)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			injected := false
			svc.runner = &safetyRunner{fakeRunner: runner, before: func(command string, _ io.Writer) error {
				if failure == "stop" && command == "stop "+testContainerID && !injected {
					injected = true
					return fmt.Errorf("synthetic source stop failure")
				}
				return nil
			}, after: func(command string, output io.Writer) error {
				if failure == "cancel_after_snapshot" && strings.Contains(command, "pg_restore --list") {
					runner.forwardCanceled = true
					cancel()
				}
				if failure == "archive" && strings.Contains(command, "pg_restore --list") {
					return fmt.Errorf("synthetic invalid archive")
				}
				if !strings.Contains(command, " pg_dump ") {
					return nil
				}
				file := output.(*os.File)
				switch failure {
				case "dump":
					return fmt.Errorf("synthetic dump failure")
				case "checksum":
					return file.Chmod(0644)
				case "metadata":
					return os.Mkdir(filepath.Join(filepath.Dir(file.Name()), "metadata.json"), 0700)
				case "cancel":
					runner.forwardCanceled = true
					cancel()
					return ctx.Err()
				case "state":
					if err := os.Remove(svc.store.path); err != nil {
						return err
					}
					return os.Mkdir(svc.store.path, 0700)
				}
				return nil
			}}
			summary := updatecontract.OperationSummary{OperationID: "upd-" + strings.Repeat("e", 24), Action: updatecontract.OperationInstall}
			err = svc.install(ctx, installRequest().Version, &summary, &state)
			require.ErrorContains(t, err, "unchanged source restarted without database restore")
			require.Equal(t, "succeeded", summary.RollbackResult)
			require.NotEqual(t, updatecontract.UpdaterStateCritical, state.Status.State)
			require.False(t, runner.applicationStopped)
			require.Equal(t, 232, runner.migration)
			require.Equal(t, "acknowledged-source-write", runner.databaseContents)
			require.True(t, runner.hasCall(" up -d --no-deps --force-recreate sub2api"))
			require.False(t, runner.hasCall(" pg_restore -U"))
			require.False(t, runner.hasCall("DROP DATABASE"))
			require.False(t, runner.hasCall("/app/sub2api --migrate"))
			after, err := os.ReadFile(svc.policy.EnvironmentFile)
			require.NoError(t, err)
			require.Equal(t, environment, after)
			if strings.HasPrefix(failure, "cancel") {
				live, bounded := runner.recoveryContextState()
				require.True(t, live)
				require.True(t, bounded)
			}
		})
	}
}

func TestInstallSnapshotFailureSourceRestartFailureIsCritical(t *testing.T) {
	for _, failure := range []string{"startup", "health", "schema", "partial_stop"} {
		t.Run(failure, func(t *testing.T) {
			runner := &fakeRunner{failBackup: true, databaseContents: "current-source-write"}
			if failure == "partial_stop" {
				runner.failStopAt = 1
			}
			svc, _ := newUpdaterTestService(t, runner)
			prepareUpdater(t, svc)
			svc.policy.HealthTimeoutSeconds = 1
			svc.runner = &safetyRunner{fakeRunner: runner, before: func(command string, _ io.Writer) error {
				if (failure == "startup" || failure == "partial_stop") && strings.Contains(command, " up -d --no-deps --force-recreate sub2api") {
					return fmt.Errorf("synthetic source startup failure")
				}
				return nil
			}, after: func(command string, _ io.Writer) error {
				if strings.Contains(command, "stop "+testContainerID) {
					switch failure {
					case "health":
						runner.mu.Lock()
						runner.healthFail = true
						runner.mu.Unlock()
					case "schema":
						runner.migration = 233
					}
				}
				return nil
			}}
			_, err := svc.Start(updatecontract.OperationInstall, installRequest())
			require.NoError(t, err)
			status := waitForUpdater(t, svc, 5*time.Second)
			require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
			require.Equal(t, "failed", status.LastAttempt.RollbackResult)
			require.True(t, runner.applicationStopped)
			require.False(t, runner.hasCall(" pg_restore -U"))
			require.False(t, runner.hasCall("DROP DATABASE"))
			require.Equal(t, "current-source-write", runner.databaseContents)
			if failure == "partial_stop" {
				require.False(t, runner.hasCall(" pg_dump "))
			}
		})
	}
}

func TestAcceptedInstallClearsStaleRecoveryBeforeCommands(t *testing.T) {
	svc, runner := installSafetyFixture(t)
	prepareRecovery(t, svc)
	state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
	require.NoError(t, err)
	oldBackup, oldRescue := state.Backup.DatabaseBackup, state.Recovery.Rescue.DatabaseBackup
	state.Recovery.Phase = recoveryCompleted
	state.Status.State = updatecontract.UpdaterStateSucceeded
	require.NoError(t, svc.store.save(state))
	var manifest updatecontract.Manifest
	require.NoError(t, json.Unmarshal(validUpdaterManifest(t), &manifest))
	manifest.ReworkVersion = "0.1.185-rework.1"
	manifest.Image = "ghcr.io/firedvl/sub2api-rework:" + manifest.ReworkVersion
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	svc.fetcher = &fakeManifestFetcher{data: data}
	_, err = svc.Start(updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: manifest.ReworkVersion, Actor: "admin:1"})
	require.NoError(t, err)
	waitForUpdater(t, svc, 5*time.Second)
	cleared := false
	var interrupted persistedState
	svc.runner = &safetyRunner{fakeRunner: runner, before: func(_ string, _ io.Writer) error {
		state, err := svc.store.load(svc.policy.InitialInstalledVersion, svc.policy.InitialMigration, Version)
		if err != nil {
			return err
		}
		cleared = state.Backup == nil && state.Recovery == nil && state.Status.RollbackVersion == "" && state.Exposure == ""
		interrupted = state
		return fmt.Errorf("synthetic crash before source mutation")
	}}
	_, err = svc.Start(updatecontract.OperationInstall, updatecontract.OperationRequest{Version: manifest.ReworkVersion, Actor: "admin:1", Confirmation: "INSTALL " + manifest.ReworkVersion})
	require.NoError(t, err)
	status := waitForUpdater(t, svc, 5*time.Second)
	require.Equal(t, "failed", status.LastAttempt.Result)
	require.True(t, cleared)
	require.FileExists(t, oldBackup)
	require.FileExists(t, oldRescue)
	require.NoError(t, svc.store.save(interrupted))
	restarted, err := NewService(svc.policy, runner, svc.fetcher)
	require.NoError(t, err)
	status, err = restarted.Status()
	require.NoError(t, err)
	require.Equal(t, updatecontract.UpdaterStateCritical, status.State)
	require.True(t, runner.applicationStopped)
	public, err := restarted.RecoveryStatus()
	require.NoError(t, err)
	require.Nil(t, public)
	_, err = restarted.Start(updatecontract.OperationPrepareRecovery, updatecontract.OperationRequest{
		Version: state.Backup.SourceVersion, Actor: "admin:1", Confirmation: "PREPARE RECOVERY " + state.Backup.SourceVersion,
	})
	require.Error(t, err)
}
