//go:build staging

package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	stagingDot6Version              = "0.2.3-rework.6"
	stagingDot6Image                = "ghcr.io/firedvl/sub2api-rework@sha256:74bbf1f694e59cf0418ef9518f91562ae0a4bab0e8bdd74afa8038180e9c28cb"
	stagingDot6Revision             = "2d61454ebfd43f36baee38bf55bf01a4b6ffd2c3"
	stagingRecoveryCandidateVersion = "0.2.4-rework.0"
	stagingRecoveryCandidateImage   = "ghcr.io/firedvl/sub2api-rework:" + stagingRecoveryCandidateVersion
	stagingLegacyPricingGroup       = "synthetic-dot6-max-multiplier"
)

type stagingRecoveryManifestFetcher struct{ digest string }

func (f stagingRecoveryManifestFetcher) Fetch(_ context.Context, version string) ([]byte, error) {
	if version != stagingRecoveryCandidateVersion {
		return nil, fmt.Errorf("unknown synthetic recovery candidate")
	}
	return json.Marshal(updatecontract.Manifest{
		SchemaVersion: 1, ReworkVersion: version, UpstreamVersion: "v0.2.3",
		GitSHA: strings.Repeat("c", 40), Image: stagingRecoveryCandidateImage, ImageDigest: f.digest,
		MigrationMin: 244, MigrationMax: stagingCandidateMigration, ReleaseDate: "2026-10-02T00:00:00Z",
		Compatibility: updatecontract.CompatibilityApproved, MinimumUpdaterVersion: "1.1.5",
	})
}

// These faults run the real command first: health fails on an actually stopped
// candidate, and a dump with unsafe permissions fails managed checksum validation.
type stagingRecoveryFaultRunner struct {
	*stagingRunner
	healthFailure       bool
	sourceHealthFailure bool
	checksumFailure     bool
	dumpFailure         string
	blockCommand        string
	entered             chan struct{}
	release             <-chan struct{}
	blocked             bool
	docker              string
	compose             []string
}

