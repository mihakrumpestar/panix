package tui

import (
	"slices"
	"time"
	"unicode"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/rs/zerolog/log"
)

// collectProbeGlyphs returns every glyph across all registry themes plus the
// picker cursor, adding each standalone rune of a multi-rune glyph too so emoji
// components still get measured. Combining marks and format runes (ZWJ, VS16,
// variation selectors) only modify a preceding glyph, so they are never probed
// alone. Deduplicated and sorted for deterministic probe order; nil without a
// registry.
func collectProbeGlyphs(conf *config.Config) []string {
	if conf == nil || conf.Theme.Registry == nil {
		return nil
	}

	seen := make(map[string]struct{})

	var glyphs []string

	add := func(glyph string) {
		if glyph == "" {
			return
		}

		if _, ok := seen[glyph]; ok {
			return
		}

		seen[glyph] = struct{}{}
		glyphs = append(glyphs, glyph)
	}

	add(themePickerCursorGlyph)

	for _, name := range conf.Theme.Registry.Names() {
		scheme, ok := conf.Theme.Registry.Build(name)
		if !ok {
			continue
		}

		for _, glyph := range theme.GlyphsOf(scheme) {
			add(glyph)

			for _, r := range glyph {
				if isProbeGlyphRune(r) {
					add(string(r))
				}
			}
		}
	}

	slices.Sort(glyphs)

	return glyphs
}

// isProbeGlyphRune reports whether r can stand alone as a probe target.
// Combining marks (Mn, Mc, Me) and format runes (Cf, e.g. ZWJ) only modify the
// glyph before them, so measuring them alone is meaningless.
func isProbeGlyphRune(r rune) bool {
	return !unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me, unicode.Cf)
}

// installMeasuredGlyphWidths installs the terminal-measured glyph widths and,
// when debug logging is on, records how many differ from the static table. The
// comparison must run BEFORE installing: CellWidth no longer reports static
// widths once installed.
func installMeasuredGlyphWidths(conf *config.Config, measured map[string]int, elapsed time.Duration) {
	differing := 0

	for glyph := range measured {
		if style.CellWidth([]byte(glyph)) != measured[glyph] {
			differing++
		}
	}

	style.InstallMeasuredGlyphWidths(measured)

	if conf != nil && conf.Flags.Logging.Debug {
		log.Debug().
			Int("glyphs_measured", len(measured)).
			Int("glyphs_differing", differing).
			Dur("elapsed", elapsed).
			Msg("terminal glyph width probe")
	}
}

// treeGlyphs maps the theme tree section onto the tree widget's glyph carrier.
// The tree composes prefixes from the four connectors and the indent step, so the
// connector width stays equal to the step.
func treeGlyphs(t theme.ThemeTree) tree.Glyphs {
	return tree.Glyphs{
		Branch:     t.BranchBytes,
		BranchLast: t.BranchLastBytes,
		Vertical:   t.VerticalBytes,
		Horizontal: t.HorizontalBytes,
	}
}

// applyTheme re-applies the theme to every widget. It is the single entry point
// for re-theming at runtime; each widget's ApplyTheme re-runs its constructor
// styling step and invalidates its render caches.
func (m *model) applyTheme() {
	activeTheme := m.theme
	tableS := activeTheme.Table

	m.header.ApplyTheme(activeTheme)
	m.footer.ApplyTheme(activeTheme)
	m.spinners.ApplyTheme(activeTheme.Spinner.FrameBytes, activeTheme.Spinner.Interval.Duration())
	m.viewports.ApplyTheme(tableS.Border.Style, tableS.SelectionHighlightBackground.Style,
		tableS.SelectionHighlightBorder.Style, activeTheme.Border.ViewportGlyphs)
	m.statsTable.ApplyTheme(activeTheme)
	m.phaseFlow.ApplyTheme(activeTheme)
	m.cachedTree.ApplyTheme(activeTheme.Tree.Enumerator.Style, treeGlyphs(activeTheme.Tree))

	// Build logs are created lazily on first render; keep their cached theme in
	// step when they already exist.
	if m.buildLogs != nil {
		m.buildLogs.ApplyTheme(activeTheme)
	}
}
