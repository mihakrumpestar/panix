package nix

import (
	"maps"

	"sort"

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
	// nix profile add has no built-in default flags.
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
	ProfileAddDefaultFlags []string `yaml:"profile_add_default_flags" json:"profile_add_default_flags,omitempty" desc:"List of base flags for 'nix profile add' command (default: [])"`

	// Extra flags are appended after the default flags for each command.
	ExtraFlags        []string `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"List of extra flags applied to both 'nix build' and 'nix copy'"`
	BuildFlags        []string `yaml:"build_flags" json:"build_flags,omitempty" desc:"List of extra flags for 'nix build' command (e.g. [--max-jobs, 4])"`
	CopyFlags         []string `yaml:"copy_flags" json:"copy_flags,omitempty" desc:"List of extra flags for 'nix copy' command (e.g. [--compress])"`
	NixosInstallFlags []string `yaml:"nixos_install_flags" json:"nixos_install_flags,omitempty" desc:"List of extra flags for 'nixos-install' command (e.g. [--no-bootloader])"`

	// Env sets environment variables for all nix commands. Some nix settings
	// are only available via env vars (e.g. NIX_USER_CONF_FILES, NIX_CONF_DIR,
	// NIX_DAEMON_SOCKET_PATH, NIX_STORE_DIR). Command-specific env fields
	// (build_env, copy_env, nixos_install_env, profile_add_env) are merged
	// on top of these, with specific keys overriding global ones.
	// Inheritance: parent env entries are inherited; child keys override
	// parent keys with the same name.
	Env map[string]string `yaml:"env" json:"env,omitempty" desc:"Environment variables applied to all nix commands (e.g. NIX_USER_CONF_FILES, NIX_STORE_DIR). Command-specific env fields override these."`

	// Command-specific env vars are merged on top of the global env field.
	// Keys in the specific map override the same keys in the global env.
	// Inheritance is the same as global env: parent entries inherited, child
	// keys override parent keys.
	BuildEnv          map[string]string `yaml:"build_env" json:"build_env,omitempty" desc:"Environment variables for 'nix build' only (merged on top of env)"`
	CopyEnv           map[string]string `yaml:"copy_env" json:"copy_env,omitempty" desc:"Environment variables for 'nix copy' only (merged on top of env)"`
	NixosInstallEnv   map[string]string `yaml:"nixos_install_env" json:"nixos_install_env,omitempty" desc:"Environment variables for 'nixos-install' only (merged on top of env)"`
	ProfileAddEnv map[string]string `yaml:"profile_add_env" json:"profile_add_env,omitempty" desc:"Environment variables for 'nix profile add' only (merged on top of env)"`
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

// GetProfileAddDefaultFlags returns the configured nix profile add
// default flags. Returns nil if not set — nix profile add has no built-in
// default flags.
func (c *NixConfig) GetProfileAddDefaultFlags() []string {
	return c.ProfileAddDefaultFlags
}

// GetBuildEnv returns env vars for `nix build`: the global env merged with
// build-specific env (specific keys override global). Returns KEY=VALUE
// strings sorted by key, or nil if neither is set.
func (c *NixConfig) GetBuildEnv() []string {
	return mergeEnvToSlice(c.Env, c.BuildEnv)
}

// GetCopyEnv returns env vars for `nix copy`: the global env merged with
// copy-specific env (specific keys override global). Returns KEY=VALUE
// strings sorted by key, or nil if neither is set.
func (c *NixConfig) GetCopyEnv() []string {
	return mergeEnvToSlice(c.Env, c.CopyEnv)
}

// GetNixosInstallEnv returns env vars for `nixos-install`: the global env
// merged with nixos-install-specific env (specific keys override global).
// Returns KEY=VALUE strings sorted by key, or nil if neither is set.
func (c *NixConfig) GetNixosInstallEnv() []string {
	return mergeEnvToSlice(c.Env, c.NixosInstallEnv)
}

// GetProfileAddEnv returns env vars for `nix profile add`: the global
// env merged with profile-install-specific env (specific keys override
// global). Returns KEY=VALUE strings sorted by key, or nil if neither is set.
func (c *NixConfig) GetProfileAddEnv() []string {
	return mergeEnvToSlice(c.Env, c.ProfileAddEnv)
}

// mergeEnvToSlice merges a global env map with a specific env map (specific
// overrides global on key conflicts), then returns sorted "KEY=VALUE" strings.
// Returns nil if both maps are empty.
func mergeEnvToSlice(global, specific map[string]string) []string {
	if len(global) == 0 && len(specific) == 0 {
		return nil
	}

	merged := make(map[string]string, len(global)+len(specific))
	maps.Copy(merged, global)

	// Specific overrides global.
	maps.Copy(merged, specific)

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	result := make([]string, 0, len(merged))
	for _, k := range keys {
		result = append(result, k+"="+merged[k])
	}

	return result
}

// Init merges parent NixConfig into this one.
//
// "Extra" flag fields (ExtraFlags, BuildFlags, CopyFlags, NixosInstallFlags) use
// append semantics: parent values are appended to the child's, so flags accumulate
// down the hierarchy.
//
// "Default" flag fields (ExperimentalFeatures, BuildDefaultFlags, CopyDefaultFlags,
// NixosInstallDefaultFlags, ProfileAddDefaultFlags) use override semantics: if
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
// Env maps use merge semantics: parent entries are always inherited, child
// keys override parent keys with the same name.
func (c *NixConfig) mergeParent(parent *NixConfig) {
	// Save child's default-flag slices and env maps before merge so we can
	// restore them. mergo.WithAppendSlice would incorrectly append parent
	// defaults into child slices, and would pollute non-nil child maps with
	// parent entries.
	savedFlags := newNixDefaultFlagsSnapshot(c)
	savedEnv := newNixEnvSnapshot(c)

	err := mergo.Merge(c, parent, mergo.WithAppendSlice)
	if err != nil {
		// mergo.Merge only returns errors for unsupported types, which won't
		// happen here. Wrap for safety.
		panic(errors.Wrap(err, "failed to inherit nix config"))
	}

	savedFlags.restore(c)
	savedEnv.restore(c)

	// Now manually merge parent env entries into child. Parent entries are
	// inherited; child keys override parent keys with the same name.
	mergeEnvMaps(c, parent)
}

// nixDefaultFlagsSnapshot captures the "default" flag slices of a NixConfig
// before a mergo merge, so they can be restored afterward (override semantics).
type nixDefaultFlagsSnapshot struct {
	experimentalFeatures       []string
	buildDefaultFlags          []string
	copyDefaultFlags           []string
	nixosInstallDefaultFlags   []string
	profileAddDefaultFlags []string
}

func newNixDefaultFlagsSnapshot(cfg *NixConfig) nixDefaultFlagsSnapshot {
	return nixDefaultFlagsSnapshot{
		experimentalFeatures:       cfg.ExperimentalFeatures,
		buildDefaultFlags:          cfg.BuildDefaultFlags,
		copyDefaultFlags:           cfg.CopyDefaultFlags,
		nixosInstallDefaultFlags:   cfg.NixosInstallDefaultFlags,
		profileAddDefaultFlags: cfg.ProfileAddDefaultFlags,
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

	if s.profileAddDefaultFlags != nil {
		cfg.ProfileAddDefaultFlags = s.profileAddDefaultFlags
	}
}

// nixEnvSnapshot captures all env maps of a NixConfig before a mergo merge,
// so they can be restored afterward. Unlike default-flag slices, env maps
// use merge semantics (parent entries always inherited), so the snapshot
// restores the child's original map reference — mergo would have polluted
// it with parent entries.
type nixEnvSnapshot struct {
	env               map[string]string
	buildEnv          map[string]string
	copyEnv           map[string]string
	nixosInstallEnv   map[string]string
	profileAddEnv map[string]string
}

func newNixEnvSnapshot(cfg *NixConfig) nixEnvSnapshot {
	return nixEnvSnapshot{
		env:               cfg.Env,
		buildEnv:          cfg.BuildEnv,
		copyEnv:           cfg.CopyEnv,
		nixosInstallEnv:   cfg.NixosInstallEnv,
		profileAddEnv: cfg.ProfileAddEnv,
	}
}

// restore writes back all saved env maps, preserving the child's original
// entries before mergo polluted them with parent data.
func (s nixEnvSnapshot) restore(cfg *NixConfig) {
	cfg.Env = s.env
	cfg.BuildEnv = s.buildEnv
	cfg.CopyEnv = s.copyEnv
	cfg.NixosInstallEnv = s.nixosInstallEnv
	cfg.ProfileAddEnv = s.profileAddEnv
}

// mergeEnvMaps inherits all env maps (global + command-specific) from parent
// into child. For each map, parent entries are inherited and child keys
// override parent keys with the same name. A nil child map inherits all
// parent entries; a non-nil child map (even if empty) inherits parent entries
// for keys it doesn't define — there are no built-in env defaults to "clear".
func mergeEnvMaps(child, parent *NixConfig) {
	mergeEnvMap(&child.Env, parent.Env)
	mergeEnvMap(&child.BuildEnv, parent.BuildEnv)
	mergeEnvMap(&child.CopyEnv, parent.CopyEnv)
	mergeEnvMap(&child.NixosInstallEnv, parent.NixosInstallEnv)
	mergeEnvMap(&child.ProfileAddEnv, parent.ProfileAddEnv)
}

// mergeEnvMap inherits entries from parent into child. If both define the
// same key, the child's value wins.
func mergeEnvMap(child *map[string]string, parent map[string]string) {
	if len(parent) == 0 {
		return
	}

	if *child == nil {
		*child = make(map[string]string, len(parent))
	}

	for k, v := range parent {
		_, exists := (*child)[k]
		if !exists {
			(*child)[k] = v
		}
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
