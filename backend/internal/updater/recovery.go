package updater

import (
	"context"
	"fmt"
	"io"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

func recoveryConfirmation(state persistedState) string {
	r := state.Recovery
	if r == nil || r.Phase == recoveryPreparing || r.Phase == recoveryCompleted || r.Phase == recoverySourceStarting {
		return ""
	}
	return fmt.Sprintf("RESTORE PREUPDATE DATABASE %s USING RESCUE %s %s ACK %s",
		state.Backup.SourceVersion, r.OperationID, r.Rescue.DatabaseSHA256, r.AuthorizationID)
}

func (s *Service) RecoveryStatus() (*updatecontract.RecoveryStatus, error) {
	state, err := s.store.load(s.policy.InitialInstalledVersion, s.policy.InitialMigration, Version)
	if err != nil {
		return nil, err
	}
	if state.Backup == nil {
		return nil, nil
	}
	status := &updatecontract.RecoveryStatus{
		SourceUpdateID: state.Backup.UpdateID, SourceVersion: state.Backup.SourceVersion,
		CurrentVersion: state.Status.InstalledVersion, CurrentSchema: state.Status.CurrentMigration,
		SourceSchema: state.Backup.SourceMigration,
	}
	if state.Exposure == exposurePossible && state.Prepared != nil {
		image, err := deploymentImageFromEnvironment(s.policy.EnvironmentFile)
		if err != nil {
			return nil, fmt.Errorf("current deployment identity unavailable")
		}
		if image == state.Prepared.ImmutableImage() {
			status.CurrentVersion = state.Prepared.ReworkVersion
		}
	}
	if r := state.Recovery; r != nil {
		status.OperationID, status.Phase, status.RescueSHA256 = r.OperationID, r.Phase, r.Rescue.DatabaseSHA256
		status.CurrentVersion, status.CurrentSchema = r.Rescue.SourceVersion, r.Rescue.SourceMigration
		if !state.Status.Busy {
			status.Confirmation = recoveryConfirmation(state)
		}
	}
	return status, nil
}

func (s *Service) auditEvent(state *persistedState, event string) error {
	if state.Status.LastAttempt == nil {
		return fmt.Errorf("missing operation identity")
	}
	summary := *state.Status.LastAttempt
	summary.Event = event
	summary.FinishedAt = s.now().UTC()
	return s.store.audit(summary)
}

func (s *Service) validateArchive(ctx context.Context, backup *backupMetadata) error {
	dump, err := openManagedFile(backup.DatabaseBackup, 0, true)
	if err != nil {
		return fmt.Errorf("backup archive unavailable")
	}
	defer func() { _ = dump.Close() }()
	if err := s.runDocker(ctx, dump, io.Discard, s.composeArgs("exec", "-T", s.policy.DatabaseService,
		"pg_restore", "--list")...); err != nil {
		return fmt.Errorf("backup archive validation failed")
	}
	return nil
}

func (s *Service) prepareRecovery(ctx context.Context, summary *updatecontract.OperationSummary, state *persistedState) error {
	state.Status.State = updatecontract.UpdaterStateCritical
	if err := s.auditEvent(state, "recovery_preparation_started"); err != nil {
		return fmt.Errorf("recovery audit failed")
	}
	if err := s.validateRecoveryBackup(state.Backup); err != nil {
		return fmt.Errorf("recorded recovery backup is invalid")
	}
	if err := s.preflightRecovery(ctx); err != nil {
		return fmt.Errorf("recovery preflight failed")
	}
	if err := s.quiesceApplication(ctx); err != nil {
		return fmt.Errorf("application quiescence could not be verified")
	}
	fence, err := s.applicationFence(ctx)
	if err != nil {
		return err
	}
	if err := s.auditEvent(state, "candidate_quiesced"); err != nil {
		return fmt.Errorf("recovery audit failed")
	}
	// Reuse the original rescue while retrying a partial restore. Never dump a
	// partially recreated database over the last known current-data snapshot.
	if r := state.Recovery; r != nil && (r.Phase == recoveryRestoring || r.Phase == recoveryDatabaseRestored) {
		if err := s.validateRecoveryBackup(r.Rescue); err != nil {
			return fmt.Errorf("recorded rescue backup is invalid")
		}
		r.AuthorizationID = summary.OperationID
		if err := s.store.save(*state); err != nil {
			return fmt.Errorf("recovery state persistence failed")
		}
		return nil
	}
	migration, err := s.currentMigration(ctx)
	if err != nil {
		return fmt.Errorf("current database unavailable for rescue backup")
	}
	currentVersion := state.Status.InstalledVersion
	image, err := deploymentImageFromEnvironment(s.policy.EnvironmentFile)
	if err != nil {
		return fmt.Errorf("current deployment identity unavailable")
	}
	if image == state.Backup.SourceDigest || image == state.Backup.SourceImage {
		currentVersion = state.Backup.SourceVersion
	} else if state.Exposure == exposurePossible && state.Prepared != nil && image == state.Prepared.ImmutableImage() {
		currentVersion = state.Prepared.ReworkVersion
	}
	rescue, err := s.createBackup(ctx, summary.OperationID, currentVersion, state.Backup.SourceVersion, migration, state.Backup.UpdateID)
	if err != nil {
		return fmt.Errorf("rescue backup creation failed")
	}
	if err := s.validateRecoveryBackup(rescue); err != nil {
		return fmt.Errorf("rescue backup checksum validation failed")
	}
	if err := s.validateArchive(ctx, rescue); err != nil {
		return err
	}
	if err := s.auditEvent(state, "rescue_backup_created_and_checksum_verified"); err != nil {
		return fmt.Errorf("recovery audit failed")
	}
	state.Recovery = &recoveryMetadata{
		OperationID: summary.OperationID, SourceUpdateID: state.Backup.UpdateID,
		AuthorizationID: summary.OperationID, Phase: recoveryPrepared, Rescue: rescue,
		Quiesced: fence,
	}
	if err := s.store.save(*state); err != nil {
		return fmt.Errorf("recovery state persistence failed")
	}
	return nil
}
