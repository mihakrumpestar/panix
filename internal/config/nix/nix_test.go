package nix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Built-in defaults ---
//
// Built-in defaults are declared in the struct tags and materialized by Init, so
// they are pinned literally here: a tag edit that changes a flag Panix depends on
// must fail a test.

func TestInit_MaterializesDefaults(t *testing.T) {
	t.Parallel()

	cfg := &NixConfig{}

	require.NoError(t, cfg.Init(nil))

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{"experimental features", cfg.ExperimentalFeatures, []string{"--extra-experimental-features", "nix-command flakes"}},
		{"build default flags", cfg.BuildDefaultFlags, []string{"--no-link", "--print-out-paths", "--keep-going"}},
		{"nixos-install default flags", cfg.NixosInstallDefaultFlags,
			[]string{"--no-root-passwd", "--no-channel-copy", "--system", "$PANIX_CLOSURE", "--root", "/mnt"}},
		{"profile add default flags", cfg.ProfileAddDefaultFlags, []string{"$PANIX_CLOSURE"}},
		{"copy default flags", cfg.CopyDefaultFlags, []string{"--to", "$PANIX_STORE_URL", "--no-check-sigs"}},
		{"nix-env set default flags", cfg.NixEnvSetDefaultFlags,
			[]string{"--profile", "$PANIX_PROFILE_PATH", "--set", "$PANIX_CLOSURE"}},
		{"nix-env list generations default flags", cfg.NixEnvListGenerationsDefaultFlags,
			[]string{"--profile", "$PANIX_PROFILE_PATH", "--list-generations"}},
		{"build command", cfg.BuildCommand, []string{"nix", "build"}},
		{"copy command", cfg.CopyCommand, []string{"nix", "copy"}},
		{"eval command", cfg.EvalCommand, []string{"nix", "eval"}},
		{"flake metadata command", cfg.FlakeMetadataCommand, []string{"nix", "flake", "metadata"}},
		{"nixos-install command", cfg.NixosInstallCommand, []string{"nixos-install"}},
		{"nix-env command", cfg.NixEnvCommand, []string{"nix-env"}},
		// profile add is flavor dependent, so it stays unset for the caller to
		// resolve: Lix never adopted the install -> add rename.
		{"profile add command", cfg.ProfileAddCommand, nil},
		{"build mode", []string{cfg.BuildMode.String()}, []string{"local"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.got)
		})
	}
}

// An explicit empty flag list means "pass nothing", so it must not be replaced
// by the built-in flags.
func TestInit_ExplicitEmptyFlagListClearsDefault(t *testing.T) {
	t.Parallel()

	cfg := &NixConfig{BuildDefaultFlags: []string{}, CopyDefaultFlags: []string{}, ExperimentalFeatures: []string{}}

	require.NoError(t, cfg.Init(nil))

	assert.NotNil(t, cfg.BuildDefaultFlags)
	assert.Empty(t, cfg.BuildDefaultFlags)
	assert.NotNil(t, cfg.CopyDefaultFlags)
	assert.Empty(t, cfg.CopyDefaultFlags)
	assert.NotNil(t, cfg.ExperimentalFeatures)
	assert.Empty(t, cfg.ExperimentalFeatures)
}

// An explicit [] is taken as written, so an empty command prefix is left empty
// for validation to reject instead of silently reverting to the built-in command.
func TestInit_EmptyCommandPrefixStaysEmpty(t *testing.T) {
	t.Parallel()

	cfg := &NixConfig{BuildCommand: []string{}, CopyCommand: []string{}}

	require.NoError(t, cfg.Init(nil))

	assert.Empty(t, cfg.BuildCommand)
	assert.Empty(t, cfg.CopyCommand)
}

