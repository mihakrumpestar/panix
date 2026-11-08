package attributes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// newParentAttr builds a minimal parent Attributes suitable for Init: Xpath must
// be set because passAttributesInto calls NewXpathWithAppend on it.
func newParentAttr() *Attributes {
	return &Attributes{Xpath: xpath.New("fleet").NewXpathWithAppend("my-flake")}
}

// Built-in defaults live in the struct tags and are materialized by Init, so they
// are pinned literally here: a tag edit that changes a command or flag Panix
// depends on must fail a test.
func TestInit_MaterializesDefaults(t *testing.T) {
	t.Parallel()

	attrs := &Attributes{}

	require.NoError(t, attrs.Init("test", newParentAttr()))

	assert.Equal(t, []string{"rsync"}, attrs.RsyncCommand)
	assert.Equal(t, []string{"-rcEx", "--mkpath", "--perms"}, attrs.RsyncDefaultFlags)
	assert.Equal(t, []string{"nixos-generate-config"}, attrs.HardwareConfig.Generate.Command)
	assert.Equal(t, []string{"--show-hardware-config", "--no-filesystems"}, attrs.HardwareConfig.Generate.Args)
	assert.Equal(t, []string{"curl"}, attrs.Bootstrap.Nix.CurlCommand)
	assert.Equal(t, []string{"--fail", "-#", "-L"}, attrs.Bootstrap.Nix.CurlDefaultFlags)
	assert.Equal(t, []string{"install", "--no-confirm"}, attrs.Bootstrap.Nix.Args)
	assert.Equal(t, []string{"curl"}, attrs.Bootstrap.Kexec.CurlCommand)
	assert.Equal(t, []string{"--fail", "-#", "-L"}, attrs.Bootstrap.Kexec.CurlDefaultFlags)
	assert.Equal(t, []string{"tar"}, attrs.Bootstrap.Kexec.TarCommand)
}

// An explicit [] is a value the operator chose: a flag list passes no flags, and
// a command prefix stays empty. Validation rejects the empty prefix at load
// time, so it never reaches a command.
func TestInit_EmptyValues(t *testing.T) {
	t.Parallel()

	attrs := &Attributes{
		RsyncDefaultFlags: []string{},
		Bootstrap: Bootstrap{
			Nix:   NixConfig{CurlDefaultFlags: []string{}},
			Kexec: KexecConfig{CurlDefaultFlags: []string{}, TarCommand: []string{}},
		},
		HardwareConfig: HardwareConfig{Generate: Generator{Args: []string{}}},
	}

	require.NoError(t, attrs.Init("test", newParentAttr()))

	assert.NotNil(t, attrs.RsyncDefaultFlags)
	assert.Empty(t, attrs.RsyncDefaultFlags, "an explicitly empty flag list clears the default")
	assert.NotNil(t, attrs.Bootstrap.Nix.CurlDefaultFlags)
	assert.Empty(t, attrs.Bootstrap.Nix.CurlDefaultFlags)
	assert.NotNil(t, attrs.Bootstrap.Kexec.CurlDefaultFlags)
	assert.Empty(t, attrs.Bootstrap.Kexec.CurlDefaultFlags)
	assert.NotNil(t, attrs.HardwareConfig.Generate.Args)
	assert.Empty(t, attrs.HardwareConfig.Generate.Args, "[] runs a flag-less generator")

	assert.Empty(t, attrs.Bootstrap.Kexec.TarCommand, "an explicitly empty prefix stays empty")
}

func TestInit_KeepsConfiguredValues(t *testing.T) {
	t.Parallel()

	attrs := &Attributes{
		RsyncCommand: []string{"rsync-wrapper", "--quiet"},
		HardwareConfig: HardwareConfig{
			Generate: Generator{Command: []string{"/run/wrappers/bin/nixos-generate-config"}, Args: []string{"-o", "/tmp/x.json"}},
		},
		Bootstrap: Bootstrap{
			Nix:   NixConfig{CurlCommand: []string{"aria2c"}, Args: []string{"install", "--extra"}},
			Kexec: KexecConfig{CurlCommand: []string{"curl-kexec"}, TarCommand: []string{"bsdtar"}},
		},
	}

	require.NoError(t, attrs.Init("test", newParentAttr()))

	assert.Equal(t, []string{"rsync-wrapper", "--quiet"}, attrs.RsyncCommand)
	assert.Equal(t, []string{"/run/wrappers/bin/nixos-generate-config"}, attrs.HardwareConfig.Generate.Command)
	assert.Equal(t, []string{"-o", "/tmp/x.json"}, attrs.HardwareConfig.Generate.Args)
	assert.Equal(t, []string{"aria2c"}, attrs.Bootstrap.Nix.CurlCommand)
	assert.Equal(t, []string{"install", "--extra"}, attrs.Bootstrap.Nix.Args)
	assert.Equal(t, []string{"curl-kexec"}, attrs.Bootstrap.Kexec.CurlCommand)
	assert.Equal(t, []string{"bsdtar"}, attrs.Bootstrap.Kexec.TarCommand)
}

