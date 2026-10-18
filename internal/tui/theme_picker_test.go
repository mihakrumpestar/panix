package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Theme names in the sorted built-in registry order the picker lists: the tests
// open on the first built-in and browse the next two. Derived so they stay valid
// when built-ins are added.
var (
	themeNameFirst = builtinNameAt(0)
	themeNameNext  = builtinNameAt(1)
	themeNameLast  = builtinNameAt(2)
)

// builtinNameAt returns the built-in theme name at index in the picker order.
func builtinNameAt(index int) string {
	return theme.NewRegistry().Names()[index]
}

func mustOpenThemePicker(t *testing.T, mdl *model) {
	t.Helper()

	mdl.Update(zeroterm.KeyPressMsg{Key: "ctrl+t"})

	require.True(t, mdl.themePicker.open, "ctrl+t should open the theme picker")
}

func TestThemePickerKeyDefRegistered(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)

	def, found := findKeyDef(mdl.footer.KeyDefs(), "ctrl+t")

	require.True(t, found, "ctrl+t must be registered in the footer keydefs")
	assert.Equal(t, []string{"ctrl+t"}, def.Keys)
	assert.Equal(t, "themes", def.Help)
	require.NotNil(t, def.Handler)
	require.NotNil(t, def.Active)
	assert.False(t, def.Active(), "picker closed: keydef inactive")

	mdl.themePicker.open = true

	assert.True(t, def.Active(), "picker open: keydef active")
}

func findKeyDef(defs []footer.KeyDef, key string) (footer.KeyDef, bool) {
	for _, kd := range defs {
		if len(kd.Keys) > 0 && kd.Keys[0] == key {
			return kd, true
		}
	}

	return footer.KeyDef{}, false
}

func keyDefKeys(defs []footer.KeyDef) []string {
	keys := make([]string, len(defs))
	for idx := range defs {
		keys[idx] = defs[idx].Keys[0]
	}

	return keys
}

func keyDefHelps(defs []footer.KeyDef) []string {
	helps := make([]string, len(defs))
	for idx := range defs {
		helps[idx] = defs[idx].Help
	}

	return helps
}

// themedWidgetOutputs returns the per-widget themed output without the footer: the
// footer keymap and notification change while quitting, while every other themed
// widget must render the reverted preview exactly like before the picker opened.
func themedWidgetOutputs(mdl *model) map[string]string {
	outputs := widgetOutputs(mdl, mdl.dimensions.Width)
	delete(outputs, "footer")

	return outputs
}

// widgetOutputs renders the model's theme-dependent widgets, keyed by name, so
// tests can compare the themed surface widget by widget. Live workflow caches are
// recomputed first so the capture does not depend on a prior render.
func widgetOutputs(mdl *model, width int) map[string]string {
	mdl.conf.Fleet.Recalculate(mdl.conf.Phases)

	render := func(linesBuf *buffer.LinesBuf) string {
		if linesBuf == nil {
			return ""
		}

		return buffer.LinesBufToStringForTests(linesBuf)
	}

	return map[string]string{
		"header": render(mdl.header.Render(width)),
		"footer": render(mdl.footer.Render(width)),
		"stats":  render(mdl.statsTable.Render(width)),
	}
}

func TestThemePickerToggleOpensAndCloses(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)
	before := renderModelFrame(mdl)

	mdl.Update(zeroterm.KeyPressMsg{Key: "ctrl+t"})

	require.True(t, mdl.themePicker.open)
	assert.Equal(t, themeNameFirst, mdl.themePicker.prevName)
	assert.Equal(t, themeNameFirst, mdl.conf.Theme.Name, "opening must not change the theme")

	frame := renderModelFrame(mdl)
	assert.Contains(t, frame, themePickerTitle)
	assert.Contains(t, frame, themeActiveMarker)
	assert.Contains(t, frame, themeNameLast, "the theme list must render")

	// ctrl+t toggles like esc: revert and close.
	mdl.Update(zeroterm.KeyPressMsg{Key: "ctrl+t"})

	assert.False(t, mdl.themePicker.open)
	assert.Equal(t, before, renderModelFrame(mdl), "closed picker must render byte-identically to before opening")
}

