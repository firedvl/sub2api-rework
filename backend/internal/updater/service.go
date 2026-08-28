package updater

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"golang.org/x/mod/semver"
)

const Version = "1.0.0"

var actorPattern = regexp.MustCompile(`^admin:[1-9][0-9]*$`)

type Service struct {
	policy     Policy
	runner     CommandRunner
	fetcher    ManifestFetcher
	store      *stateStore
	healthHTTP *http.Client
	now        func() time.Time
	statusMu   sync.RWMutex
	statusErr  *updatecontract.UpdaterStatus
}

func NewService(policy Policy, runner CommandRunner, fetcher ManifestFetcher) (*Service, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := validateManagedPaths(policy); err != nil {
		return nil, fmt.Errorf("validate updater paths: %w", err)
	}
	if runner == nil {
		runner = ExecRunner{Directory: policy.DeploymentDirectory}
	}
	if fetcher == nil {
		fetcher = NewHTTPManifestFetcher(policy.ManifestBaseURL)
	}
	service := &Service{
		policy: policy, runner: runner, fetcher: fetcher, store: newStateStore(policy), now: time.Now,
		healthHTTP: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	state, err := service.store.load(policy.InitialInstalledVersion, policy.InitialMigration, Version)
	if err != nil {
		return nil, fmt.Errorf("load updater state: %w", err)
	}
	if state.Backup != nil {
		if err := service.validateBackupMetadata(state.Backup); err != nil {
			return nil, fmt.Errorf("validate updater backup: %w", err)
		}
	}
	if state.Status.Busy {
		state.Status.Busy = false
		switch state.Status.State {
		case updatecontract.UpdaterStateInstalling, updatecontract.UpdaterStateRollingBack, updatecontract.UpdaterStateCritical:
			state.Status.State = updatecontract.UpdaterStateCritical
		default:
			state.Status.State = updatecontract.UpdaterStateFailed
		}
		state.Status.LastError = "The previous updater operation was interrupted. Review the deployment before retrying."
	}
	state.Status.UpdaterVersion = Version
	state.Status.Healthy = true
	if err := service.store.save(state); err != nil {
		return nil, fmt.Errorf("initialize updater state: %w", err)
	}
	return service, nil
}

func (s *Service) Status() (updatecontract.UpdaterStatus, error) {
	s.statusMu.RLock()
	if s.statusErr != nil {
		status := *s.statusErr
		s.statusMu.RUnlock()
		return status, nil
	}
	s.statusMu.RUnlock()
	state, err := s.store.load(s.policy.InitialInstalledVersion, s.policy.InitialMigration, Version)
	if err != nil {
		return updatecontract.UpdaterStatus{}, err
	}
	state.Status.Healthy = true
	return state.Status, nil
}

func (s *Service) Start(action updatecontract.Operation, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	if err := validateOperationRequest(action, request); err != nil {
		return nil, err
	}
	lock, err := tryOperationLock(s.policy.LockPath)
	if err != nil {
		return nil, err
	}
	state, err := s.store.load(s.policy.InitialInstalledVersion, s.policy.InitialMigration, Version)
	if err != nil {
		lock.release()
		return nil, err
	}
	if state.Status.Busy {
		lock.release()
		return nil, ErrOperationBusy
	}
	if state.Status.State == updatecontract.UpdaterStateCritical && action != updatecontract.OperationRollback {
		lock.release()
		return nil, fmt.Errorf("updater requires recovery before another prepare or install")
	}
	if action == updatecontract.OperationRollback {
		if state.Backup == nil || state.Backup.SourceVersion != request.Version || state.Status.RollbackVersion != request.Version {
			lock.release()
			return nil, fmt.Errorf("requested version is not the recorded rollback target")
		}
		if err := s.validateBackupMetadata(state.Backup); err != nil {
			lock.release()
			return nil, fmt.Errorf("recorded rollback backup is invalid")
		}
	} else if updatecontract.CompareRework(request.Version, state.Status.InstalledVersion) <= 0 {
		lock.release()
		return nil, fmt.Errorf("target version must be newer than the installed version")
	}

	operationID, err := newOperationID()
	if err != nil {
		lock.release()
		return nil, err
	}
	now := s.now().UTC()
	summary := updatecontract.OperationSummary{
		OperationID: operationID, Action: action, Actor: request.Actor,
		SourceVersion: state.Status.InstalledVersion, TargetVersion: request.Version,
		StartedAt: now, Result: "running",
	}
	state.Status.Busy = true
	state.Status.LastError = ""
	state.Status.LastAttempt = &summary
	if state.Status.State != updatecontract.UpdaterStateCritical {
		state.Status.State = operationState(action)
	}
	if err := s.store.save(state); err != nil {
		lock.release()
		return nil, err
	}

	go s.runOperation(action, request, summary, state, lock)
	return &updatecontract.OperationAccepted{OperationID: operationID, Action: action, State: "accepted"}, nil
}

func validateOperationRequest(action updatecontract.Operation, request updatecontract.OperationRequest) error {
	if !updatecontract.IsReworkVersion(request.Version) || !actorPattern.MatchString(request.Actor) || len(request.Actor) > 64 {
		return fmt.Errorf("invalid updater request")
	}
	switch action {
	case updatecontract.OperationPrepare:
		if request.Confirmation != "" {
			return fmt.Errorf("prepare does not accept confirmation data")
		}
	case updatecontract.OperationInstall:
		if request.Confirmation != "INSTALL "+request.Version {
			return fmt.Errorf("install confirmation mismatch")
		}
	case updatecontract.OperationRollback:
		if request.Confirmation != "ROLLBACK "+request.Version {
			return fmt.Errorf("rollback confirmation mismatch")
		}
	default:
		return fmt.Errorf("unsupported updater operation")
	}
	return nil
}

func operationState(action updatecontract.Operation) updatecontract.UpdaterState {
	switch action {
	case updatecontract.OperationPrepare:
		return updatecontract.UpdaterStatePreparing
	case updatecontract.OperationInstall:
		return updatecontract.UpdaterStateInstalling
	case updatecontract.OperationRollback:
		return updatecontract.UpdaterStateRollingBack
	default:
		return updatecontract.UpdaterStateFailed
	}
}

func (s *Service) runOperation(
	action updatecontract.Operation,
	request updatecontract.OperationRequest,
	summary updatecontract.OperationSummary,
	state persistedState,
	lock *operationLock,
) {
	defer lock.release()
	ctx, cancel := context.WithTimeout(context.Background(), s.policy.operationTimeout())
	defer cancel()

	var operationErr error
	switch action {
	case updatecontract.OperationPrepare:
		operationErr = s.prepare(ctx, request.Version, &state)
	case updatecontract.OperationInstall:
		operationErr = s.install(ctx, request.Version, &summary, &state)
	case updatecontract.OperationRollback:
		operationErr = s.rollback(ctx, request.Version, &state)
	}
	s.finish(summary, operationErr, &state)
}

func (s *Service) prepare(ctx context.Context, version string, state *persistedState) error {
	manifest, err := s.approvedManifest(ctx, version, state.Status.CurrentMigration)
	if err != nil {
		return err
	}
	if _, err := s.preflight(ctx, manifest, true); err != nil {
		return err
	}
	if err := s.pullAndVerify(ctx, manifest); err != nil {
		return err
	}
	state.Prepared = manifest
	state.Status.PreparedVersion = manifest.ReworkVersion
	state.Status.State = updatecontract.UpdaterStatePrepared
	return nil
}

func (s *Service) install(ctx context.Context, version string, summary *updatecontract.OperationSummary, state *persistedState) error {
	manifest, err := s.approvedManifest(ctx, version, state.Status.CurrentMigration)
	if err != nil {
		return err
	}
	if state.Prepared == nil || state.Prepared.ReworkVersion != manifest.ReworkVersion ||
		state.Prepared.ImageDigest != manifest.ImageDigest || state.Prepared.GitSHA != manifest.GitSHA {
		return fmt.Errorf("release must be prepared again before installation")
	}
	currentMigration, err := s.preflight(ctx, manifest, true)
	if err != nil {
		return err
	}
	backup, err := s.createBackup(ctx, summary.OperationID, state.Status.InstalledVersion, version, currentMigration)
	if err != nil {
		return fmt.Errorf("backup failed")
	}
	state.Backup = backup
	state.Status.RollbackVersion = backup.SourceVersion
	if err := s.store.save(*state); err != nil {
		return fmt.Errorf("persist backup metadata: %w", err)
	}
	if err := s.pullAndVerify(ctx, manifest); err != nil {
		return err
	}

	migrationAttempted := false
	err = s.stopApplication(ctx)
	if err == nil {
		err = rewriteEnvironmentImage(s.policy.EnvironmentFile, manifest.ImmutableImage())
		if err != nil {
			err = fmt.Errorf("activate approved image: %w", err)
		}
	}
	if err == nil {
		migrationAttempted = true
		err = s.runMigrations(ctx)
	}
	if err == nil {
		err = s.startApplication(ctx)
	}
	if err == nil {
		err = s.validateDeployment(ctx, manifest.MigrationMax)
	}
	if err != nil {
		recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), s.policy.operationTimeout())
		rollbackErr := s.restoreImmediate(recoveryCtx, backup, migrationAttempted)
		cancelRecovery()
		if rollbackErr != nil {
			summary.RollbackResult = "failed"
			state.Status.State = updatecontract.UpdaterStateCritical
			return fmt.Errorf("installation failed and automatic rollback failed")
		}
		summary.RollbackResult = "succeeded"
		state.Status.InstalledVersion = backup.SourceVersion
		state.Status.CurrentMigration = backup.SourceMigration
		return fmt.Errorf("installation failed; automatic rollback succeeded")
	}
	state.Prepared = nil
	state.Status.PreparedVersion = ""
	state.Status.InstalledVersion = manifest.ReworkVersion
	state.Status.CurrentMigration = manifest.MigrationMax
	state.Status.State = updatecontract.UpdaterStateSucceeded
	return nil
}

