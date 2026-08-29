//go:build unit

package updater

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

type managedTestFileInfo struct {
	mode os.FileMode
	uid  uint32
	gid  uint32
}

func (info managedTestFileInfo) Name() string       { return "managed" }
func (info managedTestFileInfo) Size() int64        { return 0 }
func (info managedTestFileInfo) Mode() os.FileMode  { return info.mode }
func (info managedTestFileInfo) ModTime() time.Time { return time.Time{} }
func (info managedTestFileInfo) IsDir() bool        { return info.mode.IsDir() }
func (info managedTestFileInfo) Sys() any {
	return &syscall.Stat_t{Uid: info.uid, Gid: info.gid}
}

func TestOperationLockRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	require.NoError(t, os.WriteFile(target, []byte("unchanged"), 0600))
	require.NoError(t, os.Symlink(target, filepath.Join(root, "operation.lock")))
	_, err := tryOperationLock(filepath.Join(root, "operation.lock"))
	require.Error(t, err)
	data, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	require.Equal(t, "unchanged", string(data))
}

func TestOperationLockRejectsConcurrentRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.lock")
	first, err := tryOperationLock(path)
	require.NoError(t, err)
	defer first.release()
	_, err = tryOperationLock(path)
	require.True(t, errors.Is(err, ErrOperationBusy))
}

func TestOperationLockRejectsBroadPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.lock")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	require.NoError(t, os.Chmod(path, 0640))

	_, err := tryOperationLock(path)

	require.ErrorContains(t, err, "permissions")
}

func TestManagedPathsRejectWritableDeploymentFile(t *testing.T) {
	policy := testPolicy(t.TempDir())
	writeUpdaterTestDeployment(t, policy)
	require.NoError(t, os.Chmod(policy.ComposeFiles[0], 0620))

	err := validateManagedPaths(policy)

	require.ErrorContains(t, err, "permissions")
}

func TestManagedPathsRejectUnsafeStateParent(t *testing.T) {
	policy := testPolicy(t.TempDir())
	writeUpdaterTestDeployment(t, policy)
	stateDirectory := filepath.Dir(policy.StatePath)
	require.NoError(t, os.MkdirAll(stateDirectory, 0700))
	require.NoError(t, os.Chmod(stateDirectory, 0770))

	err := validateManagedPaths(policy)

	require.ErrorContains(t, err, "permissions")
}

func TestManagedPathsRejectWritableAncestors(t *testing.T) {
	for _, test := range []struct {
		name string
		mode os.FileMode
	}{
		{"group writable non-sticky", 0770},
		{"world writable non-sticky", 0707},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			ancestor := filepath.Join(root, "unsafe")
			require.NoError(t, os.Mkdir(ancestor, 0700))
			require.NoError(t, os.Chmod(ancestor, test.mode))
			policy := testPolicy(root)
			policy.StatePath = filepath.Join(ancestor, "state", "state.json")
			writeUpdaterTestDeployment(t, policy)

			err := validateManagedPaths(policy)

			require.ErrorContains(t, err, "unsafe parent")
		})
	}
}

func TestManagedAncestorWriteRules(t *testing.T) {
	for _, test := range []struct {
		name     string
		info     managedTestFileInfo
		writable bool
	}{
		{
			"root non-root group 0775",
			managedTestFileInfo{mode: os.ModeDir | 0775, uid: 0, gid: 123},
			true,
		},
		{
			"root world writable non-sticky",
			managedTestFileInfo{mode: os.ModeDir | 0777, uid: 0, gid: 0},
			true,
		},
		{
			"non-root writable sticky",
			managedTestFileInfo{mode: os.ModeDir | os.ModeSticky | 0777, uid: 123, gid: 123},
			true,
		},
		{
			"root writable sticky",
			managedTestFileInfo{mode: os.ModeDir | os.ModeSticky | 0777, uid: 0, gid: 123},
			false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.writable, managedAncestorIsWritable(test.info))
		})
	}
	untrusted := managedTestFileInfo{mode: os.ModeDir | 0755, uid: uint32(os.Geteuid() + 1), gid: 123}
	require.ErrorContains(t, validateManagedOwner(untrusted), "untrusted owner")
}

