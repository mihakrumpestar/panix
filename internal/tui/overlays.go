package tui

import (
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/overlay"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

// Modal overlay layout constants. No color literals: every color comes from the
// active theme.
const (
	// overlayEdgeMargin is the minimum horizontal margin around an overlay box.
	overlayEdgeMargin = 2
)

// modalKind names the modal overlay that currently owns the keyboard. It is the
// single precedence source for input capture, overlay rendering and the footer
// keymap.
type modalKind int

const (
	modalNone modalKind = iota
	modalThemePicker
)

func (m *model) modal() modalKind {
	if m.themePicker.open {
		return modalThemePicker
	}

	return modalNone
}

// captureModalInput routes input to the open modal overlay and reports whether
// it was consumed plus the cmd to run (picker quitting keys return the global
// quit cmd). Modal capture keeps global keybinds, mouse clicks and viewport
// scrolling from firing underneath an overlay; non-input messages always flow
// through unchanged.
func (m *model) captureModalInput(msg zeroterm.Msg) (bool, zeroterm.Cmd) {
	if m.modal() != modalThemePicker {
		return false, nil
	}

	switch msg := msg.(type) {
	case zeroterm.KeyPressMsg:
		return true, m.handleThemePickerKey(msg.String())
	case zeroterm.MouseClickMsg:
		// Clicks are swallowed while the picker is open.
		return true, nil
	case zeroterm.MouseWheelMsg:
		m.moveThemePickerSelection(wheelStep(msg.Button))

		return true, nil
	}

	return false, nil
}

func wheelStep(button zeroterm.MouseButton) int {
	if button == zeroterm.MouseWheelUp {
		return -1
	}

	return 1
}

// renderOverlays composes the modal overlay over the rendered frame. With no
// overlay open it renders nothing, so the frame stays byte-identical to the
// plain render. Only one overlay renders at a time, matching input capture.
func (m *model) renderOverlays(buf *buffer.LinesBufDiff) {
	if m.modal() == modalThemePicker {
		m.composeOverlay(buf, m.themePickerBox())
	}
}

// composeOverlay dims the content area under box and centers box over it, so the
// header and footer rows stay visible and the background content reads as the
// unfocused layer instead of blanking out beside the box.
func (m *model) composeOverlay(buf *buffer.LinesBufDiff, box *overlay.Box) {
	if box == nil {
		return
	}

	region := overlay.Rect{
		Row:    m.header.Len(),
		Height: buf.Len() - m.header.Len() - m.footer.Len(),
		Width:  m.dimensions.Width,
	}

	dimRegion(buf, region)

	overlay.ComposeRegion(buf, box, region)
}

// dimRegion dims the visible content of region with the SGR faint attribute so
// the modal box reads as the focused layer above it. Empty rows stay untouched
// and content keeps its colors: the dim is an attribute, never a color literal.
func dimRegion(buf *buffer.LinesBufDiff, region overlay.Rect) {
	dim := style.NewStyle().Faint(true)

	end := min(region.Row+region.Height, buf.Len())
	for row := max(region.Row, 0); row < end; row++ {
		if !style.HasVisibleContent(buf.Line(row)) {
			continue
		}

		buf.SetLine(row, dim.RenderLine(buf.Line(row)))
	}
}

// overlayBoxStyle returns the shared modal box style: the theme's overlay border
// preset in the theme's border color with horizontal padding.
func overlayBoxStyle(cs *theme.Theme) style.Style {
	return style.NewStyle().
		Border(cs.Border.OverlayGlyphs).
		BorderForeground(cs.Table.Border.Style.GetForeground()).
		Padding(0, 1)
}

// overlayMaxWidth caps the total overlay box width, keeping side margins.
func (m *model) overlayMaxWidth() int {
	return max(m.dimensions.Width-2*overlayEdgeMargin, 0)
}

// styledCell renders text through st padded to width cells (no padding when
// width <= 0).
func styledCell(st style.Style, width int, text string) []byte {
	if width > 0 {
		st = st.Width(width)
	}

	return st.RenderLine([]byte(text))
}
