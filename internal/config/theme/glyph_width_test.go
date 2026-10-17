package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// TestBuiltinThemeGlyphs_StaticWidths guards the shipped glyph data against
// fallback table regressions: every glyph every built-in theme ships must measure
// one or two cells.
func TestBuiltinThemeGlyphs_StaticWidths(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	for _, name := range registry.Names() {
		theme, ok := registry.Build(name)
		require.True(t, ok, "theme %q must build", name)

		for field, glyph := range GlyphsOf(theme) {
			width := style.CellWidth([]byte(glyph))
			assert.True(t, width == 1 || width == 2,
				"theme %q glyph %s %q must be one or two cells, got %d", name, field, glyph, width)
		}
	}
}

// TestBuiltinThemeGlyphWidthAssumptions pins the widths the fallback table assumes
// for shipped glyphs whose rendered width depends on the font: text presentation
// pictographs are one cell, emoji some fonts do not cover are still two. The live
// terminal probe (pkg/tui/zeroterm/widthprobe.go) corrects these at runtime, so
// these pins document the fallback only.
func TestBuiltinThemeGlyphWidthAssumptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		glyph string
		want  int
	}{
		{"machine icon, text presentation", "🖥", 1},
		{"file cabinet, text presentation", "🗄", 1},
		{"control knobs, text presentation", "🎛", 1},
		{"phase icon, newer emoji", "🪜", 2},
		{"output type icon, newer emoji", "🪵", 2},
		{"books", "📚", 2},
		{"no entry", "⛔", 2},
		{"box drawing", "─", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, style.CellWidth([]byte(tc.glyph)), "CellWidth(%q)", tc.glyph)
		})
	}
}
