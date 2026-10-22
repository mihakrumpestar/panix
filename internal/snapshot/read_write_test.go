package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSnapshotTestConfig() *config.Config {
	conf := &config.Config{}
	conf.Snapshot.StartTime = time.Unix(1700000000, 0).UTC()
	conf.Snapshot.SnapshotTime = time.Unix(1700000100, 0).UTC()
	conf.Snapshot.Reason = config.SnapshotReasonManual

	return conf
}

// TestWriteCreatesMissingDir pins the contract snapshot.dir validation relies
// on (issue #30): Write creates the directory when missing.
func TestWriteCreatesMissingDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "missing", "snapshot-dir")

	conf := &config.Config{}
	conf.Snapshot.StartTime = time.Unix(1700000000, 0).UTC()
	conf.Snapshot.SnapshotTime = time.Unix(1700000100, 0).UTC()
	conf.Snapshot.Reason = config.SnapshotReasonManual

	require.NoError(t, Write(dir, conf), "Write must create a missing snapshot directory")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "Write must produce exactly one snapshot file")
	require.True(t, strings.HasPrefix(entries[0].Name(), snapshotFilePrefix))
	require.True(t, strings.HasSuffix(entries[0].Name(), ".json"))
}

// TestWriteCreatesPrivateFile pins the 0600 mode so snapshot contents do not
// become group or world readable.
func TestWriteCreatesPrivateFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, Write(dir, newSnapshotTestConfig()))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	info, err := entries[0].Info()
	require.NoError(t, err)
	assert.Equal(t, filepermissions.PrivateFilePermissions, info.Mode().Perm())
}

// TestWriteRefusesSymlink pins O_NOFOLLOW: a symlink at the snapshot name must
// fail instead of truncating the file it points at.
func TestWriteRefusesSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep me"), 0o600))

	conf := newSnapshotTestConfig()
	suffix := "fixed-suffix"
	require.NoError(t, os.Symlink(victim, filepath.Join(dir, fileName(conf, suffix))))

	err := writeWithSuffix(dir, conf, suffix)
	require.Error(t, err)

	data, err := os.ReadFile(victim) //nolint:gosec // G304: test-owned t.TempDir path
	require.NoError(t, err)
	assert.Equal(t, []byte("keep me"), data, "the symlink target must not be clobbered")
}

// TestWriteRefusesPreExistingFile pins O_EXCL: a file at the snapshot name
// makes the write fail rather than overwrite it.
func TestWriteRefusesPreExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	conf := newSnapshotTestConfig()
	suffix := "fixed-suffix"
	path := filepath.Join(dir, fileName(conf, suffix))

	require.NoError(t, os.WriteFile(path, []byte("keep me"), 0o644)) //nolint:gosec // intentionally loose mode under test

	err := writeWithSuffix(dir, conf, suffix)
	require.Error(t, err)

	data, err := os.ReadFile(path) //nolint:gosec // G304: test-owned t.TempDir path
	require.NoError(t, err)
	assert.Equal(t, []byte("keep me"), data, "a pre-existing snapshot must not be overwritten")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "the refused write must not change the file")
}

// TestWriteSameSecondProducesDistinctNames pins the unique suffix: snapshots
// sharing a start time, snapshot time (same second) and reason must both land
// instead of colliding at O_EXCL.
func TestWriteSameSecondProducesDistinctNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	conf := newSnapshotTestConfig()

	require.NoError(t, Write(dir, conf))
	require.NoError(t, Write(dir, conf))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2, "both same-second snapshots must be written")

	names := []string{entries[0].Name(), entries[1].Name()}
	assert.NotEqual(t, names[0], names[1], "snapshot names must be unique")
}
