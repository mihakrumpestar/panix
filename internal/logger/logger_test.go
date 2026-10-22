package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenLogFilePathCreatesPrivateFile pins the 0600 creation mode.
func TestOpenLogFilePathCreatesPrivateFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "panix.log")

	writer, gotPath, err := openLogFilePath(path)
	require.NoError(t, err)
	assert.Equal(t, path, gotPath)

	closer, ok := writer.(interface{ Close() error })
	require.True(t, ok)
	require.NoError(t, closer.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, filepermissions.PrivateFilePermissions, info.Mode().Perm())
}

// TestOpenLogFilePathRefusesSymlink pins O_NOFOLLOW: a symlink at the
// timestamped log path must fail instead of receiving the log stream.
func TestOpenLogFilePathRefusesSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep me"), 0o600))

	path := filepath.Join(dir, "panix.log.123")
	require.NoError(t, os.Symlink(victim, path))

	_, _, err := openLogFilePath(path)
	require.Error(t, err)

	data, err := os.ReadFile(victim) //nolint:gosec // G304: test-owned t.TempDir path
	require.NoError(t, err)
	assert.Equal(t, []byte("keep me"), data, "the symlink target must not be written")
}

// TestOpenLogFilePathTightensExistingFile pins that an existing broader-mode
// log is chmodded to 0600 after open.
func TestOpenLogFilePathTightensExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "panix.log.123")
	require.NoError(t, os.WriteFile(path, []byte("previous\n"), 0o644)) //nolint:gosec // intentionally loose mode under test

	writer, _, err := openLogFilePath(path)
	require.NoError(t, err)

	closer, ok := writer.(interface{ Close() error })
	require.True(t, ok)
	require.NoError(t, closer.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, filepermissions.PrivateFilePermissions, info.Mode().Perm())

	data, err := os.ReadFile(path) //nolint:gosec // G304: test-owned t.TempDir path
	require.NoError(t, err)
	assert.Equal(t, []byte("previous\n"), data, "opening for append must not truncate")
}
