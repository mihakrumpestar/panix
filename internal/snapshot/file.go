package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/pkg/errors"
)

const snapshotFilePrefix = "panix-snapshot-"

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

	s.PostUnmarshalInit()

	return &s, nil
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

// Helpers

func fileName(s *config.Config) string {
	return fmt.Sprintf("%s%d-%d-%s.json", snapshotFilePrefix, s.StartTime.Unix(), s.SnapshotTime.Unix(), s.SnapshotReason)
}