func TestPrivateStateLayoutStartsBesideUnsafeVarLog(t *testing.T) {
	root := t.TempDir()
	varLog := filepath.Join(root, "var", "log")
	require.NoError(t, os.MkdirAll(varLog, 0700))
	require.NoError(t, os.Chmod(varLog, 0775))

	stateDirectory := filepath.Join(root, "var", "lib", "sub2api-rework-updater")
	policy := testPolicy(root)
	policy.StatePath = filepath.Join(stateDirectory, "state.json")
	policy.AuditPath = filepath.Join(stateDirectory, "audit.jsonl")
	policy.BackupDirectory = filepath.Join(stateDirectory, "backups")
	writeUpdaterTestDeployment(t, policy)

	_, err := NewService(policy, &fakeRunner{}, &fakeManifestFetcher{})
	require.NoError(t, err)
	require.FileExists(t, policy.StatePath)
	require.Equal(t, os.FileMode(0600), fileMode(t, policy.StatePath))
	require.Equal(t, os.FileMode(0700), fileMode(t, stateDirectory))

	unsafe := policy
	unsafe.AuditPath = filepath.Join(varLog, "sub2api-rework-updater", "audit.jsonl")
	_, err = NewService(unsafe, &fakeRunner{}, &fakeManifestFetcher{})
	require.ErrorContains(t, err, "unsafe parent")
	require.NoError(t, os.Chmod(varLog, 0755))
	_, err = NewService(unsafe, &fakeRunner{}, &fakeManifestFetcher{})
	require.NoError(t, err)
}

func TestAuditRotationPreservesPrivateFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	require.NoError(t, ensureManagedDirectory(directory, 0700, true))
	path := filepath.Join(directory, "audit.jsonl")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), maxAuditBytes), 0600))
	store := &stateStore{auditPath: path}

	err := store.audit(updatecontract.OperationSummary{OperationID: "upd-test", Result: "succeeded"})

	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), fileMode(t, path))
	require.Equal(t, os.FileMode(0600), fileMode(t, path+".1"))
	rotated, err := os.ReadFile(path + ".1")
	require.NoError(t, err)
	require.Len(t, rotated, maxAuditBytes)
	current, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(current), `"operation_id":"upd-test"`)
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func TestManagedAncestorsAcceptSystemTempDirectory(t *testing.T) {
	require.NoError(t, validateManagedAncestors(filepath.Dir(t.TempDir())))
}

func TestManagedAncestorsRejectNonRootOwnedSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	alias := filepath.Join(root, "alias")
	require.NoError(t, os.Mkdir(target, 0700))
	require.NoError(t, os.Symlink(target, alias))
	if os.Geteuid() == 0 {
		require.NoError(t, os.Lchown(alias, 65534, 65534))
	}

	err := validateManagedAncestors(alias)

	require.ErrorContains(t, err, "unsafe parent")
}

func TestStateStoreRejectsSymlink(t *testing.T) {
	policy := testPolicy(t.TempDir())
	writeUpdaterTestDeployment(t, policy)
	service, err := NewService(policy, &fakeRunner{}, &fakeManifestFetcher{})
	require.NoError(t, err)
	require.NoError(t, os.Remove(policy.StatePath))
	target := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"schema_version":1}`), 0600))
	require.NoError(t, os.Symlink(target, policy.StatePath))

	_, err = service.Status()

	require.Error(t, err)
}

func TestParseMigrationOutputRejectsUnexpectedCommandOutput(t *testing.T) {
	value, err := parseMigrationOutput("232\n")
	require.NoError(t, err)
	require.Equal(t, 232, value)
	_, err = parseMigrationOutput("password=secret")
	require.Error(t, err)
}