func (r *stagingRecoveryFaultRunner) Run(ctx context.Context, in io.Reader, out io.Writer, name string, args ...string) error {
	if err := r.stagingRunner.Run(ctx, in, out, name, args...); err != nil {
		return err
	}
	command := strings.Join(args, " ")
	if r.healthFailure && strings.Contains(command, " up -d --no-deps sub2api") || r.sourceHealthFailure && strings.Contains(command, " up -d --no-deps --force-recreate sub2api") {
		r.healthFailure = false
		r.sourceHealthFailure = false
		return r.stagingRunner.Run(ctx, nil, io.Discard, r.docker,
			append(append([]string(nil), r.compose...), "stop", "sub2api")...)
	}
	if (r.checksumFailure || r.dumpFailure != "") && strings.Contains(command, " pg_dump ") {
		r.checksumFailure = false
		file, ok := out.(*os.File)
		if !ok {
			return fmt.Errorf("real rescue dump file unavailable")
		}
		switch r.dumpFailure {
		case "disk":
			return file.Close()
		case "metadata":
			return os.Mkdir(filepath.Join(filepath.Dir(file.Name()), "metadata.json"), 0700)
		case "archive":
			if err := file.Truncate(0); err != nil {
				return err
			}
			_, err := file.WriteAt([]byte("invalid-staging-postgres-archive"), 0)
			return err
		default:
			return file.Chmod(0644)
		}
	}
	if !r.blocked && r.blockCommand != "" && strings.Contains(command, r.blockCommand) {
		r.blocked = true
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func qualifyStagingDot6Recovery(t *testing.T, docker string, compose []string, service *Service, runner *stagingRunner, targetImage string) {
	t.Helper()
	if Version != "1.1.5" {
		t.Fatalf("recovery qualification requires updater1.1.5, got %s", Version)
	}
	if output, err := runStagingCommand(docker, "pull", stagingDot6Image); err != nil {
		t.Fatalf("pull immutable published .6: %v: %s", err, output)
	}
	assertStagingDot6Provenance(t, docker)
	if output, err := runStagingCommand(docker, "tag", targetImage, stagingRecoveryCandidateImage); err != nil {
		t.Fatalf("tag synthetic recovery candidate: %v: %s", err, output)
	}
	t.Cleanup(func() { _, _ = runStagingCommand(docker, "image", "rm", stagingRecoveryCandidateImage) })
	runner.mu.Lock()
	runner.targetImage = stagingRecoveryCandidateImage
	runner.failCommand, runner.failAfter, runner.failNextUp, runner.healthFailed = "", false, false, false
	runner.mu.Unlock()
	service.fetcher = stagingRecoveryManifestFetcher{digest: runner.targetDigest}
	service.healthHTTP.Transport = http.DefaultTransport
	service.policy.HealthTimeoutSeconds = 30

	if err := rewriteEnvironmentImage(service.policy.EnvironmentFile, stagingDot6Image); err != nil {
		t.Fatal(err)
	}
	if output, err := runStagingCommand(docker, append(append([]string(nil), compose...),
		"up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", "180", "sub2api")...); err != nil {
		t.Fatalf("advance disposable .13 fixture to immutable .6: %v: %s", err, output)
	}
	if stagingMigration(t, docker, compose) != 244 || stagingApplicationImageReference(t, docker, compose) != stagingDot6Image {
		t.Fatal("published .6 fixture did not reach exact source digest/schema244")
	}
	// Fixture transition only; production state is never rewritten by qualification.
	state := stagingRecoveryState(t, service)
	state.Prepared, state.Backup, state.Recovery, state.Exposure = nil, nil, nil, ""
	state.Status = updatecontract.UpdaterStatus{
		SchemaVersion: state.Status.SchemaVersion, UpdaterVersion: Version, Healthy: true,
		State: updatecontract.UpdaterStateIdle, InstalledVersion: stagingDot6Version,
		CurrentMigration: 244, UpdatedAt: time.Now().UTC(),
	}
	if err := service.store.save(state); err != nil {
		t.Fatal(err)
	}
	stagingSQL(t, docker, compose, `INSERT INTO groups (name, platform, model_pricing)
		VALUES ('synthetic-dot6-max-multiplier', 'openai',
		'[{"platform":"openai","models":["gpt-staging-max"],"billing_mode":"token","input_price":0.01,"output_price":0.02,"max_reasoning_effort_multiplier":3}]'::jsonb)`)
	assertStagingDot6Healthy(t, docker, compose, service)

	for _, scenario := range []struct {
		name, command          string
		after, exposed, health bool
	}{
		{name: "before_migration", command: " /app/sub2api --migrate"},
		{name: "after_real_migration", command: " /app/sub2api --migrate", after: true},
		{name: "candidate_up", command: " up -d --no-deps sub2api", exposed: true},
		{name: "actual_candidate_health", exposed: true, health: true},
	} {
		t.Logf(".6/schema244 -> candidate/schema%d failure matrix: %s", stagingCandidateMigration, scenario.name)
		prepareStagingRecoveryCandidate(t, service)
		before := stagingRecoveryCommands(runner)
		setStagingRecoveryFailure(runner, scenario.command, scenario.after)
		if scenario.health {
			service.runner = &stagingRecoveryFaultRunner{stagingRunner: runner, healthFailure: true, docker: docker, compose: compose}
		}
		requestHostStagingOperation(t, service, updatecontract.OperationInstall, stagingRecoveryCandidateVersion)
		want := updatecontract.UpdaterStateFailed
		if scenario.exposed {
			want = updatecontract.UpdaterStateCritical
		}
		status := waitForStagingOperation(t, service, want, 5*time.Minute)
		setStagingRecoveryFailure(runner, "", false)
		service.runner = runner
		commands := stagingRecoveryCommands(runner)[len(before):]
		if status.LastAttempt == nil || status.LastAttempt.Result != "failed" {
			t.Fatal("injected install failure did not fail the operation")
		}
		if !scenario.exposed {
			if status.LastAttempt.RollbackResult != "succeeded" || !strings.Contains(strings.Join(commands, "\n"), " pg_restore -U ") {
				t.Fatal("pre-exposure failure did not execute successful real automatic restore")
			}
			assertStagingDot6Healthy(t, docker, compose, service)
			continue
		}
		if status.LastAttempt.RollbackResult != "suppressed" || stagingMigration(t, docker, compose) != stagingCandidateMigration {
			t.Fatal("post-exposure failure did not retain candidate schema and suppress automatic restore")
		}
		assertStagingNoDestructiveCommands(t, commands, true)
		assertStagingRecoveryApplicationStopped(t, docker, compose)
		assertStagingMigratedMultiplier(t, docker, compose, "sub2api")
		requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
		assertStagingRecoveryPrepared(t, docker, compose, service, stagingCandidateMigration)
		requestHostStagingOperation(t, service, updatecontract.OperationRecover, stagingDot6Version)
		waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
		assertStagingDot6Healthy(t, docker, compose, service)
	}

	prepareStagingRecoveryCandidate(t, service)
	requestHostStagingOperation(t, service, updatecontract.OperationInstall, stagingRecoveryCandidateVersion)
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
	if status.CurrentMigration != stagingCandidateMigration || stagingMigration(t, docker, compose) != stagingCandidateMigration || stagingApplicationImageID(t, docker, compose) != runner.targetID {
		t.Fatal("successful recovery candidate did not use candidate image/schema")
	}
	assertStagingMigratedMultiplier(t, docker, compose, "sub2api")
	stagingSQL(t, docker, compose, "CREATE TABLE recovery_post_success_sentinel (value text NOT NULL); INSERT INTO recovery_post_success_sentinel VALUES ('must-be-rescued')")
	oldRequest := updatecontract.OperationRequest{Version: stagingDot6Version, Actor: "admin:1", Confirmation: "RESTORE DATABASE AND ROLLBACK " + stagingDot6Version}
	assertStagingRecoveryRejected(t, service, runner, oldRequest)
	assertStagingCurrentSentinel(t, docker, compose)

	for _, failure := range []string{"pg_dump", "archive", "checksum", "disk", "metadata"} {
		t.Logf("real rescue preparation failure: %s", strings.TrimSpace(failure))
		before := stagingRecoveryCommands(runner)
		if failure == "pg_dump" {
			setStagingRecoveryFailure(runner, " pg_dump ", true)
		} else if failure == "checksum" {
			service.runner = &stagingRecoveryFaultRunner{stagingRunner: runner, checksumFailure: true}
		} else {
			service.runner = &stagingRecoveryFaultRunner{stagingRunner: runner, dumpFailure: failure}
		}
		requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
		status = waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 3*time.Minute)
		service.runner = runner
		setStagingRecoveryFailure(runner, "", false)
		if status.LastAttempt == nil || status.LastAttempt.Result != "failed" {
			t.Fatal("rescue preparation failure reported success")
		}
		if stagingRecoveryState(t, service).Recovery != nil {
			t.Fatal("failed rescue became authorized")
		}
		assertStagingNoDestructiveCommands(t, stagingRecoveryCommands(runner)[len(before):], true)
		assertStagingCurrentSentinel(t, docker, compose)
		assertStagingRecoveryApplicationStopped(t, docker, compose)
	}

	before := stagingRecoveryCommands(runner)
	requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
	state = assertStagingRecoveryPrepared(t, docker, compose, service, stagingCandidateMigration)
	assertStagingNoDestructiveCommands(t, stagingRecoveryCommands(runner)[len(before):], true)
	assertStagingCurrentSentinel(t, docker, compose)
	staleConsent := stagingRecoveryConsent(t, service)
	before = stagingRecoveryCommands(runner)
	setStagingRecoveryFailure(runner, " pg_dump ", true)
	requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
	status = waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 3*time.Minute)
	setStagingRecoveryFailure(runner, "", false)
	if status.LastAttempt == nil || status.LastAttempt.Result != "failed" || stagingRecoveryState(t, service).Recovery.Phase != recoveryPreparing {
		t.Fatal("failed re-preparation retained an authorized recovery phase")
	}
	public, err := service.RecoveryStatus()
	if err != nil || public == nil || public.Confirmation != "" {
		t.Fatal("failed re-preparation exposed old usable consent")
	}
	assertStagingRecoveryRejected(t, service, runner, staleConsent)
	assertStagingNoDestructiveCommands(t, stagingRecoveryCommands(runner)[len(before):], true)
	assertStagingCurrentSentinel(t, docker, compose)
	if err := service.validateRecoveryBackup(state.Recovery.Rescue); err != nil {
		t.Fatal("failed re-preparation damaged previous rescue")
	}
	requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
	state = assertStagingRecoveryPrepared(t, docker, compose, service, stagingCandidateMigration)
	assertStagingRecoveryRejected(t, service, runner, staleConsent)
	preparedStatus, err := service.RecoveryStatus()
	if err != nil || preparedStatus == nil || preparedStatus.Confirmation == "" {
		t.Fatal("operation-bound consent unavailable")
	}
	request := updatecontract.OperationRequest{Version: stagingDot6Version, Actor: "admin:1", Confirmation: preparedStatus.Confirmation}
	for _, backup := range []*backupMetadata{state.Recovery.Rescue, state.Backup} {
		original, err := os.ReadFile(backup.DatabaseBackup)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(backup.DatabaseBackup, []byte("tampered-staging-archive"), 0600); err != nil {
			t.Fatal(err)
		}
		assertStagingRecoveryRejected(t, service, runner, request)
		assertStagingCurrentSentinel(t, docker, compose)
		if err := os.WriteFile(backup.DatabaseBackup, original, 0600); err != nil {
			t.Fatal(err)
		}
		if err := service.validateRecoveryBackup(backup); err != nil {
			t.Fatal(err)
		}
	}

	restarted, err := NewService(service.policy, runner, service.fetcher)
	if err != nil {
		t.Fatal(err)
	}
	restartedStatus, err := restarted.RecoveryStatus()
	if err != nil || !reflect.DeepEqual(preparedStatus, restartedStatus) || !reflect.DeepEqual(state.Recovery, stagingRecoveryState(t, restarted).Recovery) {
		t.Fatal("restart changed prepared recovery identities, rescue, fence, or nonce")
	}
	if status, err := restarted.Status(); err != nil || status.State != updatecontract.UpdaterStateCritical {
		t.Fatal("prepared restart did not remain critical")
	}
	assertStagingRecoveryApplicationStopped(t, docker, compose)
	assertStagingCurrentSentinel(t, docker, compose)
	verifyStagingDot6PricingIncompatibility(t, docker, compose, service, state.Recovery.Rescue)

	// Interrupt real recreation and restore in turn. A retry reuses the first
	// Keep the candidate-schema rescue even after the managed database has reached source244.
	for _, command := range []string{"DROP DATABASE IF EXISTS", " pg_restore -U "} {
		t.Logf("real Stage2 interruption after %s", strings.TrimSpace(command))
		consent := stagingRecoveryConsent(t, service)
		setStagingRecoveryFailure(runner, command, true)
		if command == "DROP DATABASE IF EXISTS" {
			if _, err := restarted.Start(updatecontract.OperationRecover, consent); err != nil {
				t.Fatal(err)
			}
		} else {
			requestHostStagingOperation(t, service, updatecontract.OperationRecover, stagingDot6Version)
		}
		status = waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 3*time.Minute)
		setStagingRecoveryFailure(runner, "", false)
		if status.LastAttempt == nil || status.LastAttempt.Result != "failed" {
			t.Fatal("interrupted restore reported success")
		}
		interrupted := stagingRecoveryState(t, service)
		if interrupted.Recovery.Phase != recoveryRestoring || !reflect.DeepEqual(interrupted.Recovery.Rescue, state.Recovery.Rescue) {
			t.Fatal("interrupted database restore replaced the original rescue")
		}
		assertStagingRecoveryApplicationStopped(t, docker, compose)
		assertStagingRecoveryRejected(t, service, runner, consent)
		before := stagingRecoveryCommands(runner)
		requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
		status = waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 3*time.Minute)
		if status.LastAttempt == nil || status.LastAttempt.Result != "succeeded" || !reflect.DeepEqual(stagingRecoveryState(t, service).Recovery.Rescue, state.Recovery.Rescue) {
			t.Fatal("partial restore preparation overwrote current-data rescue")
		}
		for _, call := range stagingRecoveryCommands(runner)[len(before):] {
			if strings.Contains(call, " pg_dump ") {
				t.Fatal("partial restore retry dumped over the original rescue")
			}
		}
		if err := service.validateRecoveryBackup(state.Recovery.Rescue); err != nil {
			t.Fatal(err)
		}
	}

	for _, failure := range []string{"source_up", "actual_source_health", "audit"} {
		t.Logf("real Stage2 interruption: %s", failure)
		consent := stagingRecoveryConsent(t, service)
		before := stagingRecoveryCommands(runner)
		var restoreAudit func()
		switch failure {
		case "source_up":
			setStagingRecoveryFailure(runner, " up -d --no-deps --force-recreate sub2api", true)
		case "actual_source_health":
			service.runner = &stagingRecoveryFaultRunner{stagingRunner: runner, sourceHealthFailure: true, docker: docker, compose: compose}
		case "audit":
			restoreAudit = breakStagingRecoveryAudit(t, service.policy.AuditPath)
		}
		requestHostStagingOperation(t, service, updatecontract.OperationRecover, stagingDot6Version)
		status = waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 5*time.Minute)
		if restoreAudit != nil {
			restoreAudit()
		}
		setStagingRecoveryFailure(runner, "", false)
		service.runner = runner
		if status.LastAttempt == nil || status.LastAttempt.Result != "failed" {
			t.Fatal("Stage2 interruption reported success")
		}
		if failure != "audit" && stagingRecoveryState(t, service).Recovery.Phase != recoverySourceStarting {
			t.Fatal("source-start interruption did not preserve durable source-start phase")
		}
		assertStagingRecoveryApplicationStopped(t, docker, compose)
		if failure == "audit" {
			assertStagingNoDestructiveCommands(t, stagingRecoveryCommands(runner)[len(before):], true)
		}
		if stagingMigration(t, docker, compose) != 244 {
			t.Fatal("source-start retry did not retain the already restored schema244")
		}
		if err := service.validateRecoveryBackup(state.Recovery.Rescue); err != nil {
			t.Fatal("original candidate-schema rescue was not retained after source-start/audit failure")
		}
		assertStagingRecoveryRejected(t, service, runner, consent)
		requestHostStagingOperation(t, service, updatecontract.OperationPrepareRecovery, stagingDot6Version)
		fresh := assertStagingRecoveryPrepared(t, docker, compose, service, 244)
		if stagingRecoveryConsent(t, service).Confirmation == consent.Confirmation || fresh.Recovery.Rescue.DatabaseBackup == state.Recovery.Rescue.DatabaseBackup {
			t.Fatal("source-start retry failed to create separate fresh rescue consent")
		}
		assertStagingRecoveryRejected(t, service, runner, consent)
	}

	assertStagingConcurrentRecovery(t, service, runner, updatecontract.OperationPrepareRecovery, " pg_dump ")
	assertStagingRecoveryPrepared(t, docker, compose, service, 244)
	finalConsent := stagingRecoveryConsent(t, service)
	assertStagingConcurrentRecovery(t, service, runner, updatecontract.OperationRecover, "pg_restore --list")
	status = waitForStagingOperation(t, service, updatecontract.UpdaterStateSucceeded, 5*time.Minute)
	if status.InstalledVersion != stagingDot6Version || status.CurrentMigration != 244 {
		t.Fatal("explicit recovery did not restore .6/schema244")
	}
	assertStagingDot6Healthy(t, docker, compose, service)
	if stagingQuery(t, docker, compose, "SELECT to_regclass('recovery_post_success_sentinel') IS NULL") != "t" {
		t.Fatal("source244 recovery retained post-success sentinel")
	}
	verifyStagingRescue(t, docker, compose, service, state.Recovery.Rescue)
	if err := service.validateRecoveryBackup(state.Backup); err != nil {
		t.Fatal(err)
	}
	assertStagingRecoveryRejected(t, service, runner, finalConsent)
	t.Logf(".6/schema244 -> candidate/schema%d recovery matrix passed; current writes retained in separately restored rescue", stagingCandidateMigration)
}

