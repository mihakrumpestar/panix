package attributes

import (
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/ssh"
)

// KexecImage

var (
	ErrDefaultImageUnsupportedArch = errors.New("architecture not supported by default kexec")
)

type KexecImage string

// DefaultKexecImage is the built-in kexec tarball URL. The same literal is
// duplicated in the KexecConfig.Image struct tag (struct tags cannot reference
// constants); keep both in sync. $PANIX_ARCH is expanded at runtime.
const DefaultKexecImage = "https://github.com/nix-community/nixos-images/releases/latest/download/" +
	"nixos-kexec-installer-noninteractive-$PANIX_ARCH-linux.tar.gz"

func (k KexecImage) Get() KexecImage {
	if k == "" {
		return DefaultKexecImage
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

// Rollback

// Rollback is the Activation Guard tier (docs/design/activation-guard.md
// section 2). The zero value ("") means inherit: attribute merging copies a
// parent's non-empty tier into unset children (mergo non-pointer constraint),
// so an explicit "off" at a child level overrides a parent's "magic" while
// absence inherits. An effective off is the direct, unguarded activation path.
type Rollback string

const (
	RollbackOff   Rollback = "off"   // direct activation, no guard
	RollbackAuto  Rollback = "auto"  // guarded, self-committing
	RollbackMagic Rollback = "magic" // guarded plus confirmation window
)

// Get resolves the effective tier; "" (nothing was set anywhere up the chain)
// means off.
func (r Rollback) Get() Rollback {
	if r == "" {
		return RollbackOff
	}

	return r
}

func (r Rollback) String() string {
	return string(r.Get())
}

// IsGuarded reports whether the tier runs under the activation guard: auto
// commits on success and reverts on failure or timeout; magic additionally
// waits for panix's confirmation. off (and unset) is the direct path.
func (r Rollback) IsGuarded() bool {
	return r == RollbackAuto || r == RollbackMagic
}

// Activation Guard timeouts

// Defaults for the guarded-deploy timeouts. The same literals are duplicated
// in the Attributes struct tags (struct tags cannot reference constants);
// keep both in sync.
const (
	DefaultActivationTimeout      = 15 * time.Minute
	DefaultRollbackConfirmTimeout = 60 * time.Second
)

// GetActivationTimeout resolves the bound for the activation phase on guarded
// deploys; zero (unset) falls back to the default.
func (a *Attributes) GetActivationTimeout() time.Duration {
	if a.ActivationTimeout == 0 {
		return DefaultActivationTimeout
	}

	return a.ActivationTimeout
}

// GetRollbackConfirmTimeout resolves the magic-tier confirmation window; zero
// (unset) falls back to the default.
func (a *Attributes) GetRollbackConfirmTimeout() time.Duration {
	if a.RollbackConfirmTimeout == 0 {
		return DefaultRollbackConfirmTimeout
	}

	return a.RollbackConfirmTimeout
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
