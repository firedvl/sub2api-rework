//go:build unit

package updater

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, os.Chmod(policy.ComposeFile, 0620))

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

func TestManagedAncestorsAcceptSystemTempDirectory(t *testing.T) {
	require.NoError(t, validateManagedAncestors(filepath.Dir(t.TempDir())))
}

func TestManagedAncestorsRejectNonRootOwnedSymlink(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing symlink ownership requires root")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	alias := filepath.Join(root, "alias")
	require.NoError(t, os.Mkdir(target, 0700))
	require.NoError(t, os.Symlink(target, alias))
	require.NoError(t, os.Lchown(alias, 65534, 65534))

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
