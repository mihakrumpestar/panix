package attributes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// --- GetRsyncDefaultFlags ---

func Test_GetRsyncDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default rsync flags",
			in:   nil,
			want: DefaultRsyncFlags,
		},
		{
			name: "custom value returns custom",
			in:   []string{"-avz", "--delete"},
			want: []string{"-avz", "--delete"},
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

			a := &Attributes{RsyncDefaultFlags: tt.in}

			assertion := assert.New(t)
			got := a.GetRsyncDefaultFlags()
			assertion.Equal(tt.want, got)
		})
	}
}

// --- GetCurlDefaultFlags ---
//
// GetCurlDefaultFlags is a method on KexecConfig.

func Test_GetCurlDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil returns default curl flags",
			in:   nil,
			want: DefaultCurlFlags,
		},
		{
			name: "custom value returns custom",
			in:   []string{"--silent", "--show-error"},
			want: []string{"--silent", "--show-error"},
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

			k := &KexecConfig{CurlDefaultFlags: tt.in}

			assertion := assert.New(t)
			got := k.GetCurlDefaultFlags()
			assertion.Equal(tt.want, got)
		})
	}
}

// --- Attributes.Init rsync override semantics ---
//
// "Default" flag fields (RsyncDefaultFlags) use override semantics: a child's
// non-nil value is kept; a nil child inherits the parent's value.

func Test_Attributes_Init_RsyncOverrideSemantics(t *testing.T) {
	t.Parallel()

	// newParentAttr builds a minimal parent Attributes suitable for Init:
	// Xpath must be set because passAttributesInto calls NewXpathWithAppend on it.
	newParentAttr := func(rsyncFlags []string) *Attributes {
		return &Attributes{
			RsyncDefaultFlags: rsyncFlags,
			Xpath:             xpath.New("fleet").NewXpathWithAppend("my-flake"),
		}
	}

	t.Run("RsyncDefaultFlags child overrides parent", func(t *testing.T) {
		t.Parallel()

		parent := newParentAttr([]string{"--parent-rsync"})
		child := &Attributes{
			RsyncDefaultFlags: []string{"--child-rsync"},
		}

		err := child.Init("child", parent)
		require.NoError(t, err)

		assertion := assert.New(t)
		assertion.Equal([]string{"--child-rsync"}, child.RsyncDefaultFlags,
			"child's RsyncDefaultFlags should override parent, not append")
	})

	t.Run("RsyncDefaultFlags nil child inherits parent", func(t *testing.T) {
		t.Parallel()

		parent := newParentAttr([]string{"--parent"})
		child := &Attributes{}

		err := child.Init("child", parent)
		require.NoError(t, err)

		assertion := assert.New(t)
		assertion.Equal([]string{"--parent"}, child.RsyncDefaultFlags,
			"nil child RsyncDefaultFlags should inherit parent value")
	})
}

// --- NixConfig.GetURL / GetArgs / GetCurlDefaultFlags ---
//
// Scalar defaults come from the struct tags (shared with schema generation),
// list defaults from the Default* vars; a nil receiver (unset bootstrap.nix)
// behaves like an empty config.

func Test_NixConfig_GetURL(t *testing.T) {
	t.Parallel()

	defaultURL := "https://install.determinate.systems/nix"

	tests := []struct {
		name string
		in   *NixConfig
		want string
	}{
		{"nil defaults to the tagged default", nil, defaultURL},
		{"empty defaults to the tagged default", &NixConfig{}, defaultURL},
		{"custom url preserved", &NixConfig{URL: "https://example.com/installer"}, "https://example.com/installer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.GetURL())
		})
	}
}

func Test_NixConfig_GetArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *NixConfig
		want []string
	}{
		{"nil defaults to DefaultNixInstallerArgs", nil, DefaultNixInstallerArgs},
		{"nil args default to DefaultNixInstallerArgs", &NixConfig{URL: "https://example.com/i"}, DefaultNixInstallerArgs},
		{"custom args preserved", &NixConfig{Args: []string{"install", "--extra"}}, []string{"install", "--extra"}},
		{"explicit empty args clear the default", &NixConfig{Args: []string{}}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.GetArgs())
		})
	}
}

func Test_NixConfig_GetCurlDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *NixConfig
		want []string
	}{
		{"nil returns default curl flags", nil, DefaultCurlFlags},
		{"nil flags return default curl flags", &NixConfig{}, DefaultCurlFlags},
		{"custom value returns custom", &NixConfig{CurlDefaultFlags: []string{"-fsSL"}}, []string{"-fsSL"}},
		{"explicitly empty clears defaults", &NixConfig{CurlDefaultFlags: []string{}}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.GetCurlDefaultFlags())
		})
	}
}

// --- Attributes.Init bootstrap.nix override semantics ---
//
// Bootstrap.Nix.Args is a "default" flag-style field: a child's non-nil value
// fully replaces the parent's; a nil child inherits it.

func Test_Attributes_Init_NixArgsOverrideSemantics(t *testing.T) {
	t.Parallel()

	// newParentAttr builds a minimal parent Attributes suitable for Init:
	// Xpath must be set because passAttributesInto calls NewXpathWithAppend on it.
	newParentAttr := func(nixCfg *NixConfig) *Attributes {
		return &Attributes{
			Bootstrap: Bootstrap{Nix: nixCfg},
			Xpath:     xpath.New("fleet").NewXpathWithAppend("my-flake"),
		}
	}

	t.Run("child args override parent args", func(t *testing.T) {
		t.Parallel()

		parent := newParentAttr(&NixConfig{URL: "https://parent.example.com/i", Args: []string{"parent"}})
		child := &Attributes{Bootstrap: Bootstrap{Nix: &NixConfig{Args: []string{"child"}}}}

		err := child.Init("child", parent)
		require.NoError(t, err)

		require.NotNil(t, child.Bootstrap.Nix)
		assert.Equal(t, []string{"child"}, child.Bootstrap.Nix.Args,
			"child's Args should replace parent's, not append")
		assert.Equal(t, "https://parent.example.com/i", child.Bootstrap.Nix.URL,
			"unset fields still cascade from the parent")
	})

	t.Run("nil child Nix inherits parent", func(t *testing.T) {
		t.Parallel()

		parent := newParentAttr(&NixConfig{URL: "https://parent.example.com/i", Args: []string{"parent"}})
		child := &Attributes{}

		err := child.Init("child", parent)
		require.NoError(t, err)

		require.NotNil(t, child.Bootstrap.Nix)
		assert.Equal(t, "https://parent.example.com/i", child.Bootstrap.Nix.GetURL())
		assert.Equal(t, []string{"parent"}, child.Bootstrap.Nix.GetArgs())
	})

	t.Run("nil child args inherit parent args", func(t *testing.T) {
		t.Parallel()

		parent := newParentAttr(&NixConfig{Args: []string{"parent"}})
		child := &Attributes{Bootstrap: Bootstrap{Nix: &NixConfig{URL: "https://child.example.com/i"}}}

		err := child.Init("child", parent)
		require.NoError(t, err)

		require.NotNil(t, child.Bootstrap.Nix)
		assert.Equal(t, []string{"parent"}, child.Bootstrap.Nix.Args,
			"nil child Args should inherit parent value")
	})
}
