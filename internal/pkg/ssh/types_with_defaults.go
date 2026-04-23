package ssh

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// SSHPort

const (
	SSHPortDefault = 22
)

type SSHPort uint16 //nolint:recvcheck

func (s SSHPort) Get() uint16 {
	if s == 0 {
		return SSHPortDefault
	}

	return uint16(s)
}

func (s *SSHPort) Set(port uint16) {
	*s = SSHPort(port)
}

func (s SSHPort) String() string {
	return strconv.Itoa(int(s.Get()))
}

// SSHUsername

type SSHUsername string //nolint:recvcheck

func (s SSHUsername) Get() string {
	if s == "" {
		return "root"
	}

	return string(s)
}

func (s *SSHUsername) Set(username string) {
	*s = SSHUsername(username)
}

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
