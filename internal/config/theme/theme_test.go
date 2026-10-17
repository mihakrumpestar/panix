package theme

import (
	"reflect"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearDerived returns a copy of v with every yaml:"-" (derived) field zeroed, so
// the serializable projection can be compared to a struct literal.
func clearDerived[T any](v T) T {
	clearDerivedValue(reflect.ValueOf(&v).Elem())

	return v
}

func clearDerivedValue(value reflect.Value) {
	switch value.Kind() {
	case reflect.Struct:
		typ := value.Type()
		for index := range value.NumField() {
			if typ.Field(index).Tag.Get("yaml") == "-" {
				value.Field(index).SetZero()

				continue
			}

			if value.Field(index).CanSet() {
				clearDerivedValue(value.Field(index))
			}
		}
	case reflect.Pointer:
		if !value.IsNil() {
			clearDerivedValue(value.Elem())
		}
	default:
	}
}

// TestDefaultThemeMatchesLegacyGolden pins the serializable Theme deriving, through
// every method, the exact style, border, glyph and gradient values of the legacy
// hard-coded DefaultTheme golden.
func TestDefaultThemeMatchesLegacyGolden(t *testing.T) {
	t.Parallel()

	expected := legacyResolvedTheme()

	assert.Equal(t, expected, resolveTheme(DefaultTheme()), "DefaultTheme must derive the legacy golden")

	theme := defaultTheme()

	assert.Equal(t, expected, resolveTheme(&theme), "defaultTheme must derive the legacy golden")
}

// resolveTheme drives every derivation method of the single Theme into a flat
// path-keyed map, so the golden can pin each derived value byte-for-byte.
//
//nolint:funlen // one entry per derived value
func resolveTheme(theme *Theme) map[string]any {
	return map[string]any{
		"Header.Title": theme.Header.Title.Style,

		"Status.OK":            theme.Status.OK.Style,
		"Status.Warning":       theme.Status.Warning.Style,
		"Status.Failed":        theme.Status.Failed.Style,
		"Status.Running":       theme.Status.Running.Style,
		"Status.Icons.OK":      theme.Status.Icons.OKBytes,
		"Status.Icons.Failed":  theme.Status.Icons.FailedBytes,
		"Status.Icons.Running": theme.Status.Icons.RunningBytes,

		"Table.Border":                       theme.Table.Border.Style,
		"Table.Row":                          theme.Table.Row.Style,
		"Table.SelectionHighlightBackground": theme.Table.SelectionHighlightBackground.Style,
		"Table.SelectionHighlightBorder":     theme.Table.SelectionHighlightBorder.Style,

		"PhaseStatus.Pill": theme.PhaseStatus.Pill.Style,

		"Chars.HeaderSeparator": theme.Chars.HeaderSeparatorBytes,
		"Chars.SnapshotIcon":    theme.Chars.SnapshotIconBytes,
		"Chars.ErrorIcon":       theme.Chars.ErrorIconBytes,
		"Chars.RowSpanMarker":   theme.Chars.RowSpanMarkerBytes,
		"Chars.HeaderTitleSep":  theme.Chars.HeaderTitleSepBytes,
		"Chars.PhaseArrow":      theme.Chars.PhaseArrowBytes,

		"Border.Table":        theme.Border.TableGlyphs,
		"Border.Overlay":      theme.Border.OverlayGlyphs,
		"Border.Viewport":     theme.Border.ViewportGlyphs,
		"Border.Notification": theme.Border.NotificationGlyphs,

		"Tree.Enumerator": theme.Tree.Enumerator.Style,
		"Tree.Branch":     theme.Tree.BranchBytes,
		"Tree.BranchLast": theme.Tree.BranchLastBytes,
		"Tree.Vertical":   theme.Tree.VerticalBytes,
		"Tree.Horizontal": theme.Tree.HorizontalBytes,

		"Footer.HelpKey":               theme.Footer.HelpKey.Style,
		"Footer.HelpDesc":              theme.Footer.HelpDesc.Style,
		"Footer.HelpSeparator":         theme.Footer.HelpSeparator.Style,
		"Footer.HelpSelectedKey":       theme.Footer.HelpSelectedKey.Style,
		"Footer.HelpSelectedDesc":      theme.Footer.HelpSelectedDesc.Style,
		"Footer.NotificationBaseStyle": theme.Footer.NotificationBaseStyle.Style,

		"Notification.DefaultFgColor": theme.Notification.DefaultFgColor,

		"Flake.Style":       theme.Flake.Style,
		"Flake.Icon":        theme.Flake.IconBytes,
		"Installable.Style": theme.Installable.Style,
		"Installable.Icon":  theme.Installable.IconBytes,
		"OutputType.Style":  theme.OutputType.Style,
		"OutputType.Icon":   theme.OutputType.IconBytes,
		"Machine.Style":     theme.Machine.Style,
		"Machine.Icon":      theme.Machine.IconBytes,
		"Phase.Style":       theme.Phase.Style,
		"Phase.Icon":        theme.Phase.IconBytes,
		"Command.Style":     theme.Command.Style,
		"Command.Icon":      theme.Command.IconBytes,
		"Error.Style":       theme.Error.Style,
		"Error.Icon":        theme.Error.IconBytes,

		"Spinner.Frames":   theme.Spinner.FrameBytes,
		"Spinner.Interval": theme.Spinner.Interval.Duration(),
	}
}

func TestThemeStyleDerive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		theme    ThemeStyle
		expected style.Style
	}{
		{
			name:     "empty style",
			theme:    ThemeStyle{},
			expected: style.NewStyle(),
		},
		{
			name:     "foreground only",
			theme:    ThemeStyle{Fg: "#F8F8F2"},
			expected: style.NewStyle().Foreground(style.Color("#F8F8F2")),
		},
		{
			name:     "background only",
			theme:    ThemeStyle{Bg: "#3B3258"},
			expected: style.NewStyle().Background(style.Color("#3B3258")),
		},
		{
			name:     "bold true",
			theme:    ThemeStyle{Fg: "#00ADD8", Bold: new(true)},
			expected: style.NewStyle().Foreground(style.Color("#00ADD8")).Bold(true),
		},
		{
			name:     "explicit bold false renders not bold",
			theme:    ThemeStyle{Fg: "#00ADD8", Bold: new(false)},
			expected: style.NewStyle().Foreground(style.Color("#00ADD8")),
		},
		{
			name:     "foreground and background",
			theme:    ThemeStyle{Fg: "#FFFFFF", Bg: "#BD93F9"},
			expected: style.NewStyle().Foreground(style.Color("#FFFFFF")).Background(style.Color("#BD93F9")),
		},
		{
			name:     "padding",
			theme:    ThemeStyle{Fg: "#FFFFFF", Bold: new(true), PadY: new(0), PadX: new(1)},
			expected: style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1),
		},
		{
			name:     "invalid color renders uncolored",
			theme:    ThemeStyle{Fg: "#zzzzzz", Bg: "999"},
			expected: style.NewStyle(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.theme.derive().Style)
		})
	}
}

