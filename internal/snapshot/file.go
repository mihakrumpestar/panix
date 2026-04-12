package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const filePrefix = "panix-snapshot-"

func fileName(s *Snapshot) string {
	return fmt.Sprintf("%s%d-%d-%s.json", filePrefix, s.AppStartTime, s.SnapshotTime, s.Reason)
}

func Write(dir string, s *Snapshot) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot directory")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal snapshot")
	}

	path := filepath.Join(dir, fileName(s))
	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		return errors.Wrap(err, "failed to write snapshot file")
	}

	return nil
}

func Read(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot file")
	}

	var s Snapshot
	err = json.Unmarshal(data, &s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal snapshot")
	}

	return &s, nil
}

func ReadDir(dir string) ([]*Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot directory")
	}

	var snapshots []*Snapshot

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), filePrefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		s, err := Read(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read snapshot %s", entry.Name())
		}

		snapshots = append(snapshots, s)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotTime < snapshots[j].SnapshotTime
	})

	return snapshots, nil
}