// --- RsyncDefaultFlags override semantics ---
//
// A child's non-nil flag list replaces the parent's; nil inherits it.

func Test_Attributes_Init_RsyncDefaultFlagsOverrideSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parent   []string
		child    []string
		expected []string
	}{
		{name: "child replaces parent", parent: []string{"--parent"}, child: []string{"--child"}, expected: []string{"--child"}},
		{name: "nil child inherits parent", parent: []string{"--parent"}, child: nil, expected: []string{"--parent"}},
		{name: "empty child clears parent", parent: []string{"--parent"}, child: []string{}, expected: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := newParentAttr()
			parent.RsyncDefaultFlags = tt.parent

			child := &Attributes{RsyncDefaultFlags: tt.child}

			require.NoError(t, child.Init("child", parent))

			assert.Equal(t, tt.expected, child.RsyncDefaultFlags)
		})
	}
}

// A fleet-level [] must reach an unset child. mergo cannot append [] into nil, so
// before overrides.InheritEmpty the fleet-level `args: []` was dropped and the
// machine fell back to the built-in generator flags.
func Test_Attributes_Init_EmptyParentValueReachesUnsetChild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		parentArgs []string
		childArgs  []string
		want       []string
	}{
		{name: "fleet empty reaches unset machine", parentArgs: []string{}, childArgs: nil, want: []string{}},
		{name: "fleet flags reach unset machine", parentArgs: []string{"--p"}, childArgs: nil, want: []string{"--p"}},
		{name: "machine value wins", parentArgs: []string{"--p"}, childArgs: []string{"-o", "/tmp/x"}, want: []string{"-o", "/tmp/x"}},
		{name: "machine empty wins", parentArgs: []string{"--p"}, childArgs: []string{}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := newParentAttr()
			parent.HardwareConfig.Generate.Args = tt.parentArgs

			child := &Attributes{HardwareConfig: HardwareConfig{Generate: Generator{Args: tt.childArgs}}}

			require.NoError(t, child.Init("c", parent))
			assert.Equal(t, tt.want, child.HardwareConfig.Generate.Args)
		})
	}
}

// A machine-level prefix overrides the fleet's instead of accumulating it: the
// nested fields ride the same override path as the top-level ones.
func TestAttributes_Init_CommandPrefixOverrideSemantics(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		RsyncCommand: []string{"parent-rsync"},
		HardwareConfig: HardwareConfig{
			Generate: Generator{Command: []string{"parent-generate"}, Args: []string{"parent-generate-args"}},
		},
		Bootstrap: Bootstrap{
			Kexec: KexecConfig{
				CurlDefaultFlags: []string{"--parent-kexec-curl"},
				CurlCommand:      []string{"parent-kexec-curl"},
				TarCommand:       []string{"parent-tar"},
			},
			Nix: NixConfig{
				Args:             []string{"parent-args"},
				CurlDefaultFlags: []string{"--parent-nix-curl"},
				CurlCommand:      []string{"parent-nix-curl"},
			},
		},
		Xpath: xpath.New("fleet").NewXpathWithAppend("my-flake"),
	}
	child := &Attributes{
		RsyncCommand: []string{"child-rsync"},
		Bootstrap:    Bootstrap{Nix: NixConfig{Args: []string{"child-args"}}},
	}

	require.NoError(t, child.Init("child", parent))

	assert.Equal(t, []string{"child-rsync"}, child.RsyncCommand, "child prefix wins")
	assert.Equal(t, []string{"parent-generate"}, child.HardwareConfig.Generate.Command, "nil child prefix inherits the parent")
	assert.Equal(t, []string{"parent-nix-curl"}, child.Bootstrap.Nix.CurlCommand, "nil child prefix inherits the parent")
	assert.Equal(t, []string{"parent-kexec-curl"}, child.Bootstrap.Kexec.CurlCommand, "nil child prefix inherits the parent")
	assert.Equal(t, []string{"parent-tar"}, child.Bootstrap.Kexec.TarCommand, "nil child prefix inherits the parent")

	// The bootstrap flag lists ride the same override path.
	assert.Equal(t, []string{"--parent-kexec-curl"}, child.Bootstrap.Kexec.CurlDefaultFlags)
	assert.Equal(t, []string{"child-args"}, child.Bootstrap.Nix.Args, "child args win")
	assert.Equal(t, []string{"--parent-nix-curl"}, child.Bootstrap.Nix.CurlDefaultFlags)
	assert.Equal(t, []string{"parent-generate-args"}, child.HardwareConfig.Generate.Args,
		"a nil child generator args list inherits the parent")
}

// --- Scalar defaults ---

func Test_NixConfig_GetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   NixConfig
		want string
	}{
		{"empty defaults to the tagged default", NixConfig{}, "https://install.determinate.systems/nix"},
		{"custom url preserved", NixConfig{URL: "https://example.com/installer"}, "https://example.com/installer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.GetURL())
		})
	}
}