// TestThemeStyleDerivePaddingOneSided pins that a style with only one padding side
// set treats the other side as zero.
func TestThemeStyleDerivePaddingOneSided(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.NewStyle().Padding(0, 2), (ThemeStyle{PadX: new(2)}).derive().Style)
}

// TestBuildBorderFallbacks pins the derivation fallback order: a known region
// preset wins, an unknown name falls back to the region default, and an unknown
// name plus fallback degrade to sharp.
func TestBuildBorderFallbacks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.RoundedBorder(), buildBorder("rounded", "sharp"), "a known region name wins")
	assert.Equal(t, style.NormalBorder(), buildBorder("no-such-preset", "sharp"), "an unknown name falls back to the region default")
	assert.Equal(t, style.NormalBorder(), buildBorder("no-such-preset", "also-missing"), "an unknown name and fallback degrade to the sharp preset")
}

func TestBuildFramesAreFreshBytes(t *testing.T) {
	t.Parallel()

	frames := []string{"⣾", "⣽"}

	built := buildFrames(frames)

	require.Len(t, built, 2)
	assert.Equal(t, []byte("⣾"), built[0])

	built[0][0] = 'x'

	assert.Equal(t, "⣾", frames[0], "built frames must not alias the theme strings")
}

func TestThemeYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	theme := defaultTheme()

	encoded, err := yaml.Marshal(theme)
	require.NoError(t, err)

	var decoded Theme

	require.NoError(t, yamlx.Decode(encoded, &decoded))
	decoded = decoded.derive()
	assert.Equal(t, theme, decoded, "defaultTheme must survive a YAML round-trip unchanged")
}

