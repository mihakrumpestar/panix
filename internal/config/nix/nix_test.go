package nix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GetExperimentalFeatures ---

func Test_GetExperimentalFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default experimental features",
			in:   nil,
			want: DefaultExperimentalFeatures,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--extra-experimental-features", "ca-derivations"},
			want: []string{"--extra-experimental-features", "ca-derivations"},
		},
		{
			name: "explicitly empty clears defaults",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &NixConfig{ExperimentalFeatures: tt.in}

			assertion := assert.New(t)
			assertion.Equal(tt.want, c.GetExperimentalFeatures())
		})
	}
}

// --- GetBuildDefaultFlags ---

func Test_GetBuildDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default build flags",
			in:   nil,
			want: DefaultBuildFlags,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--max-jobs", "4"},
			want: []string{"--max-jobs", "4"},
		},
		{
			name: "explicitly empty clears defaults",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &NixConfig{BuildDefaultFlags: tt.in}

			assertion := assert.New(t)
			assertion.Equal(tt.want, c.GetBuildDefaultFlags())
		})
	}
}

// --- GetCopyDefaultFlags ---

func Test_GetCopyDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default copy flags",
			in:   nil,
			want: DefaultCopyFlags,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--compress"},
			want: []string{"--compress"},
		},
		{
			name: "explicitly empty clears defaults",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &NixConfig{CopyDefaultFlags: tt.in}

			assertion := assert.New(t)
			assertion.Equal(tt.want, c.GetCopyDefaultFlags())
		})
	}
}

// --- GetNixosInstallDefaultFlags ---

func Test_GetNixosInstallDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default nixos-install flags",
			in:   nil,
			want: DefaultNixosInstallFlags,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--no-bootloader"},
			want: []string{"--no-bootloader"},
		},
		{
			name: "explicitly empty clears defaults",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &NixConfig{NixosInstallDefaultFlags: tt.in}

			assertion := assert.New(t)
			assertion.Equal(tt.want, c.GetNixosInstallDefaultFlags())
		})
	}
}

// --- GetProfileInstallDefaultFlags ---

func Test_GetProfileInstallDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns nil (no built-in defaults)",
			in:   nil,
			want: nil,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--priority", "4"},
			want: []string{"--priority", "4"},
		},
		{
			name: "explicitly empty returns empty (not nil)",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &NixConfig{ProfileInstallDefaultFlags: tt.in}

			assertion := assert.New(t)
			assertion.Equal(tt.want, c.GetProfileInstallDefaultFlags())
		})
	}
}

// --- Init override semantics ---
//
// "Default" flag fields use override semantics: a child's non-nil value is
// kept, and a nil child inherits the parent's value. "Extra" flag fields use
// append semantics: parent values are appended to the child's.

func Test_NixConfig_Init_DefaultFlags_ChildOverridesParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		child    *NixConfig
		parent   *NixConfig
		expected []string
		check    func(*NixConfig) []string
	}{
		{
			name:     "BuildDefaultFlags",
			child:    &NixConfig{BuildDefaultFlags: []string{"--child-flag"}},
			parent:   &NixConfig{BuildDefaultFlags: []string{"--parent-flag"}},
			expected: []string{"--child-flag"},
			check:    func(cfg *NixConfig) []string { return cfg.BuildDefaultFlags },
		},
		{
			name:     "ExperimentalFeatures",
			child:    &NixConfig{ExperimentalFeatures: []string{"--child-exp"}},
			parent:   &NixConfig{ExperimentalFeatures: []string{"--parent-exp"}},
			expected: []string{"--child-exp"},
			check:    func(cfg *NixConfig) []string { return cfg.ExperimentalFeatures },
		},
		{
			name:     "CopyDefaultFlags",
			child:    &NixConfig{CopyDefaultFlags: []string{"--child-copy"}},
			parent:   &NixConfig{CopyDefaultFlags: []string{"--parent-copy"}},
			expected: []string{"--child-copy"},
			check:    func(cfg *NixConfig) []string { return cfg.CopyDefaultFlags },
		},
		{
			name:     "NixosInstallDefaultFlags",
			child:    &NixConfig{NixosInstallDefaultFlags: []string{"--child-nixos"}},
			parent:   &NixConfig{NixosInstallDefaultFlags: []string{"--parent-nixos"}},
			expected: []string{"--child-nixos"},
			check:    func(cfg *NixConfig) []string { return cfg.NixosInstallDefaultFlags },
		},
		{
			name:     "ProfileInstallDefaultFlags",
			child:    &NixConfig{ProfileInstallDefaultFlags: []string{"--child-profile"}},
			parent:   &NixConfig{ProfileInstallDefaultFlags: []string{"--parent-profile"}},
			expected: []string{"--child-profile"},
			check:    func(cfg *NixConfig) []string { return cfg.ProfileInstallDefaultFlags },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.child.Init(tt.parent)
			require.NoError(t, err)

			assertion := assert.New(t)
			assertion.Equal(tt.expected, tt.check(tt.child),
				"child's %s should override parent, not append", tt.name)
		})
	}
}

