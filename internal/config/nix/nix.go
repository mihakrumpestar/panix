package nix

import (
	"dario.cat/mergo"
	"github.com/pkg/errors"
)

// Default base flags for nix commands. These are used when the user does not
// override them via the corresponding NixConfig fields.
var (
	DefaultExperimentalFeatures = []string{"--extra-experimental-features", "nix-command flakes"}
	DefaultBuildFlags           = []string{"--no-link", "--no-update-lock-file", "--print-out-paths", "--keep-going"}
	DefaultCopyFlags            = []string{"--no-check-sigs"}
	DefaultNixosInstallFlags    = []string{"--no-root-passwd", "--no-channel-copy"}
	// nix profile install has no built-in default flags.
)

//nolint:lll
type NixConfig struct {
	BuildMode BuildMode `yaml:"build_mode" json:"build_mode" desc:"Build mode: local or remote. In local mode, build runs on the local machine and the closure is copied to targets. In remote mode, build runs on the first target machine via --store ssh-ng://<target>. For single-machine installables, transfer is skipped (closure is already on target). For multi-machine installables, the closure is copied from the first machine to the rest." default:"local" validate:"omitempty,oneof=local remote"`

	// DefaultFlags replace the built-in base flags for each command when set.
	// Leave empty to use the built-in defaults.
	ExperimentalFeatures       []string `yaml:"experimental_features" json:"experimental_features,omitempty" desc:"List of flags to enable nix experimental features (default: [--extra-experimental-features, nix-command flakes])"`
	BuildDefaultFlags          []string `yaml:"build_default_flags" json:"build_default_flags,omitempty" desc:"List of base flags for 'nix build' command (default: [--no-link, --no-update-lock-file, --print-out-paths, --keep-going])"`
	CopyDefaultFlags           []string `yaml:"copy_default_flags" json:"copy_default_flags,omitempty" desc:"List of base flags for 'nix copy' command (default: [--no-check-sigs])"`
	NixosInstallDefaultFlags   []string `yaml:"nixos_install_default_flags" json:"nixos_install_default_flags,omitempty" desc:"List of base flags for 'nixos-install' command (default: [--no-root-passwd, --no-channel-copy])"`
	ProfileInstallDefaultFlags []string `yaml:"profile_install_default_flags" json:"profile_install_default_flags,omitempty" desc:"List of base flags for 'nix profile install' command (default: [])"`

	// Extra flags are appended after the default flags for each command.
	ExtraFlags        []string `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"List of extra flags applied to both 'nix build' and 'nix copy'"`
	BuildFlags        []string `yaml:"build_flags" json:"build_flags,omitempty" desc:"List of extra flags for 'nix build' command (e.g. [--max-jobs, 4])"`
	CopyFlags         []string `yaml:"copy_flags" json:"copy_flags,omitempty" desc:"List of extra flags for 'nix copy' command (e.g. [--compress])"`
	NixosInstallFlags []string `yaml:"nixos_install_flags" json:"nixos_install_flags,omitempty" desc:"List of extra flags for 'nixos-install' command (e.g. [--no-bootloader])"`
}

// GetExperimentalFeatures returns the configured experimental features flags,
// or the default if not set (nil). An explicitly empty slice ([]) clears the defaults.
func (c *NixConfig) GetExperimentalFeatures() []string {
	if c.ExperimentalFeatures != nil {
		return c.ExperimentalFeatures
	}

	return DefaultExperimentalFeatures
}

// GetBuildDefaultFlags returns the configured build default flags,
// or the default if not set (nil). An explicitly empty slice ([]) clears the defaults.
func (c *NixConfig) GetBuildDefaultFlags() []string {
	if c.BuildDefaultFlags != nil {
		return c.BuildDefaultFlags
	}

	return DefaultBuildFlags
}

// GetCopyDefaultFlags returns the configured copy default flags,
// or the default if not set (nil). An explicitly empty slice ([]) clears the defaults.
func (c *NixConfig) GetCopyDefaultFlags() []string {
	if c.CopyDefaultFlags != nil {
		return c.CopyDefaultFlags
	}

	return DefaultCopyFlags
}

// GetNixosInstallDefaultFlags returns the configured nixos-install default flags,
// or the default if not set (nil). An explicitly empty slice ([]) clears the defaults.
func (c *NixConfig) GetNixosInstallDefaultFlags() []string {
	if c.NixosInstallDefaultFlags != nil {
		return c.NixosInstallDefaultFlags
	}

	return DefaultNixosInstallFlags
}

