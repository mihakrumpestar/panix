package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/pkg/errors"
)

const filePrefix = "panix-snapshot-"

func fileName(s *config.Config) string {
	return fmt.Sprintf("%s%d-%d-%s.json", filePrefix, s.StartTime, s.SnapshotTime, s.SnapshotReason)
}

func Write(dir string, s *config.Config) error {
	err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot directory")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal snapshot")
	}

	path := filepath.Join(dir, fileName(s))
	err = os.WriteFile(path, data, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write snapshot file")
	}

	return nil
}

func Read(path string) (*config.Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot file")
	}

	var s config.Config
	err = json.Unmarshal(data, &s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal snapshot")
	}

	return &s, nil
}

func ReadDir(dir string) ([]*config.Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot directory")
	}

	var snapshots []*config.Config

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

	return snapshots, nil
}