func Test_NixConfig_Init_DefaultFlags_NilChildInheritsParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parent   *NixConfig
		expected []string
		check    func(*NixConfig) []string
	}{
		{
			name:     "BuildDefaultFlags",
			parent:   &NixConfig{BuildDefaultFlags: []string{"--parent"}},
			expected: []string{"--parent"},
			check:    func(cfg *NixConfig) []string { return cfg.BuildDefaultFlags },
		},
		{
			name:     "ExperimentalFeatures",
			parent:   &NixConfig{ExperimentalFeatures: []string{"--parent-exp"}},
			expected: []string{"--parent-exp"},
			check:    func(cfg *NixConfig) []string { return cfg.ExperimentalFeatures },
		},
		{
			name:     "CopyDefaultFlags",
			parent:   &NixConfig{CopyDefaultFlags: []string{"--parent"}},
			expected: []string{"--parent"},
			check:    func(cfg *NixConfig) []string { return cfg.CopyDefaultFlags },
		},
		{
			name:     "NixosInstallDefaultFlags",
			parent:   &NixConfig{NixosInstallDefaultFlags: []string{"--parent"}},
			expected: []string{"--parent"},
			check:    func(cfg *NixConfig) []string { return cfg.NixosInstallDefaultFlags },
		},
		{
			name:     "ProfileInstallDefaultFlags",
			parent:   &NixConfig{ProfileInstallDefaultFlags: []string{"--parent"}},
			expected: []string{"--parent"},
			check:    func(cfg *NixConfig) []string { return cfg.ProfileInstallDefaultFlags },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			child := &NixConfig{}
			err := child.Init(tt.parent)
			require.NoError(t, err)

			assertion := assert.New(t)
			assertion.Equal(tt.expected, tt.check(child),
				"nil child %s should inherit parent value", tt.name)
		})
	}
}

func Test_NixConfig_Init_ExtraFlags_AppendSemantics(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{ExtraFlags: []string{"--parent-extra"}}
	child := &NixConfig{ExtraFlags: []string{"--child-extra"}}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal([]string{"--child-extra", "--parent-extra"}, child.ExtraFlags,
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
		NixosInstallEnv:   map[string]string{"NIX_CONFIG": "max-jobs = 1"},
		ProfileInstallEnv: map[string]string{},
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
			name: "GetProfileInstallEnv: global only (specific is empty map)",
			got:  nixC.GetProfileInstallEnv(),
			want: []string{"NIX_BUILD_CORES=4", "NIX_SSL_CERT_FILE=/global"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.got)
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
	assertion.Nil(nixC.GetProfileInstallEnv())
}

func Test_CommandEnvGetters_GlobalOnly(t *testing.T) {
	t.Parallel()

	nixC := &NixConfig{Env: map[string]string{"NIX_SSL_CERT_FILE": "/certs"}}

	want := []string{"NIX_SSL_CERT_FILE=/certs"}

	assertion := assert.New(t)
	assertion.Equal(want, nixC.GetBuildEnv())
	assertion.Equal(want, nixC.GetCopyEnv())
	assertion.Equal(want, nixC.GetNixosInstallEnv())
	assertion.Equal(want, nixC.GetProfileInstallEnv())
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
	// Empty env maps inherit parent entries — unlike default-flag slices where
	// [] clears built-in defaults, env maps have no built-in defaults to clear.
	assertion.Equal("max-jobs = 2", child.NixosInstallEnv["NIX_CONFIG"], "empty child map should inherit parent entries")
}

func Test_NixConfig_Init_CommandEnv_NilChildInheritsParent(t *testing.T) {
	t.Parallel()

	parent := &NixConfig{
		BuildEnv:          map[string]string{"NIX_BUILD_CORES": "4"},
		CopyEnv:           map[string]string{"NIX_CONFIG": "max-jobs = 2"},
		NixosInstallEnv:   map[string]string{"TMPDIR": "/tmp"},
		ProfileInstallEnv: map[string]string{"NIX_SSL_CERT_FILE": "/certs"},
	}
	child := &NixConfig{}

	err := child.Init(parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("4", child.BuildEnv["NIX_BUILD_CORES"])
	assertion.Equal("max-jobs = 2", child.CopyEnv["NIX_CONFIG"])
	assertion.Equal("/tmp", child.NixosInstallEnv["TMPDIR"])
	assertion.Equal("/certs", child.ProfileInstallEnv["NIX_SSL_CERT_FILE"])
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
