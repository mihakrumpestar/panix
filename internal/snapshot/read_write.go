package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/pkg/jsonx"
	"github.com/pkg/errors"
)

const snapshotFilePrefix = "panix-snapshot-"

func Read(path string) (*config.Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot file")
	}

	var s config.Config

	err = jsonx.Decode(data, &s)
	if err != nil {
		return nil, err
	}

	s.PostUnmarshalInit()

	return &s, nil
}

func Write(dir string, s *config.Config) error {
	err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot directory")
	}

	var data bytes.Buffer

	err = jsonx.Encode(s, &data)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, fileName(s))
	err = os.WriteFile(path, data.Bytes(), filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write snapshot")
	}

	return nil
}

// Helpers

func fileName(s *config.Config) string {
	return fmt.Sprintf("%s%d-%d-%s.json", snapshotFilePrefix, s.Snapshot.StartTime.Unix(), s.Snapshot.SnapshotTime.Unix(), s.Snapshot.Reason)
}
