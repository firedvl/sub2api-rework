package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	maxCommandOutput = 64 * 1024
	maxAuditBytes    = 1024 * 1024
	maxStateBytes    = 1024 * 1024
)

var ErrOperationBusy = errors.New("another updater operation is running")

type CommandRunner interface {
	Run(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error
}

type ExecRunner struct{ Directory string }

func (r ExecRunner) Run(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = r.Directory
	command.Stdin = stdin
	command.Stdout = stdout
	var stderr boundedBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%s exited with status %d", filepath.Base(name), exitErr.ExitCode())
		}
		return fmt.Errorf("%s failed", filepath.Base(name))
	}
	return nil
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	if b.Len() < maxCommandOutput {
		remaining := maxCommandOutput - b.Len()
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.Buffer.Write(data)
	}
	return original, nil
}

type backupMetadata struct {
	UpdateID        string    `json:"update_id"`
	Directory       string    `json:"directory"`
	DatabaseBackup  string    `json:"database_backup"`
	EnvironmentCopy string    `json:"environment_copy"`
	ComposeCopy     string    `json:"compose_copy"`
	SourceVersion   string    `json:"source_version"`
	TargetVersion   string    `json:"target_version"`
	SourceImage     string    `json:"source_image"`
	SourceDigest    string    `json:"source_digest"`
	SourceMigration int       `json:"source_migration"`
	CreatedAt       time.Time `json:"created_at"`
}

type persistedState struct {
	SchemaVersion int                          `json:"schema_version"`
	Status        updatecontract.UpdaterStatus `json:"status"`
	Prepared      *updatecontract.Manifest     `json:"prepared,omitempty"`
	Backup        *backupMetadata              `json:"backup,omitempty"`
}

type stateStore struct {
	path      string
	auditPath string
	mu        sync.Mutex
}

func newStateStore(policy Policy) *stateStore {
	return &stateStore{path: policy.StatePath, auditPath: policy.AuditPath}
}