// TestThemePickerFooterShowsPickerKeybinds pins the footer keymap to the picker
// context: the picker keybinds plus the quitting keybinds while it owns the
// keyboard, the global bindings again once it closes.
func TestThemePickerFooterShowsPickerKeybinds(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)

	assert.Contains(t, keyDefHelps(mdl.footer.KeyDefs()), "quit", "closed picker: global bindings")

	mustOpenThemePicker(t, mdl)

	frame := renderModelFrame(mdl)

	assert.Equal(t, []string{"enter", "up/down", "esc", "q", "ctrl+q"}, keyDefKeys(mdl.footer.KeyDefs()))
	assert.Equal(t, []string{"apply", "select", "cancel", "quit", "force quit"}, keyDefHelps(mdl.footer.KeyDefs()))
	assert.Contains(t, frame, "apply")
	assert.Contains(t, frame, "select")
	assert.Contains(t, frame, "cancel")
	assert.Contains(t, frame, "quit")
	assert.Contains(t, frame, "force quit")

	// The quitting entries stay display-only while the picker owns the keyboard:
	// the picker routes them itself and modal footer entries are never dispatched.
	for _, key := range []string{quitKey, forceQuitKey} {
		def, found := findKeyDef(mdl.footer.KeyDefs(), key)
		require.True(t, found, "the quitting keybind %q must stay visible in the picker footer", key)
		assert.Nil(t, def.Handler, "modal footer entries must stay handler-less")
	}

	mdl.Update(zeroterm.KeyPressMsg{Key: "esc"})

	frame = renderModelFrame(mdl)

	assert.Equal(t, keyDefKeys(mdl.keyDefs()), keyDefKeys(mdl.footer.KeyDefs()), "closed picker: global bindings restored")
	assert.Equal(t, keyDefHelps(mdl.keyDefs()), keyDefHelps(mdl.footer.KeyDefs()), "closed picker: global bindings restored")
	assert.Contains(t, frame, "quit", "the footer must render the global keybinds again")
}

// TestQuittingKeyDefsSharedSource pins the single source of truth for the quitting
// keybinds: the picker footer and the global footer get their quitting entries from
// the same builder, so keys and help texts cannot drift apart.
func TestQuittingKeyDefsSharedSource(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)

	global := mdl.keyDefs()
	picker := mdl.modalKeyDefs(modalThemePicker)

	// The picker footer appends the shared quitting defs, display-only.
	assert.Equal(t, displayOnlyKeyDefs(mdl.quittingKeyDefs()), picker[len(picker)-2:])

	for _, key := range []string{quitKey, forceQuitKey} {
		globalDef, found := findKeyDef(global, key)
		require.True(t, found, "the global keymap must bind %q", key)

		pickerDef, found := findKeyDef(picker, key)
		require.True(t, found, "the picker footer must show %q", key)

		assert.Equal(t, globalDef.Keys, pickerDef.Keys, "both keymaps must reference the same quitting keys")
		assert.Equal(t, globalDef.Help, pickerDef.Help, "both keymaps must reference the same quitting helps")
		assert.NotNil(t, globalDef.Handler, "the global quitting defs must dispatch")
		assert.Nil(t, pickerDef.Handler, "the picker footer quitting defs must stay handler-less")
	}
}