func stagingRecoveryState(t *testing.T, service *Service) persistedState {
	t.Helper()
	state, err := service.store.load(service.policy.InitialInstalledVersion, service.policy.InitialMigration, Version)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func stagingRecoveryCommands(runner *stagingRunner) []string {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return append([]string(nil), runner.commands...)
}

func setStagingRecoveryFailure(runner *stagingRunner, command string, after bool) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.failCommand, runner.failAfter = command, after
}

func prepareStagingRecoveryCandidate(t *testing.T, service *Service) {
	t.Helper()
	requestHostStagingOperation(t, service, updatecontract.OperationPrepare, stagingRecoveryCandidateVersion)
	waitForStagingOperation(t, service, updatecontract.UpdaterStatePrepared, 3*time.Minute)
}

func assertStagingDot6Provenance(t *testing.T, docker string) {
	t.Helper()
	labels, err := runStagingCommand(docker, "image", "inspect", stagingDot6Image, "--format", "{{json .Config.Labels}}")
	var values map[string]string
	if err != nil || json.Unmarshal([]byte(labels), &values) != nil || values["org.opencontainers.image.revision"] != stagingDot6Revision || values["org.opencontainers.image.version"] != stagingDot6Version {
		t.Fatal("published .6 OCI version/revision does not match authoritative source")
	}
	version, err := runStagingCommandCombined(docker, "run", "--rm", "--network", "none", stagingDot6Image, "/app/sub2api", "--version")
	if err != nil || !strings.Contains(version, stagingDot6Version) || !strings.Contains(version, stagingDot6Revision[:7]) {
		t.Fatal("published .6 binary identity differs from expected version/revision")
	}
}

