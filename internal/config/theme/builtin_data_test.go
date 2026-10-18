package theme

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opaqueHexRE pins the encoder-ready color format of the embedded themes: an
// opaque "#rrggbb" (any case, no alpha, no shorthand).
var opaqueHexRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// themeColorFields returns every color-bearing field of a theme keyed by YAML
// path, so the opaque-hex and value-pin tests share one enumeration.
func themeColorFields(theme Theme) map[string]string {
	return map[string]string{
		"header.title.fg":                         string(theme.Header.Title.Fg),
		"status.ok.fg":                            string(theme.Status.OK.Fg),
		"status.warning.fg":                       string(theme.Status.Warning.Fg),
		"status.failed.fg":                        string(theme.Status.Failed.Fg),
		"status.running.fg":                       string(theme.Status.Running.Fg),
		"table.border.fg":                         string(theme.Table.Border.Fg),
		"table.row.fg":                            string(theme.Table.Row.Fg),
		"table.selection_highlight_background.bg": string(theme.Table.SelectionHighlightBackground.Bg),
		"table.selection_highlight_border.bg":     string(theme.Table.SelectionHighlightBorder.Bg),
		"phase_status.running[0]":                 string(theme.PhaseStatus.Running[0]),
		"phase_status.running[1]":                 string(theme.PhaseStatus.Running[1]),
		"phase_status.failed[0]":                  string(theme.PhaseStatus.Failed[0]),
		"phase_status.failed[1]":                  string(theme.PhaseStatus.Failed[1]),
		"phase_status.done[0]":                    string(theme.PhaseStatus.Done[0]),
		"phase_status.done[1]":                    string(theme.PhaseStatus.Done[1]),
		"phase_status.default[0]":                 string(theme.PhaseStatus.Default[0]),
		"phase_status.default[1]":                 string(theme.PhaseStatus.Default[1]),
		"phase_status.pill.fg":                    string(theme.PhaseStatus.Pill.Fg),
		"tree.enumerator.fg":                      string(theme.Tree.Enumerator.Fg),
		"footer.help_key.fg":                      string(theme.Footer.HelpKey.Fg),
		"footer.help_desc.fg":                     string(theme.Footer.HelpDesc.Fg),
		"footer.help_separator.fg":                string(theme.Footer.HelpSeparator.Fg),
		"footer.help_selected_key.fg":             string(theme.Footer.HelpSelectedKey.Fg),
		"footer.help_selected_desc.fg":            string(theme.Footer.HelpSelectedDesc.Fg),
		"notification.default_fg_color":           string(theme.Notification.DefaultFgColor),
		"flake.color":                             string(theme.Flake.Color),
		"installable.color":                       string(theme.Installable.Color),
		"output_type.color":                       string(theme.OutputType.Color),
		"machine.color":                           string(theme.Machine.Color),
		"phase.color":                             string(theme.Phase.Color),
		"command.color":                           string(theme.Command.Color),
		"error.color":                             string(theme.Error.Color),
	}
}

// TestBuiltinThemesColorsAreOpaqueHex asserts every color field of every embedded
// theme is an opaque "#rrggbb": no alpha, no shorthand, no palette index.
func TestBuiltinThemesColorsAreOpaqueHex(t *testing.T) {
	t.Parallel()

	for name, theme := range builtinThemes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for field, value := range themeColorFields(theme) {
				assert.Regexp(t, opaqueHexRE, value, "%s %s", name, field)
			}
		})
	}
}

