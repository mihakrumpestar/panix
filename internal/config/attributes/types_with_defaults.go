package attributes

import (
	"os"
	"slices"
	"strconv"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/ssh"
)

// KexecImage

var (
	ErrDefaultImageUnsupportedArch = errors.New("architecture not supported by default kexec")
)

type KexecImage string

func (k KexecImage) Get() KexecImage {
	if k == "" {
		return "https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"
	}

	return k
}

func (k KexecImage) String() string {
	return string(k.Get())
}

func (k KexecImage) IfDefaultImageIsArchSupported(arch string) error {
	if string(k) == "" {
		defaultKexecImageSupportedPlatforms := []string{"x86_64", "aarch64"}

		if !slices.Contains(defaultKexecImageSupportedPlatforms, arch) {
			return errors.Wrapf(ErrDefaultImageUnsupportedArch, "%s (supported: %s)", strconv.Quote(arch), defaultKexecImageSupportedPlatforms)
		}
	}

	return nil
}

// KexecSSHPort

type KexecSSHPort uint16

func (p KexecSSHPort) Get() uint16 {
	if p == 0 {
		return ssh.SSHDefaultPort
	}

	return uint16(p)
}

func (p KexecSSHPort) String() string {
	return strconv.Itoa(int(p.Get()))
}

// SudoProgram

type SudoProgram string

func (s SudoProgram) Get() SudoProgram {
	if s == "" {
		return "sudo"
	}

	return s
}

func (s SudoProgram) String() string {
	return string(s.Get())
}

// FileMode

const (
	FileModeDefault FileMode = 0700
)

type FileMode os.FileMode

func (f FileMode) Get() FileMode {
	if f == 0 {
		return FileModeDefault
	}

	return f
}

func (f FileMode) String() string {
	return strconv.FormatUint(uint64(f.Get()), 8)
}


