package nix

import (
	"maps"
	"sort"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/config/overrides"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/pkg/errors"
)

//nolint:lll
type NixConfig struct {
	BuildMode BuildMode `yaml:"build_mode" json:"build_mode" desc:"Build mode: local or remote. In local mode, build runs on the local machine and the closure is copied to targets. In remote mode, build runs on the first target machine via --store ssh-ng://<target>. For single-machine installables, transfer is skipped (closure is already on target). For multi-machine installables, the closure is copied from the first machine to the rest." default:"local" validate:"omitempty,oneof=local remote"`

	// Default flags replace the built-in base flags for each command when set.
	// The lists carrying a value panix computes at run time (copy, nixos-install,
	// profile add, both nix-env lists) may reference it as $PANIX_* (see
	// runtimevars); the other lists are passed through verbatim.
	ExperimentalFeatures              []string `yaml:"experimental_features" json:"experimental_features,omitempty" desc:"List of flags to enable nix experimental features, set to [] to pass none" default:"[--extra-experimental-features, nix-command flakes]"`
	BuildDefaultFlags                 []string `yaml:"build_default_flags" json:"build_default_flags,omitempty" desc:"List of base flags for 'nix build', set to [] to pass none" default:"[--no-link, --print-out-paths, --keep-going]"`
	CopyDefaultFlags                  []string `yaml:"copy_default_flags" json:"copy_default_flags,omitempty" desc:"Flags for 'nix copy', set to [] to pass none; $PANIX_STORE_URL is the destination store" default:"[--to, $PANIX_STORE_URL, --no-check-sigs]"`
	NixosInstallDefaultFlags          []string `yaml:"nixos_install_default_flags" json:"nixos_install_default_flags,omitempty" desc:"Flags for 'nixos-install', set to [] to pass none; $PANIX_CLOSURE is the built system and /mnt the target root" default:"[--no-root-passwd, --no-channel-copy, --system, $PANIX_CLOSURE, --root, /mnt]"`
	ProfileAddDefaultFlags            []string `yaml:"profile_add_default_flags" json:"profile_add_default_flags,omitempty" desc:"Flags and operands for 'nix profile add', $PANIX_CLOSURE is the package closure" default:"[$PANIX_CLOSURE]"`
	NixEnvListGenerationsDefaultFlags []string `yaml:"nix_env_list_generations_default_flags" json:"nix_env_list_generations_default_flags,omitempty" desc:"Flags and operands for listing generations, $PANIX_PROFILE_PATH is the profile path" default:"[--profile, $PANIX_PROFILE_PATH, --list-generations]"`
	NixEnvSetDefaultFlags             []string `yaml:"nix_env_set_default_flags" json:"nix_env_set_default_flags,omitempty" desc:"Flags and operands for setting the profile, $PANIX_PROFILE_PATH is the profile path and $PANIX_CLOSURE the closure" default:"[--profile, $PANIX_PROFILE_PATH, --set, $PANIX_CLOSURE]"`

	// Command prefixes replace the executable and nix subcommand a step runs, so
	// it can be delegated (e.g. 'nom build'); panix appends its flags after them.
	// Leave empty for the built-in prefix.
	BuildCommand         []string `yaml:"build_command" json:"build_command,omitempty" desc:"Command prefix for builds, e.g. [nom, build] to monitor build output" validate:"nonempty,dive,required" default:"[nix, build]"`
	CopyCommand          []string `yaml:"copy_command" json:"copy_command,omitempty" desc:"Command prefix for closure copies, e.g. [nom, copy] to monitor transfers" validate:"nonempty,dive,required" default:"[nix, copy]"`
	EvalCommand          []string `yaml:"eval_command" json:"eval_command,omitempty" desc:"Command prefix for flake validation evaluation" validate:"nonempty,dive,required" default:"[nix, eval]"`
	FlakeMetadataCommand []string `yaml:"flake_metadata_command" json:"flake_metadata_command,omitempty" desc:"Command prefix for flake URL validation" validate:"nonempty,dive,required" default:"[nix, flake, metadata]"`
	NixosInstallCommand  []string `yaml:"nixos_install_command" json:"nixos_install_command,omitempty" desc:"Command prefix for the NixOS install" validate:"nonempty,dive,required" default:"[nixos-install]"`
	NixEnvCommand        []string `yaml:"nix_env_command" json:"nix_env_command,omitempty" desc:"Command prefix for profile reads and generation switches" validate:"nonempty,dive,required" default:"[nix-env]"`
	ProfileAddCommand    []string `yaml:"profile_add_command" json:"profile_add_command,omitempty" desc:"Command prefix for package profile installs, leave unset for [nix, profile, add], or [nix, profile, install] on Lix" validate:"nonempty,dive,required"`

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
	BuildEnv        map[string]string `yaml:"build_env" json:"build_env,omitempty" desc:"Environment variables for 'nix build' only (merged on top of env)"`
	CopyEnv         map[string]string `yaml:"copy_env" json:"copy_env,omitempty" desc:"Environment variables for 'nix copy' only (merged on top of env)"`
	NixosInstallEnv map[string]string `yaml:"nixos_install_env" json:"nixos_install_env,omitempty" desc:"Environment variables for 'nixos-install' only (merged on top of env)"`
	ProfileAddEnv   map[string]string `yaml:"profile_add_env" json:"profile_add_env,omitempty" desc:"Environment variables for 'nix profile add' only (merged on top of env)"`
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

// Init merges the parent config into this one, then materializes the built-in
// defaults, so every command and flag list is usable as-is. Extra flags
// accumulate down the hierarchy; default flags and command prefixes override,
// where a nil child inherits the parent and an explicit value wins.
func (c *NixConfig) Init(parent *NixConfig) error {
	if parent != nil {
		c.mergeParent(parent)
	}

	if c.BuildMode == "" {
		c.BuildMode = BuildMode(structdefault.String[NixConfig]("BuildMode"))
	}

	structdefault.Apply(c)

	return nil
}

// mergeParent merges the parent into this config, then restores the fields whose
// mergo semantics differ: overridden slices keep the child value, and env maps
// merge instead of being replaced.
func (c *NixConfig) mergeParent(parent *NixConfig) {
	// mergo.WithAppendSlice would append parent values into child slices and
	// pollute non-nil child maps, so capture both first and restore them after.
	overridden := overrides.Capture(c.overrideFields()...)
	savedEnv := newNixEnvSnapshot(c)

	err := mergo.Merge(c, parent, mergo.WithAppendSlice)
	if err != nil {
		// mergo only errors on unsupported types, which cannot occur here.
		panic(errors.Wrap(err, "failed to inherit nix config"))
	}

	overridden.Restore()
	overrides.InheritEmpty(c.overrideFields(), parent.overrideFields())
	savedEnv.restore(c)

	// Parent env entries are inherited; child keys win on conflict.
	mergeEnvMaps(c, parent)
}

// overrideFields lists the NixConfig slices whose child value must win over the
// parent's during merge, rather than accumulate.
func (c *NixConfig) overrideFields() []*[]string {
	return []*[]string{
		&c.ExperimentalFeatures,
		&c.BuildDefaultFlags,
		&c.CopyDefaultFlags,
		&c.NixosInstallDefaultFlags,
		&c.ProfileAddDefaultFlags,
		&c.NixEnvListGenerationsDefaultFlags,
		&c.NixEnvSetDefaultFlags,
		&c.BuildCommand,
		&c.CopyCommand,
		&c.EvalCommand,
		&c.FlakeMetadataCommand,
		&c.NixosInstallCommand,
		&c.NixEnvCommand,
		&c.ProfileAddCommand,
	}
}

// nixEnvSnapshot captures all env maps of a NixConfig before a mergo merge,
// so they can be restored afterward. Unlike default-flag slices, env maps
// use merge semantics (parent entries always inherited), so the snapshot
// restores the child's original map reference; mergo would have polluted
// it with parent entries.
type nixEnvSnapshot struct {
	env             map[string]string
	buildEnv        map[string]string
	copyEnv         map[string]string
	nixosInstallEnv map[string]string
	profileAddEnv   map[string]string
}

func newNixEnvSnapshot(cfg *NixConfig) nixEnvSnapshot {
	return nixEnvSnapshot{
		env:             cfg.Env,
		buildEnv:        cfg.BuildEnv,
		copyEnv:         cfg.CopyEnv,
		nixosInstallEnv: cfg.NixosInstallEnv,
		profileAddEnv:   cfg.ProfileAddEnv,
	}
}

// restore writes back the saved env maps, undoing mergo's pollution.
func (s nixEnvSnapshot) restore(cfg *NixConfig) {
	cfg.Env = s.env
	cfg.BuildEnv = s.buildEnv
	cfg.CopyEnv = s.copyEnv
	cfg.NixosInstallEnv = s.nixosInstallEnv
	cfg.ProfileAddEnv = s.profileAddEnv
}

// mergeEnvMaps inherits every env map (global and command-specific) from parent
// into child, keeping the child's value where both define a key. There are no
// built-in env defaults, so an empty child map still inherits all parent
// entries.
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
