package attributes

import (
	"testing"
	"time"

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

// --- Rollback tier ---

func TestRollbackGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Rollback
		want Rollback
	}{
		{name: "unset resolves to off", in: "", want: RollbackOff},
		{name: "off preserved", in: RollbackOff, want: RollbackOff},
		{name: "auto preserved", in: RollbackAuto, want: RollbackAuto},
		{name: "magic preserved", in: RollbackMagic, want: RollbackMagic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.Get())
			assert.Equal(t, tt.want.String(), tt.in.String())
		})
	}
}

func TestRollbackIsGuarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Rollback
		want bool
	}{
		{name: "unset is not guarded", in: "", want: false},
		{name: "off is not guarded", in: RollbackOff, want: false},
		{name: "auto is guarded", in: RollbackAuto, want: true},
		{name: "magic is guarded", in: RollbackMagic, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.in.IsGuarded())
		})
	}
}

// --- Activation Guard timeouts ---

func TestGetActivationTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "unset returns default", in: 0, want: DefaultActivationTimeout},
		{name: "explicit value preserved", in: 30 * time.Minute, want: 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := &Attributes{ActivationTimeout: tt.in}

			assert.Equal(t, tt.want, a.GetActivationTimeout())
		})
	}
}

func TestGetRollbackConfirmTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "unset returns default", in: 0, want: DefaultRollbackConfirmTimeout},
		{name: "explicit value preserved", in: 90 * time.Second, want: 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := &Attributes{RollbackConfirmTimeout: tt.in}

			assert.Equal(t, tt.want, a.GetRollbackConfirmTimeout())
		})
	}
}
