package footer

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
)

func TestSetKeyDefs(t *testing.T) {
	t.Parallel()

	modalDefs := []KeyDef{
		{Keys: []string{"enter"}, Help: "apply"},
		{Keys: []string{"esc"}, Help: "cancel"},
	}

	footer := New(testKeyDefs(), theme.DefaultTheme())

	before := buffer.LinesBufToStringForTests(footer.Render(80))
	assert.Contains(t, before, "force quit")

	footer.SetKeyDefs(modalDefs)

	assert.Equal(t, modalDefs, footer.KeyDefs(), "KeyDefs must report the swapped bindings")

	// Same width as before: without cache invalidation the old pixels are served.
	after := buffer.LinesBufToStringForTests(footer.Render(80))
	assert.NotEqual(t, before, after, "SetKeyDefs must visibly change the rendered footer")
	assert.Contains(t, after, "apply")
	assert.Contains(t, after, "cancel")
	assert.NotContains(t, after, "force quit")

	footer.SetKeyDefs(testKeyDefs())

	assert.Equal(t, buffer.LinesBufToStringForTests(footer.Render(80)), before,
		"restoring the original bindings must restore the original rendering")
}

// testKeyDefs returns a representative keymap containing the force quit binding
// the test swaps out for a modal keymap.
func testKeyDefs() []KeyDef {
	return []KeyDef{
		{Keys: []string{"q"}, Help: "quit"},
		{Keys: []string{"ctrl+q"}, Help: "force quit"},
	}
}