//nolint:funlen // large live preview case table
func TestThemePickerSelectionLivePreview(t *testing.T) {
	t.Parallel()

	wheelDown := zeroterm.MouseWheelMsg{Button: zeroterm.MouseWheelDown}
	wheelUp := zeroterm.MouseWheelMsg{Button: zeroterm.MouseWheelUp}

	tests := []struct {
		name          string
		msgs          []zeroterm.Msg
		wantTheme     string
		wantRenderNew bool
	}{
		{
			name:          "down previews the next theme",
			msgs:          []zeroterm.Msg{zeroterm.KeyPressMsg{Key: "down"}},
			wantTheme:     themeNameNext,
			wantRenderNew: true,
		},
		{
			name:          "j previews the next theme",
			msgs:          []zeroterm.Msg{zeroterm.KeyPressMsg{Key: "j"}},
			wantTheme:     themeNameNext,
			wantRenderNew: true,
		},
		{
			name:          "up clamps at the top",
			msgs:          []zeroterm.Msg{zeroterm.KeyPressMsg{Key: "up"}},
			wantTheme:     themeNameFirst,
			wantRenderNew: false,
		},
		{
			name: "k returns to the previous theme",
			msgs: []zeroterm.Msg{
				zeroterm.KeyPressMsg{Key: "down"},
				zeroterm.KeyPressMsg{Key: "k"},
			},
			wantTheme:     themeNameFirst,
			wantRenderNew: false,
		},
		{
			name: "down twice previews the last theme",
			msgs: []zeroterm.Msg{
				zeroterm.KeyPressMsg{Key: "down"},
				zeroterm.KeyPressMsg{Key: "down"},
			},
			wantTheme:     themeNameLast,
			wantRenderNew: true,
		},
		{
			name:          "wheel down previews the next theme",
			msgs:          []zeroterm.Msg{wheelDown},
			wantTheme:     themeNameNext,
			wantRenderNew: true,
		},
		{
			name:          "wheel up clamps at the top",
			msgs:          []zeroterm.Msg{wheelUp},
			wantTheme:     themeNameFirst,
			wantRenderNew: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mdl := themeOverlayModel(t, themeConfigContent)
			mustOpenThemePicker(t, mdl)

			beforeScheme := mdl.theme
			before := renderModelFrame(mdl)

			for _, msg := range tt.msgs {
				mdl.Update(msg)
			}

			assert.Equal(t, tt.wantTheme, mdl.conf.Theme.Name, "Theme.Name must follow the previewed theme")
			require.True(t, mdl.themePicker.open, "moving the selection must keep the picker open")

			if tt.wantRenderNew {
				assert.NotSame(t, beforeScheme, mdl.theme, "preview must build a fresh scheme")
				assert.NotEqual(t, before, renderModelFrame(mdl), "preview must restyle the rendered frame")
			} else {
				assert.Equal(t, before, renderModelFrame(mdl), "an unmoved selection must not restyle the frame")
			}
		})
	}
}

func TestThemePickerEscRevertsAndCloses(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)
	before := renderModelFrame(mdl)

	mustOpenThemePicker(t, mdl)
	mdl.Update(zeroterm.KeyPressMsg{Key: "down"})

	require.Equal(t, themeNameNext, mdl.conf.Theme.Name, "precondition: preview moved")
	require.NotEqual(t, before, renderModelFrame(mdl), "precondition: preview restyled the frame")

	mdl.Update(zeroterm.KeyPressMsg{Key: "esc"})

	assert.False(t, mdl.themePicker.open)
	assert.Equal(t, themeNameFirst, mdl.conf.Theme.Name, "esc must restore the pre-open theme")
	assert.Equal(t, before, renderModelFrame(mdl), "esc must restore the pre-open rendering byte-identically")
	assert.False(t, mdl.quitting, "esc must cancel without quitting")
}