func TestInit_KeepsConfiguredValues(t *testing.T) {
	t.Parallel()

	cfg := &NixConfig{
		BuildCommand:         []string{"nom", "build"},
		BuildDefaultFlags:    []string{"--print-out-paths"},
		CopyCommand:          []string{"nom", "copy"},
		ExperimentalFeatures: []string{"--extra-experimental-features", "ca-derivations"},
	}

	require.NoError(t, cfg.Init(nil))

	assert.Equal(t, []string{"nom", "build"}, cfg.BuildCommand)
	assert.Equal(t, []string{"--print-out-paths"}, cfg.BuildDefaultFlags)
	assert.Equal(t, []string{"nom", "copy"}, cfg.CopyCommand)
	assert.Equal(t, []string{"--extra-experimental-features", "ca-derivations"}, cfg.ExperimentalFeatures)
}

// --- Init override semantics ---
//
// Flag-list and command-prefix fields use override semantics: a child value is
// kept, and a nil child inherits the parent's value. "Extra" flag fields use
// append semantics: parent values are appended to the child's.
//
// The table holds an accessor pair rather than a prepared struct, so both tests
// build their own configs and stay independent.

type overrideField struct {
	name   string
	set    func(*NixConfig, []string)
	get    func(*NixConfig) []string
	parent []string
	child  []string
}

var overrideFields = []overrideField{
	{
		name:   "BuildDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.BuildDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.BuildDefaultFlags },
		parent: []string{"--parent-flag"}, child: []string{"--child-flag"},
	},
	{
		name:   "ExperimentalFeatures",
		set:    func(c *NixConfig, v []string) { c.ExperimentalFeatures = v },
		get:    func(c *NixConfig) []string { return c.ExperimentalFeatures },
		parent: []string{"--parent-exp"}, child: []string{"--child-exp"},
	},
	{
		name:   "CopyDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.CopyDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.CopyDefaultFlags },
		parent: []string{"--parent-copy"}, child: []string{"--child-copy"},
	},
	{
		name:   "NixosInstallDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.NixosInstallDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.NixosInstallDefaultFlags },
		parent: []string{"--parent-nixos"}, child: []string{"--child-nixos"},
	},
	{
		name:   "ProfileAddDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.ProfileAddDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.ProfileAddDefaultFlags },
		parent: []string{"--parent-profile"}, child: []string{"--child-profile"},
	},
	{
		name:   "NixEnvListGenerationsDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.NixEnvListGenerationsDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.NixEnvListGenerationsDefaultFlags },
		parent: []string{"--parent-list"}, child: []string{"--child-list"},
	},
	{
		name:   "NixEnvSetDefaultFlags",
		set:    func(c *NixConfig, v []string) { c.NixEnvSetDefaultFlags = v },
		get:    func(c *NixConfig) []string { return c.NixEnvSetDefaultFlags },
		parent: []string{"--parent-set"}, child: []string{"--child-set"},
	},
	{
		name:   "EvalCommand",
		set:    func(c *NixConfig, v []string) { c.EvalCommand = v },
		get:    func(c *NixConfig) []string { return c.EvalCommand },
		parent: []string{"nix", "eval"}, child: []string{"waybar-eval", "eval"},
	},
	{
		name:   "FlakeMetadataCommand",
		set:    func(c *NixConfig, v []string) { c.FlakeMetadataCommand = v },
		get:    func(c *NixConfig) []string { return c.FlakeMetadataCommand },
		parent: []string{"nix", "flake", "metadata"}, child: []string{"waybar-flake", "metadata"},
	},
	{
		name:   "NixosInstallCommand",
		set:    func(c *NixConfig, v []string) { c.NixosInstallCommand = v },
		get:    func(c *NixConfig) []string { return c.NixosInstallCommand },
		parent: []string{"nixos-install"}, child: []string{"/run/wrappers/bin/nixos-install"},
	},
	{
		name:   "NixEnvCommand",
		set:    func(c *NixConfig, v []string) { c.NixEnvCommand = v },
		get:    func(c *NixConfig) []string { return c.NixEnvCommand },
		parent: []string{"nix-env"}, child: []string{"/run/wrappers/bin/nix-env"},
	},
	{
		name:   "BuildCommand",
		set:    func(c *NixConfig, v []string) { c.BuildCommand = v },
		get:    func(c *NixConfig) []string { return c.BuildCommand },
		parent: []string{"nom", "build"}, child: []string{"waybar-nom", "build"},
	},
	{
		name:   "CopyCommand",
		set:    func(c *NixConfig, v []string) { c.CopyCommand = v },
		get:    func(c *NixConfig) []string { return c.CopyCommand },
		parent: []string{"nom", "copy"}, child: []string{"waybar-nom", "copy"},
	},
	{
		name:   "ProfileAddCommand",
		set:    func(c *NixConfig, v []string) { c.ProfileAddCommand = v },
		get:    func(c *NixConfig) []string { return c.ProfileAddCommand },
		parent: []string{"nix", "profile", "install"}, child: []string{"nix", "profile", "add"},
	},
}