// GetProfileInstallDefaultFlags returns the configured nix profile install
// default flags. Returns nil if not set — nix profile install has no built-in
// default flags.
func (c *NixConfig) GetProfileInstallDefaultFlags() []string {
	return c.ProfileInstallDefaultFlags
}

// Init merges parent NixConfig into this one.
//
// "Extra" flag fields (ExtraFlags, BuildFlags, CopyFlags, NixosInstallFlags) use
// append semantics: parent values are appended to the child's, so flags accumulate
// down the hierarchy.
//
// "Default" flag fields (ExperimentalFeatures, BuildDefaultFlags, CopyDefaultFlags,
// NixosInstallDefaultFlags, ProfileInstallDefaultFlags) use override semantics: if
// the child has a value, it is kept as-is; if the child is nil, it inherits the
// parent's value. This prevents parent defaults from polluting a child's explicit
// override.
func (c *NixConfig) Init(parent *NixConfig) error {
	if parent != nil {
		c.mergeParent(parent)
	}

	if c.BuildMode == "" {
		c.BuildMode = BuildModeLocal
	}

	return nil
}

// mergeParent merges parent NixConfig into this one. "Extra" flag fields use
// append semantics (mergo.WithAppendSlice), while "default" flag fields use
// override semantics (child's non-nil value is preserved after merge).
func (c *NixConfig) mergeParent(parent *NixConfig) {
	// Save child's default-flag slices before merge so we can restore them.
	// mergo.WithAppendSlice would incorrectly append parent defaults into child.
	saved := newNixDefaultFlagsSnapshot(c)

	err := mergo.Merge(c, parent, mergo.WithAppendSlice)
	if err != nil {
		// mergo.Merge only returns errors for unsupported types, which won't
		// happen here. Wrap for safety.
		panic(errors.Wrap(err, "failed to inherit nix config"))
	}

	saved.restore(c)
}

// nixDefaultFlagsSnapshot captures the "default" flag slices of a NixConfig
// before a mergo merge, so they can be restored afterward (override semantics).
type nixDefaultFlagsSnapshot struct {
	experimentalFeatures       []string
	buildDefaultFlags          []string
	copyDefaultFlags           []string
	nixosInstallDefaultFlags   []string
	profileInstallDefaultFlags []string
}

func newNixDefaultFlagsSnapshot(cfg *NixConfig) nixDefaultFlagsSnapshot {
	return nixDefaultFlagsSnapshot{
		experimentalFeatures:       cfg.ExperimentalFeatures,
		buildDefaultFlags:          cfg.BuildDefaultFlags,
		copyDefaultFlags:           cfg.CopyDefaultFlags,
		nixosInstallDefaultFlags:   cfg.NixosInstallDefaultFlags,
		profileInstallDefaultFlags: cfg.ProfileInstallDefaultFlags,
	}
}

// restore writes back any non-nil saved slices, preserving the child's override.
func (s nixDefaultFlagsSnapshot) restore(cfg *NixConfig) {
	if s.experimentalFeatures != nil {
		cfg.ExperimentalFeatures = s.experimentalFeatures
	}

	if s.buildDefaultFlags != nil {
		cfg.BuildDefaultFlags = s.buildDefaultFlags
	}

	if s.copyDefaultFlags != nil {
		cfg.CopyDefaultFlags = s.copyDefaultFlags
	}

	if s.nixosInstallDefaultFlags != nil {
		cfg.NixosInstallDefaultFlags = s.nixosInstallDefaultFlags
	}

	if s.profileInstallDefaultFlags != nil {
		cfg.ProfileInstallDefaultFlags = s.profileInstallDefaultFlags
	}
}

// BuildMode determines how nix builds are executed.
type BuildMode string

const (
	// BuildModeLocal:
	// Build runs on the local machine. Closure is copied to the target via nix copy.
	BuildModeLocal BuildMode = "local"

	// BuildModeRemote:
	// Build runs on the first target machine via --store ssh-ng://<target>.
	// - Single-machine configuration: transfer is skipped (closure is already on target).
	// - Multi-machine configuration: closure is copied from the first machine to the rest
	//   via nix copy --from <first-machine> --to <other-machine>.
	BuildModeRemote BuildMode = "remote"
)

func (m BuildMode) String() string {
	return string(m)
}
