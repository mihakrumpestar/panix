package snapshot

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

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

	conf.PostUnmarshalSnapshotInit()

	return &conf, nil
}

func Write(dir string, conf *config.Config) error {
	return writeWithSuffix(dir, conf, rand.Text())
}

// writeWithSuffix is Write with an explicit unique suffix, so tests can target
// the exact path. Production names use a random suffix so two snapshots taken
// in the same second with the same start time and reason never collide at the
// O_EXCL create.
func writeWithSuffix(dir string, conf *config.Config, suffix string) error {
	err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot directory")
	}

	var data bytes.Buffer

	err = jsonx.Encode(conf, &data)
	if err != nil {
		return errors.Wrap(err, "failed to encode snapshot")
	}

	path := filepath.Join(dir, fileName(conf, suffix))

	// O_EXCL refuses any pre-existing file at the snapshot name, and O_NOFOLLOW
	// refuses a symlink instead of truncating its target; the 0600 mode keeps
	// snapshot contents private.
	file, err := os.OpenFile( //nolint:gosec // G304: path is the configured snapshot destination
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW,
		filepermissions.PrivateFilePermissions,
	)
	if err != nil {
		return errors.Wrap(err, "failed to write snapshot")
	}

	_, err = data.WriteTo(file)
	if err != nil {
		_ = file.Close()

		return errors.Wrap(err, "failed to write snapshot")
	}

	err = file.Close()
	if err != nil {
		return errors.Wrap(err, "failed to write snapshot")
	}

	return nil
}

// Helpers

func fileName(s *config.Config, suffix string) string {
	return fmt.Sprintf("%s%d-%d-%s-%s.json", snapshotFilePrefix, s.Snapshot.StartTime.Unix(), s.Snapshot.SnapshotTime.Unix(), s.Snapshot.Reason, suffix)
}