// A configured child value survives the merge; the parent value must not be
// appended to it.
func TestInit_OverrideFields_ChildOverridesParent(t *testing.T) {
	t.Parallel()

	for _, tt := range overrideFields {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := &NixConfig{}
			tt.set(parent, tt.parent)

			child := &NixConfig{}
			tt.set(child, tt.child)

			require.NoError(t, child.Init(parent))

			assert.Equal(t, tt.child, tt.get(child), "child must override the parent, not append")
		})
	}
}

// An unset child inherits the parent value.
func TestInit_OverrideFields_NilChildInheritsParent(t *testing.T) {
	t.Parallel()

	for _, tt := range overrideFields {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := &NixConfig{}
			tt.set(parent, tt.parent)

			child := &NixConfig{}

			require.NoError(t, child.Init(parent))

			assert.Equal(t, tt.parent, tt.get(child), "an unset child must inherit the parent value")
		})
	}
}

// An explicitly empty parent value is a real value: a child that is unset must
// inherit "pass nothing" instead of falling back to the built-in flags.
func TestInit_EmptyParentValueInherited(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{CopyDefaultFlags: []string{}}
	child := &NixConfig{}

	require.NoError(t, child.Init(parent))

	assert.NotNil(t, child.CopyDefaultFlags, "an empty parent list must reach the child, not the built-in default")
	assert.Empty(t, child.CopyDefaultFlags)
}

func TestInit_ExtraFlags_AppendSemantics(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{ExtraFlags: []string{"--parent-extra"}}
	child := &NixConfig{ExtraFlags: []string{"--child-extra"}}

	require.NoError(t, child.Init(parent))

	assert.Equal(t, []string{"--child-extra", "--parent-extra"}, child.ExtraFlags,
		"ExtraFlags should append parent values to child (append semantics)")
}

// --- Command-specific env getters (merge global + specific) ---