// TestBuiltinThemeValuePins pins explicit values for representative embedded
// themes: canonical hexes plus a few glyph/spinner/border/mode values, so a data
// regression fails here.
//
//nolint:funlen // large per-theme pin table
func TestBuiltinThemeValuePins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		theme  string
		colors map[string]string
		extra  func(t *testing.T, theme Theme)
	}{
		{
			name:  "catppuccin-mocha",
			theme: "catppuccin-mocha",
			colors: map[string]string{
				"header.title.fg":  "#cba6f7",
				"status.ok.fg":     "#a6e3a1",
				"status.failed.fg": "#f38ba8",
				"table.row.fg":     "#cdd6f4",
				"table.selection_highlight_background.bg": "#45475a",
				"phase_status.running[0]":                 "#cba6f7",
				"phase_status.running[1]":                 "#94e2d5",
				"error.color":                             "#f38ba8",
			},
			extra: func(t *testing.T, theme Theme) {
				t.Helper()

				assert.Equal(t, "dark", theme.Mode)
				assert.Equal(t, "✅", theme.Status.Icons.OK)
				assert.Equal(t, "➜", theme.Chars.PhaseArrow)
				assert.Equal(t, "rounded", theme.Border.Table)
				assert.Equal(t, "♡", theme.Spinner.Frames[0])
				assert.Equal(t, yamlx.Duration(150*time.Millisecond), theme.Spinner.Interval)
			},
		},
		{
			name:  "gruvbox-dark",
			theme: "gruvbox-dark",
			colors: map[string]string{
				"header.title.fg":  "#d3869b",
				"status.ok.fg":     "#b8bb26",
				"status.failed.fg": "#fb4934",
				"table.row.fg":     "#ebdbb2",
				"table.selection_highlight_background.bg": "#665c54",
				"phase_status.failed[1]":                  "#d3869b",
				"notification.default_fg_color":           "#a89984",
				"error.color":                             "#fb4934",
			},
			extra: func(t *testing.T, theme Theme) {
				t.Helper()

				assert.Equal(t, "dark", theme.Mode)
				assert.Equal(t, "✘", theme.Status.Icons.Failed)
				assert.Equal(t, "¦", theme.Chars.HeaderSeparator)
				assert.Equal(t, "heavy", theme.Border.Table)
				assert.Equal(t, new(0), theme.PhaseStatus.Pill.PadX)
				assert.Equal(t, "♪", theme.Spinner.Frames[0])
				assert.Equal(t, yamlx.Duration(150*time.Millisecond), theme.Spinner.Interval)
			},
		},
		{
			name:  "tokyonight-night",
			theme: "tokyonight-night",
			colors: map[string]string{
				"header.title.fg":        "#bb9af7",
				"status.running.fg":      "#7dcfff",
				"phase_status.failed[1]": "#ff007c",
				"error.color":            "#f7768e",
			},
			extra: func(t *testing.T, theme Theme) {
				t.Helper()

				assert.Equal(t, "dark", theme.Mode)
				assert.Equal(t, "✔", theme.Status.Icons.OK)
				assert.Equal(t, "▸", theme.Chars.PhaseArrow)
				assert.Equal(t, "double", theme.Border.Overlay)
				assert.Equal(t, "▂", theme.Spinner.Frames[0])
				assert.Equal(t, yamlx.Duration(100*time.Millisecond), theme.Spinner.Interval)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			theme, ok := builtinTheme(tt.theme)
			require.True(t, ok, "embedded theme %q must exist", tt.theme)

			fields := themeColorFields(theme)

			for field, want := range tt.colors {
				assert.Equal(t, want, fields[field], "%s %s", tt.theme, field)
			}

			tt.extra(t, theme)
		})
	}
}

// TestBuiltinThemeGradientPins pins the phase-status gradient pairs of
// representative embedded themes.
func TestBuiltinThemeGradientPins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		theme   string
		running ThemePair
		failed  ThemePair
		done    ThemePair
		def     ThemePair
	}{
		{
			theme:   "catppuccin-mocha",
			running: ThemePair{"#cba6f7", "#94e2d5"},
			failed:  ThemePair{"#f38ba8", "#f5c2e7"},
			done:    ThemePair{"#a6e3a1", "#94e2d5"},
			def:     ThemePair{"#a6adc8", "#9399b2"},
		},
		{
			theme:   "gruvbox-dark",
			running: ThemePair{"#d3869b", "#8ec07c"},
			failed:  ThemePair{"#fb4934", "#d3869b"},
			done:    ThemePair{"#b8bb26", "#8ec07c"},
			def:     ThemePair{"#bdae93", "#a89984"},
		},
		{
			theme:   "tokyonight-night",
			running: ThemePair{"#bb9af7", "#7dcfff"},
			failed:  ThemePair{"#f7768e", "#ff007c"},
			done:    ThemePair{"#9ece6a", "#7dcfff"},
			def:     ThemePair{"#a9b1d6", "#565f89"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.theme, func(t *testing.T) {
			t.Parallel()

			theme, ok := builtinTheme(tt.theme)
			require.True(t, ok, "embedded theme %q must exist", tt.theme)

			assert.Equal(t, tt.running, theme.PhaseStatus.Running)
			assert.Equal(t, tt.failed, theme.PhaseStatus.Failed)
			assert.Equal(t, tt.done, theme.PhaseStatus.Done)
			assert.Equal(t, tt.def, theme.PhaseStatus.Default)
		})
	}
}

