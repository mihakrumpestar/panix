package ssh

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// KnownHostsFile

type KnownHostsFile string //nolint:recvcheck

func (k *KnownHostsFile) Create() error {
	// User provided file does not need creation
	if k != nil && *k != "" {
		return nil
	}

	// Create tmp one
	tmpFile, err := os.CreateTemp("", "panix-knownhosts-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp known_hosts file")
	}

	err = tmpFile.Close()
	if err != nil {
		_ = os.Remove(tmpFile.Name())

		return errors.Wrap(err, "failed to close temp known_hosts file upon creation")
	}

	*k = KnownHostsFile(tmpFile.Name())

	return nil
}

func (k KnownHostsFile) IsAuto() bool {
	return k != "" && strings.HasPrefix(filepath.Dir(string(k)), os.TempDir())
}

func (k KnownHostsFile) RemoveIfAuto() {
	if k.IsAuto() {
		_ = os.Remove(string(k))
	}
}