func (s *Service) rollback(ctx context.Context, version string, state *persistedState) error {
	backup := state.Backup
	if backup == nil || backup.SourceVersion != version {
		return fmt.Errorf("recorded rollback backup is unavailable")
	}
	currentMigration, err := s.preflightRollback(ctx)
	if err != nil {
		return err
	}
	if currentMigration != backup.SourceMigration {
		return fmt.Errorf("manual rollback blocked because database migrations changed")
	}
	if err := s.restoreApplication(ctx, backup); err != nil {
		state.Status.State = updatecontract.UpdaterStateCritical
		return fmt.Errorf("manual rollback failed")
	}
	state.Status.InstalledVersion = backup.SourceVersion
	state.Status.CurrentMigration = backup.SourceMigration
	state.Status.RollbackVersion = ""
	state.Status.State = updatecontract.UpdaterStateSucceeded
	return nil
}

func (s *Service) approvedManifest(ctx context.Context, version string, currentMigration int) (*updatecontract.Manifest, error) {
	data, err := s.fetcher.Fetch(ctx, version)
	if err != nil {
		return nil, err
	}
	manifest, err := updatecontract.Parse(data, s.policy.TrustedImageRepository)
	if err != nil {
		return nil, fmt.Errorf("release manifest validation failed")
	}
	if manifest.ReworkVersion != version || manifest.Compatibility != updatecontract.CompatibilityApproved {
		return nil, fmt.Errorf("release is not approved")
	}
	if semver.Compare("v"+Version, "v"+manifest.MinimumUpdaterVersion) < 0 {
		return nil, fmt.Errorf("release requires a newer updater")
	}
	if currentMigration < manifest.MigrationMin || currentMigration > manifest.MigrationMax {
		return nil, fmt.Errorf("installed migration state is outside the release compatibility range")
	}
	return manifest, nil
}

