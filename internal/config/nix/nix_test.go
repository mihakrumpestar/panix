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