func assertStagingDot6Healthy(t *testing.T, docker string, compose []string, service *Service) {
	t.Helper()
	status, err := service.Status()
	if err != nil || status.InstalledVersion != stagingDot6Version || status.CurrentMigration != 244 || stagingMigration(t, docker, compose) != 244 || stagingApplicationImageReference(t, docker, compose) != stagingDot6Image {
		t.Fatal("source recovery identity differs from immutable .6/schema244")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := service.validateDeployment(ctx, 244); err != nil {
		t.Fatalf("recovered .6 deployment unhealthy: %v", err)
	}
	if stagingQuery(t, docker, compose, "SELECT value FROM recovery_source_sentinel") != "source-before-backup" {
		t.Fatal("source sentinel was not retained")
	}
}

func assertStagingNoDestructiveCommands(t *testing.T, commands []string, noSourceUp bool) {
	t.Helper()
	for _, command := range commands {
		if strings.Contains(command, " pg_restore -U ") || strings.Contains(command, "DROP DATABASE IF EXISTS") || noSourceUp && strings.Contains(command, " up -d --no-deps") && strings.Contains(command, "--force-recreate") {
			t.Fatal("unsafe automatic database restore or source startup was attempted")
		}
	}
}

func assertStagingRecoveryApplicationStopped(t *testing.T, docker string, compose []string) {
	t.Helper()
	output, err := runStagingCommand(docker, append(append([]string(nil), compose...), "ps", "--all", "-q", "sub2api")...)
	if err != nil || strings.TrimSpace(output) == "" {
		t.Fatal("cannot find quiesced application container")
	}
	for _, id := range strings.Fields(output) {
		state, err := runStagingCommand(docker, "inspect", "--format", "{{.State.Running}} {{.HostConfig.RestartPolicy.Name}}", id)
		if err != nil || strings.TrimSpace(state) != "false no" {
			t.Fatal("critical application is running or has an automatic restart policy")
		}
	}
}

func assertStagingRecoveryPrepared(t *testing.T, docker string, compose []string, service *Service, currentSchema int) persistedState {
	t.Helper()
	status := waitForStagingOperation(t, service, updatecontract.UpdaterStateCritical, 3*time.Minute)
	if status.LastAttempt == nil || status.LastAttempt.Result != "succeeded" || status.LastAttempt.Action != updatecontract.OperationPrepareRecovery {
		t.Fatal("rescue preparation did not finish successfully in critical state")
	}
	state := stagingRecoveryState(t, service)
	if state.Recovery == nil || state.Recovery.Phase != recoveryPrepared || state.Recovery.Rescue.SourceMigration != currentSchema || state.Backup.SourceMigration != 244 || state.Backup.SourceDigest != stagingDot6Image {
		t.Fatal("prepared rescue/source schema or digest differs")
	}
	if err := service.validateRecoveryBackup(state.Recovery.Rescue); err != nil {
		t.Fatal(err)
	}
	public, err := service.RecoveryStatus()
	currentVersion := stagingRecoveryCandidateVersion
	if currentSchema == 244 {
		currentVersion = stagingDot6Version
	}
	if err != nil || public == nil || public.CurrentSchema != currentSchema || public.SourceSchema != 244 || public.CurrentVersion != currentVersion || public.SourceVersion != stagingDot6Version || public.OperationID != state.Recovery.OperationID || !strings.Contains(public.Confirmation, state.Recovery.AuthorizationID) || !strings.Contains(public.Confirmation, state.Recovery.Rescue.DatabaseSHA256) {
		t.Fatal("public recovery status is not bound to prepared rescue consent")
	}
	assertStagingRecoveryApplicationStopped(t, docker, compose)
	return state
}

func assertStagingRecoveryRejected(t *testing.T, service *Service, runner *stagingRunner, request updatecontract.OperationRequest) {
	t.Helper()
	before := stagingRecoveryCommands(runner)
	if _, err := service.Start(updatecontract.OperationRecover, request); err == nil {
		t.Fatal("unsafe/stale recovery confirmation accepted")
	}
	if !reflect.DeepEqual(before, stagingRecoveryCommands(runner)) {
		t.Fatal("rejected recovery executed Docker commands")
	}
}

func stagingRecoveryConsent(t *testing.T, service *Service) updatecontract.OperationRequest {
	t.Helper()
	status, err := service.RecoveryStatus()
	if err != nil || status == nil || status.Confirmation == "" {
		t.Fatal("current operation-bound recovery consent unavailable")
	}
	return updatecontract.OperationRequest{Version: stagingDot6Version, Actor: "admin:1", Confirmation: status.Confirmation}
}

func breakStagingRecoveryAudit(t *testing.T, path string) func() {
	t.Helper()
	saved := path + ".staging-saved"
	if err := os.Rename(path, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			if err := os.Remove(path); err != nil {
				t.Errorf("remove owned failed audit path: %v", err)
				return
			}
			if err := os.Rename(saved, path); err != nil {
				t.Errorf("restore staging audit file: %v", err)
			}
		})
	}
	t.Cleanup(restore)
	return restore
}