func (s *Service) finish(summary updatecontract.OperationSummary, operationErr error, state *persistedState) {
	summary.FinishedAt = s.now().UTC()
	state.Status.Busy = false
	if operationErr != nil {
		summary.Result = "failed"
		summary.Error = safeOperationError(operationErr)
		state.Status.LastError = summary.Error
		if state.Status.State != updatecontract.UpdaterStateCritical {
			state.Status.State = updatecontract.UpdaterStateFailed
		}
	} else {
		summary.Result = "succeeded"
		state.Status.LastError = ""
		if summary.Action == updatecontract.OperationPrepare {
			state.Status.State = updatecontract.UpdaterStatePrepared
		} else {
			state.Status.State = updatecontract.UpdaterStateSucceeded
		}
	}
	state.Status.LastAttempt = &summary
	if summary.Action == updatecontract.OperationRollback {
		state.Status.LastRollback = &summary
	}
	if err := s.store.audit(summary); err != nil {
		state.Status.State = updatecontract.UpdaterStateCritical
		state.Status.LastError = "Updater audit persistence failed."
	}
	if err := s.store.save(*state); err != nil {
		failed := state.Status
		failed.Healthy = false
		failed.Busy = false
		failed.State = updatecontract.UpdaterStateCritical
		failed.LastError = "Updater state persistence failed."
		s.statusMu.Lock()
		s.statusErr = &failed
		s.statusMu.Unlock()
		return
	}
	s.statusMu.Lock()
	s.statusErr = nil
	s.statusMu.Unlock()
}

func newOperationID() (string, error) {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return "upd-" + hex.EncodeToString(data), nil
}
