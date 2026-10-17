package theme

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// TestGlyphsOf_NilTheme pins that a nil theme enumerates nothing instead of
// panicking.
func TestGlyphsOf_NilTheme(t *testing.T) {
	t.Parallel()

	assert.Empty(t, GlyphsOf(nil))
}

// TestGlyphsOf_CoversEveryGlyphLeaf checks the enumerator against direct field
// access: every glyph a theme ships, borders included, must appear under the field
// path that holds it. The exact count catches a missed or leaked leaf.
func TestGlyphsOf_CoversEveryGlyphLeaf(t *testing.T) {
	t.Parallel()

	theme := DefaultTheme()
	glyphs := GlyphsOf(theme)

	require.NotEmpty(t, glyphs)

	direct := map[string]string{
		"Status.Icons.OK":       theme.Status.Icons.OK,
		"Status.Icons.Failed":   theme.Status.Icons.Failed,
		"Status.Icons.Running":  theme.Status.Icons.Running,
		"Chars.HeaderSeparator": theme.Chars.HeaderSeparator,
		"Chars.SnapshotIcon":    theme.Chars.SnapshotIcon,
		"Chars.ErrorIcon":       theme.Chars.ErrorIcon,
		"Chars.RowSpanMarker":   theme.Chars.RowSpanMarker,
		"Chars.HeaderTitleSep":  theme.Chars.HeaderTitleSep,
		"Chars.PhaseArrow":      theme.Chars.PhaseArrow,
		"Flake.Icon":            theme.Flake.Icon,
		"Installable.Icon":      theme.Installable.Icon,
		"OutputType.Icon":       theme.OutputType.Icon,
		"Machine.Icon":          theme.Machine.Icon,
		"Phase.Icon":            theme.Phase.Icon,
		"Command.Icon":          theme.Command.Icon,
		"Error.Icon":            theme.Error.Icon,
		"Tree.Branch":           theme.Tree.Branch,
		"Tree.BranchLast":       theme.Tree.BranchLast,
		"Tree.Vertical":         theme.Tree.Vertical,
		"Tree.Horizontal":       theme.Tree.Horizontal,
	}

	for field, glyph := range direct {
		assert.Equal(t, glyph, glyphs[field], "glyph leaf %s", field)
	}

	for i, frame := range theme.Spinner.Frames {
		assert.Equal(t, frame, glyphs[fmt.Sprintf("Spinner.Frames[%d]", i)])
	}

	borders := map[string]style.Border{
		"Border.Table":        theme.Border.TableGlyphs,
		"Border.Overlay":      theme.Border.OverlayGlyphs,
		"Border.Viewport":     theme.Border.ViewportGlyphs,
		"Border.Notification": theme.Border.NotificationGlyphs,
	}

	for prefix, border := range borders {
		for suffix, glyph := range borderGlyphFields(border) {
			assert.Equal(t, glyph, glyphs[prefix+"."+suffix], "glyph leaf %s.%s", prefix, suffix)
		}
	}

	expectedCount := len(direct) + len(theme.Spinner.Frames) + len(borders)*len(borderGlyphFields(style.Border{}))
	assert.Len(t, glyphs, expectedCount)
}

// borderGlyphFields lists the exported glyph fields of a resolved border.
func borderGlyphFields(border style.Border) map[string]string {
	return map[string]string{
		"TopLeft":     string(border.TopLeft),
		"TopRight":    string(border.TopRight),
		"BottomLeft":  string(border.BottomLeft),
		"BottomRight": string(border.BottomRight),
		"Horizontal":  string(border.Horizontal),
		"Vertical":    string(border.Vertical),
		"TopMid":      string(border.TopMid),
		"BottomMid":   string(border.BottomMid),
		"LeftMid":     string(border.LeftMid),
		"RightMid":    string(border.RightMid),
		"MidMid":      string(border.MidMid),
	}
}