// TestThemePickerEnterAppliesAndCloses pins the enter contract: the chosen theme
// stays applied for the session, the picker closes, the global keybinds dispatch
// again, and nothing is ever written back to the config file.
func TestThemePickerEnterAppliesAndCloses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "plain config stays untouched", config: themeConfigContent},
		{name: "templated config stays untouched", config: "name: {{ .X }}\n" + themeConfigContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mdl := themeOverlayModel(t, tt.config)
			mustOpenThemePicker(t, mdl)
			mdl.Update(zeroterm.KeyPressMsg{Key: "down"})
			require.Equal(t, themeNameNext, mdl.conf.Theme.Name, "precondition: preview moved")

			mdl.Update(zeroterm.KeyPressMsg{Key: "enter"})

			assert.False(t, mdl.themePicker.open, "enter must close the picker")
			assert.Equal(t, themeNameNext, mdl.conf.Theme.Name, "enter must keep the previewed theme")
			assert.Equal(t, themeNameNext, mdl.conf.Flags.Tui.Theme, "enter must pin the theme for the session")
			assert.False(t, mdl.quitting, "enter must apply without quitting")

			raw, err := os.ReadFile(mdl.conf.Flags.Config) //nolint:gosec // test writes its own temp file
			require.NoError(t, err)
			assert.Equal(t, tt.config, string(raw), "enter must never write back to the config file")

			mdl.Update(zeroterm.KeyPressMsg{Key: "h"})
			assert.True(t, mdl.conf.Flags.Tui.ShowAllBuildLogs, "global keybinds must dispatch again after the close")
		})
	}
}

// TestThemePickerQuittingKeys pins that the picker never blocks the quitting
// keybinds: the esc teardown runs first (close + revert the live preview) and then
// the global quit handler, so a deferred quit cannot leave the preview behind. The
// observable must match the same key routed without the picker open.
func TestThemePickerQuittingKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		wantQuitMsg bool // the returned cmd quits immediately instead of deferring
	}{
		{name: "q quits gracefully", key: quitKey, wantQuitMsg: false},
		{name: "ctrl+q force quits", key: forceQuitKey, wantQuitMsg: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mdl := themeOverlayModel(t, themeConfigContent)
			beforeTheme := mdl.conf.Theme.Name
			beforeWidgets := themedWidgetOutputs(mdl)

			mustOpenThemePicker(t, mdl)
			mdl.Update(zeroterm.KeyPressMsg{Key: "down"})

			require.Equal(t, themeNameNext, mdl.conf.Theme.Name, "precondition: the live preview moved")
			require.True(t, mdl.themePicker.open)

			cmd := mdl.Update(zeroterm.KeyPressMsg{Key: tt.key})

			assert.False(t, mdl.themePicker.open, "the picker must close like esc before the quit handler runs")
			assert.Equal(t, beforeTheme, mdl.conf.Theme.Name, "the live preview must revert like esc")
			assert.Equal(t, beforeWidgets, themedWidgetOutputs(mdl), "the reverted preview must render like before opening")
			assert.True(t, mdl.quitting, "the quitting keybind must reach the global quit handler")

			require.NotNil(t, cmd, "the quit handler cmd must flow out of Update")

			msg := cmd()
			_, isQuit := msg.(zeroterm.QuitMsg)
			assert.Equal(t, tt.wantQuitMsg, isQuit, "the cmd must match the global quit behavior")

			// Same observable as the same key routed without the picker open.
			twin := themeOverlayModel(t, themeConfigContent)
			twinCmd := twin.HandleKeyInput(zeroterm.KeyPressMsg{Key: tt.key})

			require.NotNil(t, twinCmd)
			assert.IsType(t, twinCmd(), msg, "the picker must not change the quitting behavior")
			assert.Equal(t, twin.quitting, mdl.quitting)
		})
	}
}

