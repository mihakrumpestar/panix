package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

// TestCollectProbeGlyphs_CoversRegistryThemes checks that the probe glyph set
// covers every glyph of every registry theme, including font gap glyphs, without
// duplicates.
func TestCollectProbeGlyphs_CoversRegistryThemes(t *testing.T) {
	t.Parallel()

	registry := theme.NewRegistry()
	conf := &config.Config{Theme: config.Theme{Registry: registry, Name: "panix"}}

	glyphs := collectProbeGlyphs(conf)
	require.NotEmpty(t, glyphs)

	present := make(map[string]struct{}, len(glyphs))

	for _, glyph := range glyphs {
		_, duplicate := present[glyph]
		assert.False(t, duplicate, "glyph %q must appear once", glyph)

		present[glyph] = struct{}{}
	}

	for _, name := range registry.Names() {
		scheme, ok := registry.Build(name)
		require.True(t, ok, "theme %q must build", name)

		for field, glyph := range theme.GlyphsOf(scheme) {
			assert.Contains(t, present, glyph, "theme %q glyph %s", name, field)
		}
	}

	assert.Contains(t, present, "🪜", "font gap glyph from the built-in themes")
	assert.Contains(t, present, "🪵", "font gap glyph from the built-in themes")
	assert.Contains(t, present, themePickerCursorGlyph, "picker cursor glyph")
}

// TestIsProbeGlyphRune pins the standalone-rune filter: combining marks and
// format runes only modify a preceding glyph, so they are never probe targets.
func TestIsProbeGlyphRune(t *testing.T) {
	t.Parallel()

	assert.True(t, isProbeGlyphRune('🪜'))
	assert.True(t, isProbeGlyphRune('\u6f22'))
	assert.False(t, isProbeGlyphRune('\u200d'), "ZWJ is a format rune")
	assert.False(t, isProbeGlyphRune('\ufe0f'), "VS16 is a combining mark")
	assert.False(t, isProbeGlyphRune('\u0301'), "combining acute is a combining mark")
}

// TestCollectProbeGlyphs_NoRegistry pins that a hand built config yields no probe
// glyphs instead of panicking.
func TestCollectProbeGlyphs_NoRegistry(t *testing.T) {
	t.Parallel()

	assert.Empty(t, collectProbeGlyphs(nil))
	assert.Empty(t, collectProbeGlyphs(&config.Config{}))
}

// TestInstallMeasuredGlyphWidths pins global installation and the nil-config guard.
//
//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths(t *testing.T) {
	t.Cleanup(style.ResetMeasuredGlyphWidths)

	assert.Equal(t, 2, style.CellWidth([]byte("\u6f22")), "static table baseline")

	debugConf := &config.Config{}
	debugConf.Flags.Logging.Debug = true

	installMeasuredGlyphWidths(debugConf, map[string]int{"\u6f22": 1}, 5*time.Millisecond)

	assert.Equal(t, 1, style.CellWidth([]byte("\u6f22")))

	installMeasuredGlyphWidths(nil, map[string]int{"\u6f22": 2}, 0)

	assert.Equal(t, 2, style.CellWidth([]byte("\u6f22")))
}
