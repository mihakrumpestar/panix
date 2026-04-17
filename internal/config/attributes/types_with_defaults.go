package attributes

import (
	"fmt"
	"os"
	"slices"
	"strconv"
)

// KexecImage

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

func (k KexecImage) IfDefaultIsArchSupported(arch string) error { // TODO: add this check to inspect after we know arch and bootstrap status
	if string(k) == "" {
		defaultKexecImageSupportedPlatforms := []string{"x86_64", "aarch64"}

		if !slices.Contains(defaultKexecImageSupportedPlatforms, arch) {
			return fmt.Errorf("architecture not supported by default kexec: %s (supported: %s)", strconv.Quote(arch), defaultKexecImageSupportedPlatforms)
		}
	}

	return nil
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
	return string(rune(f.Get()))
}

// ActivationMode

type ActivationModeD ActivationMode

type ActivationMode string

const (
	ActivationModeCheck       ActivationMode = "check"        // run pre-switch checks and exit
	ActivationModeSwitch      ActivationMode = "switch"       // make the configuration the boot default and activate now
	ActivationModeBoot        ActivationMode = "boot"         // make the configuration the boot default
	ActivationModeTest        ActivationMode = "test"         // activate the configuration, but don't make it the boot default
	ActivationModeDryActivate ActivationMode = "dry-activate" // show what would be done if this configuration were activated
)

func (am ActivationModeD) Get() ActivationMode {
	if am == "" {
		return ActivationModeSwitch
	}

	return ActivationMode(am)
}

func (am ActivationModeD) String() string {
	return string(am.Get())
}
