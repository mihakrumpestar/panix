package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/stretchr/testify/require"
)

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