func assertStagingConcurrentRecovery(t *testing.T, service *Service, runner *stagingRunner, operation updatecontract.Operation, command string) {
	t.Helper()
	oldConsent := stagingRecoveryConsent(t, service)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	service.runner = &stagingRecoveryFaultRunner{stagingRunner: runner, blockCommand: command, entered: entered, release: release}
	requestHostStagingOperation(t, service, operation, stagingDot6Version)
	select {
	case <-entered:
	case <-time.After(time.Minute):
		unblock()
		t.Fatal("real recovery command did not reach concurrency checkpoint")
	}
	before := stagingRecoveryCommands(runner)
	for _, competing := range []struct {
		action  updatecontract.Operation
		request updatecontract.OperationRequest
	}{
		{updatecontract.OperationPrepareRecovery, updatecontract.OperationRequest{Version: stagingDot6Version, Actor: "admin:1", Confirmation: "PREPARE RECOVERY " + stagingDot6Version}},
		{updatecontract.OperationRecover, oldConsent},
		{updatecontract.OperationPrepare, updatecontract.OperationRequest{Version: stagingRecoveryCandidateVersion, Actor: "admin:1"}},
		{updatecontract.OperationInstall, updatecontract.OperationRequest{Version: stagingRecoveryCandidateVersion, Actor: "admin:1", Confirmation: "INSTALL " + stagingRecoveryCandidateVersion}},
		{updatecontract.OperationRollback, updatecontract.OperationRequest{Version: stagingDot6Version, Actor: "admin:1", Confirmation: "ROLLBACK " + stagingDot6Version}},
	} {
		if _, err := service.Start(competing.action, competing.request); !errors.Is(err, ErrOperationBusy) {
			t.Fatalf("concurrent %s did not fail at operation lock: %v", competing.action, err)
		}
	}
	if !reflect.DeepEqual(before, stagingRecoveryCommands(runner)) {
		t.Fatal("concurrent rejected operation ran Docker commands")
	}
	public, err := service.RecoveryStatus()
	if err != nil || public == nil || public.Confirmation != "" {
		t.Fatal("busy recovery exposed usable consent")
	}
	unblock()
	want := updatecontract.UpdaterStateCritical
	if operation == updatecontract.OperationRecover {
		want = updatecontract.UpdaterStateSucceeded
	}
	status := waitForStagingOperation(t, service, want, 5*time.Minute)
	service.runner = runner
	if status.LastAttempt == nil || status.LastAttempt.Result != "succeeded" {
		t.Fatal("held legitimate recovery operation did not complete")
	}
	assertStagingRecoveryRejected(t, service, runner, oldConsent)
	t.Logf("real concurrent %s rejected competing Stage1/Stage2/prepare/install/rollback without Docker commands", operation)
}