//nolint:funlen // large modal capture case table
func TestThemePickerModalCapture(t *testing.T) {
	t.Parallel()

	click := zeroterm.MouseClickMsg{X: 5, Y: 5, Button: zeroterm.MouseLeft}

	tests := []struct {
		name  string
		msg   zeroterm.Msg
		check func(t *testing.T, mdl *model, w *workflow.Workflow)
	}{
		{
			name: "m does not toggle fullscreen",
			msg:  zeroterm.KeyPressMsg{Key: "m"},
			check: func(t *testing.T, mdl *model, _ *workflow.Workflow) {
				t.Helper()

				assert.False(t, mdl.viewports.IsFullscreen())
			},
		},
		{
			name: "h does not toggle build logs",
			msg:  zeroterm.KeyPressMsg{Key: "h"},
			check: func(t *testing.T, mdl *model, _ *workflow.Workflow) {
				t.Helper()

				assert.False(t, mdl.conf.Flags.Tui.ShowAllBuildLogs)
			},
		},
		{
			name: "a does not toggle active only",
			msg:  zeroterm.KeyPressMsg{Key: "a"},
			check: func(t *testing.T, mdl *model, _ *workflow.Workflow) {
				t.Helper()

				assert.False(t, mdl.conf.Flags.Tui.ShowActiveOnly)
			},
		},
		{
			name: "c does not toggle commands",
			msg:  zeroterm.KeyPressMsg{Key: "c"},
			check: func(t *testing.T, mdl *model, _ *workflow.Workflow) {
				t.Helper()

				assert.False(t, mdl.conf.Flags.Tui.ShowCommandsInLabels)
			},
		},
		{
			name: "ctrl+r does not restart the workflow",
			msg:  zeroterm.KeyPressMsg{Key: "ctrl+r"},
			check: func(t *testing.T, mdl *model, w *workflow.Workflow) {
				t.Helper()

				assert.Same(t, w, mdl.workflow)
			},
		},
		{
			name: "mouse click is swallowed",
			msg:  click,
			check: func(t *testing.T, mdl *model, _ *workflow.Workflow) {
				t.Helper()

				assert.Equal(t, compNone, mdl.active)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mdl := themeOverlayModel(t, themeConfigContent)
			mustOpenThemePicker(t, mdl)

			mdl.Update(tt.msg)

			require.True(t, mdl.themePicker.open, "captured input must keep the picker open")
			tt.check(t, mdl, mdl.workflow)
		})
	}

	t.Run("direct mouse handling is captured too", func(t *testing.T) {
		t.Parallel()

		mdl := themeOverlayModel(t, themeConfigContent)
		mustOpenThemePicker(t, mdl)

		mdl.handleMouseClick(click)

		require.True(t, mdl.themePicker.open)
		assert.Equal(t, compNone, mdl.active)
	})
}

func TestThemePickerScrollsToSelection(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)
	renderModelFrame(mdl) // populate the header/footer heights

	// Shrink the terminal so only one theme row fits in the picker.
	mdl.dimensions.Height = mdl.header.Len() + mdl.footer.Len() + themePickerChromeRows + 1

	mustOpenThemePicker(t, mdl)

	mdl.Update(zeroterm.KeyPressMsg{Key: "down"})
	mdl.Update(zeroterm.KeyPressMsg{Key: "down"})

	require.Equal(t, themeNameLast, mdl.conf.Theme.Name)
	assert.Equal(t, 2, mdl.themePicker.offset, "the window must scroll with the selection")

	box := mdl.themePickerBox()
	require.NotNil(t, box)

	var boxText strings.Builder
	for idx := range box.Len() {
		boxText.Write(box.Line(idx))
	}

	assert.Contains(t, boxText.String(), themeNameLast, "the selected row must be visible")
	assert.NotContains(t, boxText.String(), themeNameFirst, "scrolled-away rows must be clipped")
}

// TestThemePickerRenderDoesNotScroll pins that rendering the picker does not
// mutate scroll state; selection changes and window resizes own that adjustment.
func TestThemePickerRenderDoesNotScroll(t *testing.T) {
	t.Parallel()

	mdl := themeOverlayModel(t, themeConfigContent)
	renderModelFrame(mdl) // populate the header/footer heights

	mdl.dimensions.Height = mdl.header.Len() + mdl.footer.Len() + themePickerChromeRows + 1

	mustOpenThemePicker(t, mdl)

	// Force the selected row outside the visible window without moving the offset.
	mdl.themePicker.index = 2
	mdl.themePicker.offset = 0

	box := mdl.themePickerBox()
	require.NotNil(t, box)
	assert.Equal(t, 0, mdl.themePicker.offset, "render must not adjust scroll state")
}