// TestBuiltinThemeModes pins the terminal background mode of every embedded theme:
// exactly the nine light themes, dark for the rest.
func TestBuiltinThemeModes(t *testing.T) {
	t.Parallel()

	modes := BuiltinThemeModes()
	require.Len(t, modes, 34)

	light := map[string]bool{
		"catppuccin-latte": true,
		"gruvbox-light":    true,
		"tokyonight-day":   true,
		"kanagawa-lotus":   true,
		"everforest-light": true,
		"ayu-light":        true,
		"solarized-light":  true,
		"github-light":     true,
		"rose-pine-dawn":   true,
	}

	for name, mode := range modes {
		want := "dark"
		if light[name] {
			want = "light"
		}

		assert.Equal(t, want, mode, "theme %q mode", name)
	}
}

// assertThemeGlyphs asserts the curated glyph invariants: every border name
// resolves, structural glyphs are one cell wide and icon glyphs are one or two.
func assertThemeGlyphs(t *testing.T, theme Theme) {
	t.Helper()

	borders := []string{
		theme.Border.Table, theme.Border.Overlay, theme.Border.Viewport, theme.Border.Notification,
	}
	for _, preset := range borders {
		_, ok := style.BorderByName(preset)
		assert.True(t, ok, "border preset %q must resolve", preset)
	}

	structural := map[string]string{
		"header separator": theme.Chars.HeaderSeparator,
		"row span marker":  theme.Chars.RowSpanMarker,
		"header title sep": theme.Chars.HeaderTitleSep,
		"phase arrow":      theme.Chars.PhaseArrow,
	}
	for label, glyph := range structural {
		assert.Equal(t, 1, style.CellWidth([]byte(glyph)), "%s must be one cell wide", label)
	}

	icons := map[string]string{
		"snapshot icon":  theme.Chars.SnapshotIcon,
		"error icon":     theme.Chars.ErrorIcon,
		"status ok":      theme.Status.Icons.OK,
		"status failed":  theme.Status.Icons.Failed,
		"status running": theme.Status.Icons.Running,
		"entity flake":   theme.Flake.Icon,
		"entity install": theme.Installable.Icon,
		"entity output":  theme.OutputType.Icon,
		"entity machine": theme.Machine.Icon,
		"entity phase":   theme.Phase.Icon,
		"entity command": theme.Command.Icon,
		"entity error":   theme.Error.Icon,
	}
	for label, glyph := range icons {
		width := style.CellWidth([]byte(glyph))
		assert.GreaterOrEqual(t, width, 1, "%s must not be empty", label)
		assert.LessOrEqual(t, width, 2, "%s must be at most two cells wide", label)
	}
}

// assertThemeSpinner asserts spinner frames are non-empty, share one cell width
// and keep the interval inside the operational range.
func assertThemeSpinner(t *testing.T, theme Theme) {
	t.Helper()

	const (
		minInterval = 60 * time.Millisecond
		maxInterval = 200 * time.Millisecond
	)

	require.NotEmpty(t, theme.Spinner.Frames, "spinner frames must not be empty")

	frameWidth := style.CellWidth([]byte(theme.Spinner.Frames[0]))
	for _, frame := range theme.Spinner.Frames {
		assert.Equal(t, frameWidth, style.CellWidth([]byte(frame)), "spinner frames must share one cell width")
	}

	interval := theme.Spinner.Interval.Duration()
	assert.GreaterOrEqual(t, interval, minInterval, "spinner interval must be at least %s", minInterval)
	assert.LessOrEqual(t, interval, maxInterval, "spinner interval must be at most %s", maxInterval)
}

// TestBuiltinThemesAreStructural asserts the curated theme invariants for every
// embedded theme: border presets resolve, glyph widths match their role, spinner
// frames are non-empty and share one width, and intervals stay in range.
func TestBuiltinThemesAreStructural(t *testing.T) {
	t.Parallel()

	for name, theme := range builtinThemes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertThemeGlyphs(t, theme)
			assertThemeSpinner(t, theme)
		})
	}
}

// entityGlyph is the pointer-free signature of a log entity's glyph fields.
type entityGlyph struct {
	Icon string
	Bold bool
}

