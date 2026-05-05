package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/pkg/jsonx"
	"github.com/pkg/errors"
)

const snapshotFilePrefix = "panix-snapshot-"

func Read(path string) (*config.Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-provided by design
	if err != nil {
		return nil, errors.Wrap(err, "failed to read snapshot file")
	}

	var conf config.Config

	err = jsonx.Decode(data, &conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode snapshot")
	}

	conf.PostUnmarshalInit()

	return &conf, nil
}

func Write(dir string, conf *config.Config) error {
	err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot directory")
	}

	var data bytes.Buffer

	err = jsonx.Encode(conf, &data)
	if err != nil {
		return errors.Wrap(err, "failed to encode snapshot")
	}

	path := filepath.Join(dir, fileName(conf))

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