func assertStagingCurrentSentinel(t *testing.T, docker string, compose []string) {
	t.Helper()
	if stagingMigration(t, docker, compose) != stagingCandidateMigration || stagingQuery(t, docker, compose, "SELECT value FROM recovery_post_success_sentinel") != "must-be-rescued" {
		t.Fatal("failed/rejected recovery changed current candidate schema or post-success data")
	}
}

func assertStagingMigratedMultiplier(t *testing.T, docker string, compose []string, database string) {
	t.Helper()
	query := "SELECT model_pricing->0->'reasoning_effort_multipliers'->>'max' FROM groups WHERE name='" + stagingLegacyPricingGroup + "'"
	args := append(append([]string(nil), compose...), "exec", "-T", "postgres", "psql", "-U", "sub2api", "-d", database, "-Atc", query)
	output, err := runStagingCommand(docker, args...)
	if err != nil || strings.TrimSpace(output) != "3" {
		t.Fatal("migration247 did not preserve legacy group max multiplier3")
	}
}

func verifyStagingDot6PricingIncompatibility(t *testing.T, docker string, compose []string, service *Service, rescue *backupMetadata) {
	t.Helper()
	const database = "dot6_pricing_negative"
	args := append(append([]string(nil), compose...), "exec", "-T", "postgres", "createdb", "-U", "sub2api", database)
	if _, err := runStagingCommand(docker, args...); err != nil {
		t.Fatal("create isolated negative compatibility DB")
	}
	t.Cleanup(func() {
		_, _ = runStagingCommand(docker, append(append([]string(nil), compose...), "exec", "-T", "postgres", "dropdb", "-U", "sub2api", "--if-exists", database)...)
	})
	dump, err := openManagedFile(rescue.DatabaseBackup, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	args = append(append([]string(nil), compose...), "exec", "-T", "postgres", "pg_restore", "-U", "sub2api", "-d", database, "--no-owner", "--no-privileges", "--exit-on-error")
	command := exec.Command(docker, args...)
	command.Stdin = dump
	err = command.Run()
	_ = dump.Close()
	if err != nil {
		t.Fatal("restore rescue to isolated negative compatibility DB")
	}
	assertStagingMigratedMultiplier(t, docker, compose, database)
	// Synthetic acknowledgement fixture exists only in this disposable rescue copy.
	query := `INSERT INTO settings (key, value, updated_at)
		SELECT 'admin_compliance_acknowledgement:' || id,
			jsonb_build_object('version', 'v2026.06.10', 'admin_user_id', id,
				'document_zh', 'docs/legal/admin-compliance.zh.md',
				'document_en', 'docs/legal/admin-compliance.en.md',
				'user_agent', 'synthetic-disposable-compatibility-fixture',
				'accepted_at', '2026-10-02T00:00:00Z')::text, NOW()
		FROM users WHERE email='admin@sub2api.local' AND role='admin' AND deleted_at IS NULL
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=EXCLUDED.updated_at
		RETURNING value::jsonb->>'admin_user_id'`
	adminID, err := runStagingCommand(docker, append(append([]string(nil), compose...), "exec", "-T", "postgres", "psql", "-U", "sub2api", "-d", database, "-Atq", "-v", "ON_ERROR_STOP=1", "-c", query)...)
	if err != nil {
		t.Fatal("seed isolated synthetic admin compliance fixture")
	}
	syntheticAdminID, err := strconv.ParseInt(strings.TrimSpace(adminID), 10, 64)
	if err != nil || syntheticAdminID < 1 {
		t.Fatal("isolated synthetic admin compliance fixture did not identify exactly one admin")
	}
	postgres, err := runStagingCommand(docker, append(append([]string(nil), compose...), "ps", "-q", "postgres")...)
	if err != nil || strings.TrimSpace(postgres) == "" {
		t.Fatal("isolated compatibility postgres unavailable")
	}
	networkJSON, err := runStagingCommand(docker, "inspect", "--format", "{{json .NetworkSettings.Networks}}", strings.TrimSpace(postgres))
	var networks map[string]json.RawMessage
	if err != nil || json.Unmarshal([]byte(networkJSON), &networks) != nil || len(networks) != 1 {
		t.Fatal("cannot determine disposable compatibility network")
	}
	var network string
	for name := range networks {
		network = name
	}
	container := "sub2api-dot6-negative-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	port := freeStagingPort(t)
	args = []string{"run", "-d", "--name", container, "--network", network, "--restart=no", "--tmpfs", "/app/data:rw,mode=1777", "-p", "127.0.0.1:" + strconv.Itoa(port) + ":8080"}
	for _, env := range []string{
		"AUTO_SETUP=true", "SERVER_HOST=0.0.0.0", "SERVER_PORT=8080", "RUN_MODE=standard",
		"DATABASE_HOST=postgres", "DATABASE_PORT=5432", "DATABASE_USER=sub2api", "DATABASE_PASSWORD=synthetic-staging-password", "DATABASE_DBNAME=" + database, "DATABASE_SSLMODE=disable",
		"REDIS_HOST=redis", "REDIS_PORT=6379", "REDIS_DB=15", "ADMIN_EMAIL=admin@sub2api.local", "ADMIN_PASSWORD=synthetic-staging-admin-password",
		"JWT_SECRET=synthetic-staging-jwt-secret-with-at-least-32-bytes", "TOTP_ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000",
	} {
		args = append(args, "-e", env)
	}
	args = append(args, stagingDot6Image)
	if _, err := runStagingCommand(docker, args...); err != nil {
		t.Fatal("start immutable .6 against isolated negative compatibility DB")
	}
	t.Cleanup(func() { _, _ = runStagingCommand(docker, "rm", "-f", container) })
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(3 * time.Minute)
	healthy := false
	for time.Now().Before(deadline) {
		response, err := client.Get(base + "/health")
		if err == nil {
			_ = response.Body.Close()
			healthy = response.StatusCode == http.StatusOK
		}
		if healthy {
			break
		}
		time.Sleep(time.Second)
	}
	if !healthy {
		t.Fatal("isolated .6 compatibility application did not become healthy")
	}
	login := stagingCompatibilityHTTP(t, client, http.MethodPost, base+"/api/v1/auth/login", "", map[string]string{"email": "admin@sub2api.local", "password": "synthetic-staging-admin-password"})
	var auth struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(login, &auth) != nil || auth.AccessToken == "" {
		t.Fatal("isolated .6 admin login missing token")
	}
	compliance := stagingCompatibilityHTTP(t, client, http.MethodGet, base+"/api/v1/admin/compliance", auth.AccessToken, nil)
	var acknowledgement struct {
		Required        bool `json:"required"`
		Acknowledgement struct {
			Version     string    `json:"version"`
			AdminUserID int64     `json:"admin_user_id"`
			AcceptedAt  time.Time `json:"accepted_at"`
		} `json:"acknowledgement"`
	}
	if json.Unmarshal(compliance, &acknowledgement) != nil || acknowledgement.Required || acknowledgement.Acknowledgement.Version != "v2026.06.10" || acknowledgement.Acknowledgement.AdminUserID != syntheticAdminID || acknowledgement.Acknowledgement.AcceptedAt.IsZero() {
		t.Fatal("immutable .6 did not recognize isolated synthetic compliance fixture")
	}
	version := stagingCompatibilityHTTP(t, client, http.MethodGet, base+"/api/v1/admin/system/version", auth.AccessToken, nil)
	var identity struct {
		Version string `json:"version"`
		Commit  string `json:"git_commit"`
	}
	if json.Unmarshal(version, &identity) != nil || identity.Version != stagingDot6Version || !strings.HasPrefix(stagingDot6Revision, identity.Commit) || len(identity.Commit) < 7 {
		t.Fatal("isolated .6 API returned incorrect version/revision")
	}
	query = "SELECT id FROM groups WHERE name='" + stagingLegacyPricingGroup + "'"
	groupID, err := runStagingCommand(docker, append(append([]string(nil), compose...), "exec", "-T", "postgres", "psql", "-U", "sub2api", "-d", database, "-Atc", query)...)
	if err != nil {
		t.Fatal("isolated pricing group unavailable")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(groupID), 10, 64)
	if err != nil || id < 1 {
		t.Fatal("isolated pricing group has invalid identity")
	}
	url := base + "/api/v1/admin/groups/" + strconv.FormatInt(id, 10)
	group := stagingCompatibilityHTTP(t, client, http.MethodGet, url, auth.AccessToken, nil)
	var read struct {
		ModelPricing []json.RawMessage `json:"model_pricing"`
	}
	if json.Unmarshal(group, &read) != nil || len(read.ModelPricing) != 1 {
		t.Fatal("isolated .6 group GET did not return pricing entry")
	}
	for _, entry := range read.ModelPricing {
		var fields map[string]json.RawMessage
		if json.Unmarshal(entry, &fields) != nil {
			t.Fatal("invalid isolated pricing response")
		}
		if _, retained := fields["reasoning_effort_multipliers"]; retained {
			t.Fatal("expected .6 negative compatibility failure did not reproduce at GET boundary")
		}
	}
	stagingCompatibilityHTTP(t, client, http.MethodPut, url, auth.AccessToken, map[string]any{"model_pricing": read.ModelPricing})
	query = "SELECT COALESCE(model_pricing->0->'reasoning_effort_multipliers'->>'max', 'lost') FROM groups WHERE name='" + stagingLegacyPricingGroup + "'"
	output, err := runStagingCommand(docker, append(append([]string(nil), compose...), "exec", "-T", "postgres", "psql", "-U", "sub2api", "-d", database, "-Atc", query)...)
	if err != nil || strings.TrimSpace(output) != "lost" {
		t.Fatal("expected .6 negative pricing round-trip failure did not reproduce")
	}
	assertStagingMigratedMultiplier(t, docker, compose, "sub2api")
	assertStagingCurrentSentinel(t, docker, compose)
	if err := service.validateRecoveryBackup(rescue); err != nil {
		t.Fatal(err)
	}
	t.Logf("FAIL_EXPECTED: exact %s revision %s loses migrated reasoning max multiplier3 on isolated schema%d GET/PUT; managed DB and rescue unchanged", stagingDot6Version, stagingDot6Revision, stagingCandidateMigration)
	if _, err := runStagingCommand(docker, "rm", "-f", container); err != nil {
		t.Fatal("remove owned isolated .6 negative container")
	}
	if _, err := runStagingCommand(docker, append(append([]string(nil), compose...), "exec", "-T", "postgres", "dropdb", "-U", "sub2api", database)...); err != nil {
		t.Fatal("remove owned isolated .6 negative DB")
	}
}

func stagingCompatibilityHTTP(t *testing.T, client *http.Client, method, url, token string, body any) json.RawMessage {
	t.Helper()
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		input = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, url, input)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("isolated compatibility HTTP %s failed", method)
	}
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope) != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		t.Fatalf("isolated compatibility HTTP %s rejected: %d", method, response.StatusCode)
	}
	return envelope.Data
}