// signature is the pointer-free projection of a theme's visual surface (borders,
// glyphs, icons, bold flags, pill padding, spinner): the fields that make a theme
// visually distinct. Pointers are dereferenced for deterministic formatting.
type signature struct {
	Border                    ThemeBorder
	Chars                     ThemeChars
	Status                    ThemeStatusIcons
	Entities                  [7]entityGlyph
	PillPadX                  int
	PillBold                  bool
	HeaderTitleBold           bool
	FooterHelpSelectedKeyBold bool
	Spinner                   ThemeSpinner
}

func signatureOf(theme Theme) signature {
	return signature{
		Border: theme.Border,
		Chars:  theme.Chars,
		Status: theme.Status.Icons,
		Entities: [7]entityGlyph{
			{Icon: theme.Flake.Icon, Bold: derefBool(theme.Flake.Bold)},
			{Icon: theme.Installable.Icon, Bold: derefBool(theme.Installable.Bold)},
			{Icon: theme.OutputType.Icon, Bold: derefBool(theme.OutputType.Bold)},
			{Icon: theme.Machine.Icon, Bold: derefBool(theme.Machine.Bold)},
			{Icon: theme.Phase.Icon, Bold: derefBool(theme.Phase.Bold)},
			{Icon: theme.Command.Icon, Bold: derefBool(theme.Command.Bold)},
			{Icon: theme.Error.Icon, Bold: derefBool(theme.Error.Bold)},
		},
		PillPadX:                  derefInt(theme.PhaseStatus.Pill.PadX),
		PillBold:                  derefBool(theme.PhaseStatus.Pill.Bold),
		HeaderTitleBold:           derefBool(theme.Header.Title.Bold),
		FooterHelpSelectedKeyBold: derefBool(theme.Footer.HelpSelectedKey.Bold),
		Spinner:                   theme.Spinner,
	}
}

func derefBool(v *bool) bool {
	if v == nil {
		return false
	}

	return *v
}

// TestBuiltinThemeTuplesAreUnique asserts no two embedded themes share the full
// theme tuple (borders, glyphs, icons, bold flags, pill padding, spinner): every
// theme is visually distinct even where palettes or spinner frames repeat.
func TestBuiltinThemeTuplesAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[string]string{}

	for name, theme := range builtinThemes() {
		key := fmt.Sprintf("%#v", signatureOf(theme))

		owner, exists := seen[key]
		assert.False(t, exists, "theme tuple collides with %q", owner)

		seen[key] = name
	}

	assert.Len(t, seen, 34, "every built-in theme must own a distinct theme tuple")
}

// canonicalCycle returns the lexicographically smallest rotation of the joined
// spinner cycle. Two cycles are equal iff one is a rotation of the other.
func canonicalCycle(frames []string) string {
	best := ""

	for start := range frames {
		rotated := append(slices.Clone(frames[start:]), frames[:start]...)

		candidate := strings.Join(rotated, "\x00")
		if best == "" || candidate < best {
			best = candidate
		}
	}

	return best
}

// TestBuiltinThemeSpinnersAreUnique asserts every spinner cycle is pairwise
// distinct up to rotation across all 34 embedded themes (panix included).
func TestBuiltinThemeSpinnersAreUnique(t *testing.T) {
	t.Parallel()

	spinners := map[string][]string{}
	for name, theme := range builtinThemes() {
		spinners[name] = theme.Spinner.Frames
	}

	require.Len(t, spinners, 34, "the set must cover every theme")

	seen := map[string]string{}

	for _, name := range slices.Sorted(maps.Keys(spinners)) {
		canonical := canonicalCycle(spinners[name])

		other, collision := seen[canonical]
		assert.False(t, collision, "spinner cycle of %s must not be a rotation of %s", name, other)

		seen[canonical] = name
	}
}

// TestBuiltinThemesBuild asserts every embedded theme is wired into the registry,
// builds a *Theme and keeps builtin provenance.
func TestBuiltinThemesBuild(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	for _, name := range NewRegistry().Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, ok := reg.Build(name)

			require.True(t, ok, "theme %q must be registered", name)
			require.NotNil(t, scheme)
			assert.NotEmpty(t, scheme.Status.OK.Style.GetForeground(), "the built scheme must carry colors")
			assert.Equal(t, SourceBuiltin, reg.Source(name))
		})
	}
}