// legacyResolvedTheme is the golden copy of the historical hard-coded DefaultTheme,
// projected through the same path keys as resolveTheme: derivation is
// behavior-neutral when the two maps are equal.
//
//nolint:funlen // one entry per derived value
func legacyResolvedTheme() map[string]any {
	borderStyle := legacyForegroundStyle("#6272A4", false)

	return map[string]any{
		"Header.Title": legacyForegroundStyle("#00ADD8", true),

		"Status.OK":            legacyForegroundStyle("#50FA7B", false),
		"Status.Warning":       legacyForegroundStyle("#FFB86C", false),
		"Status.Failed":        legacyForegroundStyle("#FF5555", false),
		"Status.Running":       legacyForegroundStyle("#00BFFF", false),
		"Status.Icons.OK":      legacyRuneBytes('✅'),
		"Status.Icons.Failed":  legacyRuneBytes('🔴'),
		"Status.Icons.Running": legacyRuneBytes('🔄'),

		"Table.Border":                       borderStyle,
		"Table.Row":                          legacyForegroundStyle("#F8F8F2", false),
		"Table.SelectionHighlightBackground": legacyBackgroundStyle("#3B3258"),
		"Table.SelectionHighlightBorder":     legacyBackgroundStyle("#BD93F9"),

		"PhaseStatus.Pill": style.NewStyle().Foreground(style.Color("#FFFFFF")).Bold(true).Padding(0, 1),

		"Chars.HeaderSeparator": []byte("│"),
		"Chars.SnapshotIcon":    legacyRuneBytes('◉'),
		"Chars.ErrorIcon":       legacyRuneBytes('✗'),
		"Chars.RowSpanMarker":   legacyRuneBytes('󱞩'),
		"Chars.HeaderTitleSep":  []byte(":"),
		"Chars.PhaseArrow":      legacyRuneBytes('󰜴'),

		"Border.Table":        style.NormalBorder(),
		"Border.Overlay":      style.NormalBorder(),
		"Border.Viewport":     style.RoundedBorder(),
		"Border.Notification": style.RoundedBorder(),

		"Tree.Enumerator": borderStyle,
		"Tree.Branch":     []byte("├"),
		"Tree.BranchLast": []byte("╰"),
		"Tree.Vertical":   []byte("│"),
		"Tree.Horizontal": []byte("─"),

		"Footer.HelpKey":               style.NewStyle().Foreground(style.Color("#FFFFFF")),
		"Footer.HelpDesc":              style.NewStyle().Foreground(style.Color("#8e8e8e")),
		"Footer.HelpSeparator":         style.NewStyle().Foreground(style.Color("#6272A4")),
		"Footer.HelpSelectedKey":       style.NewStyle().Foreground(style.Color("#8BE9FD")).Bold(true),
		"Footer.HelpSelectedDesc":      style.NewStyle().Foreground(style.Color("#568CAF")),
		"Footer.NotificationBaseStyle": style.NewStyle().Bold(true),

		"Notification.DefaultFgColor": style.Color("#B4B4B4"),

		"Flake.Style":       legacyForegroundStyle("#F1FA8C", true),
		"Flake.Icon":        legacyRuneBytes('📁'),
		"Installable.Style": legacyForegroundStyle("#FFB86C", false),
		"Installable.Icon":  legacyRuneBytes('📦'),
		"OutputType.Style":  legacyForegroundStyle("#F8D97C", false),
		"OutputType.Icon":   legacyRuneBytes('🧬'),
		"Machine.Style":     legacyForegroundStyle("#8BE9FD", false),
		"Machine.Icon":      legacyRuneBytes('💻'),
		"Phase.Style":       legacyForegroundStyle("#FF79C6", false),
		"Phase.Icon":        legacyRuneBytes('📋'),
		"Command.Style":     legacyForegroundStyle("#BD93F9", false),
		"Command.Icon":      legacyRuneBytes('⚙'),
		"Error.Style":       legacyForegroundStyle("#FF5555", false),
		"Error.Icon":        legacyRuneBytes('✗'),

		"Spinner.Frames":   [][]byte{[]byte("⣾"), []byte("⣽"), []byte("⣻"), []byte("⢿"), []byte("⡿"), []byte("⣟"), []byte("⣯"), []byte("⣷")},
		"Spinner.Interval": time.Second / 10,
	}
}

func legacyForegroundStyle(color string, bold bool) style.Style {
	s := style.NewStyle().Foreground(style.Color(color))
	if bold {
		s = s.Bold(true)
	}

	return s
}

func legacyBackgroundStyle(color string) style.Style {
	return style.NewStyle().Background(style.Color(color))
}

func legacyRuneBytes(r rune) []byte {
	return []byte(string(r))
}
