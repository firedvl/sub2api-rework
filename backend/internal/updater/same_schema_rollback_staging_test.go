//go:build staging

package updater

import (
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

func qualifyStagingSameSchemaRollback(t *testing.T, docker string, compose []string, service *Service, runner *stagingRunner) {
	t.Helper()
	source := stagingRecoverySource{
		version:  "0.2.3-rework.8",
		image:    "ghcr.io/firedvl/sub2api-rework@sha256:da3f32d69e04a8c2ae5ece8da023ab4f131e15d11031eed1a99f39cc410f5604",
		revision: "c066ff092042455e81759e9b154bfaaa92ddedbb", migration: 251,
	}
	if stagingCandidateMigration != source.migration {
		t.Fatal("same-schema rollback fixture needs a separate review when candidate migrations change")
	}
	if output, err := runStagingCommand(docker, "pull", source.image); err != nil {
		t.Fatalf("pull immutable .8 rollback source: %v: %s", err, output)
	}
	assertStagingRecoveryProvenance(t, docker, source)
	if err := rewriteEnvironmentImage(service.policy.EnvironmentFile, source.image); err != nil {
		t.Fatal(err)
	}
	if output, err := runStagingCommand(docker, append(append([]string(nil), compose...), "up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", "180", "sub2api")...); err != nil {
		t.Fatalf("advance disposable fixture to .8: %v: %s", err, output)
	}
	// Reset only the disposable test state after the preceding recovery scenarios.
	state := stagingRecoveryState(t, service)
	state.Prepared, state.Backup, state.Recovery, state.Exposure = nil, nil, nil, ""
	state.Status = updatecontract.UpdaterStatus{SchemaVersion: state.Status.SchemaVersion, UpdaterVersion: Version, Healthy: true,
		State: updatecontract.UpdaterStateIdle, InstalledVersion: source.version, CurrentMigration: source.migration, UpdatedAt: time.Now().UTC()}
	if err := service.store.save(state); err != nil {
		t.Fatal(err)
	}
	service.healthHTTP.Transport = http.DefaultTransport
	service.fetcher = stagingRecoveryManifestFetcher{digest: runner.targetDigest}
	assertStagingRecoveryHealthy(t, docker, compose, service, source)
	prepareStagingRecoveryCandidate(t, service)
	requestHostStagingOperation(t, service, updatecontract.OperationInstall, stagingRecoveryCandidateVersion)
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
	if status.CurrentMigration != source.migration || stagingApplicationImageID(t, docker, compose) != runner.targetID {
		t.Fatal("same-schema candidate install did not reach exact candidate image/schema")
	}
	stagingSQL(t, docker, compose, "CREATE TABLE same_schema_post_upgrade_sentinel (value text NOT NULL); INSERT INTO same_schema_post_upgrade_sentinel VALUES ('keep-acknowledged-write')")
	before := len(stagingRecoveryCommands(runner))
	if _, err := service.Start(updatecontract.OperationRollback, updatecontract.OperationRequest{
		Version: source.version, Actor: "admin:1", Confirmation: "ROLLBACK " + source.version,
	}); err != nil {
		t.Fatal(err)
	}
	waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
	assertStagingNoDestructiveCommands(t, stagingRecoveryCommands(runner)[before:], false)
	assertStagingRecoveryHealthy(t, docker, compose, service, source)
	if stagingQuery(t, docker, compose, "SELECT value FROM same_schema_post_upgrade_sentinel") != "keep-acknowledged-write" {
		t.Fatal("application-only rollback lost an acknowledged post-upgrade database write")
	}
}