func Test_CommandEnvGetters_GlobalAndSpecificMerged(t *testing.T) {
	t.Parallel()

	// Global env is inherited by all command-specific getters; specific keys
	// override global keys with the same name.
	nixC := &NixConfig{
		Env: map[string]string{
			"NIX_SSL_CERT_FILE": "/global",
			"NIX_BUILD_CORES":   "4",
		},
		BuildEnv: map[string]string{
			"NIX_BUILD_CORES": "8", // overrides global
		},
		CopyEnv: map[string]string{
			"NIX_CONFIG": "max-jobs = 2", // copy-only
		},
		NixosInstallEnv: map[string]string{"NIX_CONFIG": "max-jobs = 1"},
		ProfileAddEnv:   map[string]string{},
	}

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "GetBuildEnv: global merged, specific overrides",
			got:  nixC.GetBuildEnv(),
			want: []string{"NIX_BUILD_CORES=8", "NIX_SSL_CERT_FILE=/global"},
		},
		{
			name: "GetCopyEnv: global + copy-only",
			got:  nixC.GetCopyEnv(),
			want: []string{"NIX_BUILD_CORES=4", "NIX_CONFIG=max-jobs = 2", "NIX_SSL_CERT_FILE=/global"},
		},
		{
			name: "GetNixosInstallEnv: global + nixos-install-only",
			got:  nixC.GetNixosInstallEnv(),
			want: []string{"NIX_BUILD_CORES=4", "NIX_CONFIG=max-jobs = 1", "NIX_SSL_CERT_FILE=/global"},
		},
		{
			name: "GetProfileAddEnv: global only (specific is empty map)",
			got:  nixC.GetProfileAddEnv(),
			want: []string{"NIX_BUILD_CORES=4", "NIX_SSL_CERT_FILE=/global"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func Test_CommandEnvGetters_BothNilReturnsNil(t *testing.T) {
	t.Parallel()

	nixC := &NixConfig{}

	assertion := assert.New(t)
	assertion.Nil(nixC.GetBuildEnv())
	assertion.Nil(nixC.GetCopyEnv())
	assertion.Nil(nixC.GetNixosInstallEnv())
	assertion.Nil(nixC.GetProfileAddEnv())
}

func Test_CommandEnvGetters_GlobalOnly(t *testing.T) {
	t.Parallel()

	nixC := &NixConfig{Env: map[string]string{"NIX_SSL_CERT_FILE": "/certs"}}

	want := []string{"NIX_SSL_CERT_FILE=/certs"}

	assertion := assert.New(t)
	assertion.Equal(want, nixC.GetBuildEnv())
	assertion.Equal(want, nixC.GetCopyEnv())
	assertion.Equal(want, nixC.GetNixosInstallEnv())
	assertion.Equal(want, nixC.GetProfileAddEnv())
}

func Test_CommandEnvGetters_SpecificOnly(t *testing.T) {
	t.Parallel()

	c := &NixConfig{BuildEnv: map[string]string{"NIX_BUILD_CORES": "8"}}

	assertion := assert.New(t)
	assertion.Equal([]string{"NIX_BUILD_CORES=8"}, c.GetBuildEnv())
	assertion.Nil(c.GetCopyEnv(), "copy env should be nil when neither global nor copy-specific is set")
}

// --- Env inheritance ---

func Test_NixConfig_Init_Env_ChildOverridesParent(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{Env: map[string]string{"NIX_BUILD_CORES": "4", "NIX_SSL_CERT_FILE": "/parent"}}
	child := &NixConfig{Env: map[string]string{"NIX_BUILD_CORES": "8"}}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("8", child.Env["NIX_BUILD_CORES"], "child should override parent for same key")
	assertion.Equal("/parent", child.Env["NIX_SSL_CERT_FILE"], "child should inherit parent keys it doesn't define")
}

func Test_NixConfig_Init_Env_NilChildInheritsParent(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{Env: map[string]string{"NIX_BUILD_CORES": "4", "NIX_SSL_CERT_FILE": "/certs"}}
	child := &NixConfig{}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("4", child.Env["NIX_BUILD_CORES"], "nil child env should inherit all parent entries")
	assertion.Equal("/certs", child.Env["NIX_SSL_CERT_FILE"], "nil child env should inherit all parent entries")
}

func Test_NixConfig_Init_Env_NilParentPreservesChild(t *testing.T) {
	t.Parallel()

	child := &NixConfig{Env: map[string]string{"NIX_CONFIG": "max-jobs = 4"}}

	err := child.Init(nil)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("max-jobs = 4", child.Env["NIX_CONFIG"], "nil parent should not modify child env")
}

// --- Command-specific env inheritance ---

func Test_NixConfig_Init_CommandEnv_ChildOverridesParent(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{
		BuildEnv:        map[string]string{"NIX_BUILD_CORES": "4", "TMPDIR": "/parent-tmp"},
		NixosInstallEnv: map[string]string{"NIX_CONFIG": "max-jobs = 2"},
	}
	child := &NixConfig{
		BuildEnv:        map[string]string{"NIX_BUILD_CORES": "8"},
		NixosInstallEnv: map[string]string{},
	}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("8", child.BuildEnv["NIX_BUILD_CORES"], "child build env should override parent")
	assertion.Equal("/parent-tmp", child.BuildEnv["TMPDIR"], "child build env should inherit parent keys it doesn't define")
	// Empty env maps inherit parent entries: unlike flag lists where [] clears
	// the built-in defaults, env maps have no built-in defaults to clear.
	assertion.Equal("max-jobs = 2", child.NixosInstallEnv["NIX_CONFIG"], "empty child map should inherit parent entries")
}

func Test_NixConfig_Init_CommandEnv_NilChildInheritsParent(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{
		BuildEnv:        map[string]string{"NIX_BUILD_CORES": "4"},
		CopyEnv:         map[string]string{"NIX_CONFIG": "max-jobs = 2"},
		NixosInstallEnv: map[string]string{"TMPDIR": "/tmp"},
		ProfileAddEnv:   map[string]string{"NIX_SSL_CERT_FILE": "/certs"},
	}
	child := &NixConfig{}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("4", child.BuildEnv["NIX_BUILD_CORES"])
	assertion.Equal("max-jobs = 2", child.CopyEnv["NIX_CONFIG"])
	assertion.Equal("/tmp", child.NixosInstallEnv["TMPDIR"])
	assertion.Equal("/certs", child.ProfileAddEnv["NIX_SSL_CERT_FILE"])
}

func Test_NixConfig_Init_CommandEnv_GettersMergeInherited(t *testing.T) {
	t.Parallel()

	// After Init, the getter should merge inherited global env with inherited
	// command-specific env.
	parent := &NixConfig{
		Env:      map[string]string{"NIX_SSL_CERT_FILE": "/certs"},
		BuildEnv: map[string]string{"NIX_BUILD_CORES": "4"},
	}
	child := &NixConfig{BuildEnv: map[string]string{"NIX_BUILD_CORES": "8"}}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	// Global NIX_SSL_CERT_FILE inherited from parent, BuildEnv NIX_BUILD_CORES=8 (child overrides parent)
	assertion.Equal([]string{"NIX_BUILD_CORES=8", "NIX_SSL_CERT_FILE=/certs"}, child.GetBuildEnv())
	// Copy env: only global is inherited, no copy-specific
	assertion.Equal([]string{"NIX_SSL_CERT_FILE=/certs"}, child.GetCopyEnv())
}

// The whole fleet→flake→installable chain must keep one set of semantics: extra
// flags accumulate at every level, while a default flag list or command overrides.
// A regression here would append a parent command onto a child's executable.
func TestInit_ThreeLevelHierarchy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		get  func(*NixConfig) []string
		want []string
	}{
		{
			// mergo appends the parent after the child at every level, so the
			// installable's flags come first and the fleet's last.
			name: "extra flags accumulate from every level",
			get:  func(c *NixConfig) []string { return c.ExtraFlags },
			want: []string{"--installable", "--flake", "--fleet"},
		},
		{
			name: "an installable default flag list overrides both parents",
			get:  func(c *NixConfig) []string { return c.BuildDefaultFlags },
			want: []string{"--installable-build"},
		},
		{
			name: "an installable command overrides both parents",
			get:  func(c *NixConfig) []string { return c.BuildCommand },
			want: []string{"waybar-nom", "build"},
		},
		{
			name: "an unset child inherits the flake value",
			get:  func(c *NixConfig) []string { return c.CopyDefaultFlags },
			want: []string{"--flake-copy"},
		},
		{
			name: "a flake [] reaches the installable as pass nothing",
			get:  func(c *NixConfig) []string { return c.ProfileAddDefaultFlags },
			want: []string{},
		},
	}

	fleet := &NixConfig{ExtraFlags: []string{"--fleet"}}
	flake := &NixConfig{
		ExtraFlags:             []string{"--flake"},
		CopyDefaultFlags:       []string{"--flake-copy"},
		ProfileAddDefaultFlags: []string{},
	}
	child := &NixConfig{
		ExtraFlags:        []string{"--installable"},
		BuildDefaultFlags: []string{"--installable-build"},
		BuildCommand:      []string{"waybar-nom", "build"},
	}

	require.NoError(t, fleet.Init(nil))
	require.NoError(t, flake.Init(fleet))
	require.NoError(t, child.Init(flake))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.get(child))
		})
	}
}
