package overrides

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Capture restores a set child value and leaves a nil one alone: nil means
// "inherit from the parent", so there is nothing to write back.
func TestCaptureRestore(t *testing.T) {
	t.Parallel()

	child := []string{"child"}
	inherited := []string(nil)

	captured := Capture(&child, &inherited, nil)

	// Simulate mergo.WithAppendSlice.
	child = append(child, "parent")
	inherited = append(inherited, "parent")

	captured.Restore()

	assert.Equal(t, []string{"child"}, child)
	assert.Equal(t, []string{"parent"}, inherited, "nil fields are not captured")
}

// An empty parent value must reach an unset child: mergo cannot append [] into
// nil, so without InheritEmpty a parent `args: []` would be silently discarded
// and the child would fall back to the built-in default.
func TestInheritEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		child  []string
		parent []string
		want   []string
	}{
		{name: "unset child inherits empty parent", child: nil, parent: []string{}, want: []string{}},
		{name: "unset child inherits set parent", child: nil, parent: []string{"p"}, want: []string{"p"}},
		{name: "set child is untouched", child: []string{}, parent: []string{"p"}, want: []string{}},
		{name: "unset parent leaves child nil", child: nil, parent: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			child, parent := tt.child, tt.parent

			InheritEmpty([]*[]string{&child}, []*[]string{&parent})

			assert.Equal(t, tt.want, child)
		})
	}
}

// A mismatched field list is a programming error and must fail loudly: copying
// by index would otherwise write a parent value into the wrong field.
func TestInheritEmptyLengthMismatchPanics(t *testing.T) {
	t.Parallel()

	child := []string(nil)
	other := []string(nil)

	assert.Panics(t, func() {
		InheritEmpty([]*[]string{&child, &other}, []*[]string{&other})
	})
}