func validateManagedPaths(policy Policy) error {
	if err := validateManagedDirectory(policy.DeploymentDirectory, false); err != nil {
		return fmt.Errorf("deployment directory: %w", err)
	}
	if _, err := validateManagedRegularFile(policy.ComposeFile, 2*1024*1024, false); err != nil {
		return fmt.Errorf("compose file: %w", err)
	}
	if _, err := validateManagedRegularFile(policy.EnvironmentFile, 2*1024*1024, true); err != nil {
		return fmt.Errorf("deployment environment: %w", err)
	}
	for _, directory := range []string{filepath.Dir(policy.StatePath), filepath.Dir(policy.AuditPath)} {
		if err := ensureManagedDirectory(directory, 0700, true); err != nil {
			return err
		}
	}
	if err := ensureManagedDirectory(filepath.Dir(policy.LockPath), 0700, false); err != nil {
		return err
	}
	if err := ensureManagedDirectory(filepath.Dir(policy.SocketPath), 0750, false); err != nil {
		return err
	}
	if err := ensureManagedDirectory(policy.BackupDirectory, 0700, true); err != nil {
		return err
	}
	for _, path := range []string{policy.StatePath, policy.AuditPath, policy.LockPath} {
		if _, err := validateManagedRegularFile(path, 0, true); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *stateStore) load(initialVersion string, initialMigration int, updaterVersion string) (persistedState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := readManagedFile(s.path, maxStateBytes, true)
	if errors.Is(err, os.ErrNotExist) {
		return persistedState{
			SchemaVersion: 1,
			Status: updatecontract.UpdaterStatus{
				SchemaVersion: 1, UpdaterVersion: updaterVersion, Healthy: true,
				State: updatecontract.UpdaterStateIdle, InstalledVersion: initialVersion,
				CurrentMigration: initialMigration, UpdatedAt: time.Now().UTC(),
			},
		}, nil
	}
	if err != nil {
		return persistedState{}, err
	}
	var state persistedState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || state.SchemaVersion != 1 || state.Status.SchemaVersion != 1 {
		return persistedState{}, fmt.Errorf("invalid updater state")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return persistedState{}, fmt.Errorf("invalid updater state")
	}
	return state, nil
}

func (s *stateStore) save(state persistedState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.Status.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(s.path, append(data, '\n'), 0600)
}

func (s *stateStore) audit(summary updatecontract.OperationSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ensureManagedDirectory(filepath.Dir(s.auditPath), 0700, true); err != nil {
		return err
	}
	if info, err := validateManagedRegularFile(s.auditPath, 0, true); err == nil && info.Size() >= maxAuditBytes {
		if _, rotatedErr := validateManagedRegularFile(s.auditPath+".1", 0, true); rotatedErr != nil && !errors.Is(rotatedErr, os.ErrNotExist) {
			return rotatedErr
		}
		if err := os.Remove(s.auditPath + ".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(s.auditPath, s.auditPath+".1"); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	fd, err := syscall.Open(s.auditPath, syscall.O_CREAT|syscall.O_APPEND|syscall.O_WRONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), s.auditPath)
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := validateManagedFileInfo(info, true); err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := ensureManagedDirectory(directory, 0700, false); err != nil {
		return err
	}
	if _, err := validateManagedRegularFile(path, 0, mode.Perm()&077 == 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".updater-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

type operationLock struct{ file *os.File }

func tryOperationLock(path string) (*operationLock, error) {
	if err := ensureManagedDirectory(filepath.Dir(path), 0700, false); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := validateManagedFileInfo(info, true); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("invalid updater lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrOperationBusy
		}
		return nil, err
	}
	return &operationLock{file: file}, nil
}

func ensureManagedDirectory(path string, mode os.FileMode, private bool) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return validateManagedDirectory(path, private)
}

func validateManagedDirectory(path string, private bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed path must be a directory")
	}
	if err := validateManagedOwner(info); err != nil {
		return err
	}
	if info.Mode().Perm()&022 != 0 || private && info.Mode().Perm()&077 != 0 {
		return fmt.Errorf("managed directory permissions are too broad")
	}
	return validateManagedAncestors(filepath.Dir(path))
}

func validateManagedAncestors(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid != 0 {
				return fmt.Errorf("managed path has an unsafe parent directory")
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil || resolved == current {
				return fmt.Errorf("managed path has an unsafe parent directory")
			}
			if err := validateManagedAncestors(resolved); err != nil {
				return err
			}
		} else if !info.IsDir() || managedAncestorIsWritable(info) {
			return fmt.Errorf("managed path has an unsafe parent directory")
		} else if err := validateManagedOwner(info); err != nil {
			return err
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func managedAncestorIsWritable(info os.FileInfo) bool {
	if info.Mode().Perm()&022 == 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || stat.Uid != 0 || info.Mode()&os.ModeSticky == 0
}

func validateManagedRegularFile(path string, maxBytes int64, private bool) (os.FileInfo, error) {
	if err := validateManagedAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := validateManagedFileInfo(info, private); err != nil {
		return nil, err
	}
	if maxBytes > 0 && (info.Size() < 0 || info.Size() > maxBytes) {
		return nil, fmt.Errorf("managed file is too large")
	}
	return info, nil
}

func validateManagedFileInfo(info os.FileInfo, private bool) error {
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed path must be a regular file")
	}
	if err := validateManagedOwner(info); err != nil {
		return err
	}
	if info.Mode().Perm()&022 != 0 || private && info.Mode().Perm()&077 != 0 {
		return fmt.Errorf("managed file permissions are too broad")
	}
	return nil
}

func validateManagedOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("managed path ownership is unavailable")
	}
	uid := uint32(os.Geteuid())
	if stat.Uid != 0 && stat.Uid != uid {
		return fmt.Errorf("managed path has an untrusted owner")
	}
	return nil
}

func readManagedFile(path string, maxBytes int64, private bool) ([]byte, error) {
	file, err := openManagedFile(path, maxBytes, private)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	reader := io.Reader(file)
	if maxBytes > 0 {
		reader = io.LimitReader(file, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err == nil && maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("managed file is too large")
	}
	return data, err
}

func openManagedFile(path string, maxBytes int64, private bool) (*os.File, error) {
	if err := validateManagedAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := validateManagedFileInfo(info, private); err != nil {
		_ = file.Close()
		return nil, err
	}
	if maxBytes > 0 && (info.Size() < 0 || info.Size() > maxBytes) {
		_ = file.Close()
		return nil, fmt.Errorf("managed file is too large")
	}
	return file, nil
}

func (l *operationLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}

func safeOperationError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 256 {
		message = message[:256]
	}
	return message
}

func parseMigrationOutput(value string) (int, error) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid migration state")
	}
	return index, nil
}
