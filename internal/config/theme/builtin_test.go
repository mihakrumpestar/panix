package theme

import (
	"math"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosePineBaseDawn is the canonical rose-pine-dawn base hex
// (rose-pine/neovim palette.lua), used by the readability check.
const rosePineBaseDawn = "#faf4ed"

// TestBuiltinThemesRosePineRoleMapping pins the canonical Rosé Pine hexes on the
// right theme roles, read from the embedded themes: hexes must never be invented
// or altered.
//
//nolint:funlen // large role mapping table
func TestBuiltinThemesRosePineRoleMapping(t *testing.T) {
	t.Parallel()

	themes := builtinThemes()
	require.Contains(t, themes, "rose-pine")
	require.Contains(t, themes, "rose-pine-dawn")

	tests := []struct {
		name         string
		color        func(Theme) string
		rosePine     string
		rosePineDawn string
	}{
		{name: "header title is iris", color: func(th Theme) string { return string(th.Header.Title.Fg) }, rosePine: "#c4a7e7", rosePineDawn: "#907aa9"},
		{name: "status ok is pine", color: func(th Theme) string { return string(th.Status.OK.Fg) }, rosePine: "#31748f", rosePineDawn: "#286983"},
		{name: "status warning is gold", color: func(th Theme) string { return string(th.Status.Warning.Fg) }, rosePine: "#f6c177", rosePineDawn: "#ea9d34"}, //nolint:lll // large role mapping table
		{name: "status failed is love", color: func(th Theme) string { return string(th.Status.Failed.Fg) }, rosePine: "#eb6f92", rosePineDawn: "#b4637a"},
		{name: "status running is foam", color: func(th Theme) string { return string(th.Status.Running.Fg) }, rosePine: "#9ccfd8", rosePineDawn: "#56949f"}, //nolint:lll // large role mapping table
		{name: "table border is muted", color: func(th Theme) string { return string(th.Table.Border.Fg) }, rosePine: "#6e6a86", rosePineDawn: "#9893a5"},
		{name: "table row is text", color: func(th Theme) string { return string(th.Table.Row.Fg) }, rosePine: "#e0def4", rosePineDawn: "#464261"},
		{
			name:         "selection background is highlight_med",
			color:        func(th Theme) string { return string(th.Table.SelectionHighlightBackground.Bg) },
			rosePine:     "#403d52",
			rosePineDawn: "#dfdad9",
		},
		{
			name:         "selection border is iris",
			color:        func(th Theme) string { return string(th.Table.SelectionHighlightBorder.Bg) },
			rosePine:     "#c4a7e7",
			rosePineDawn: "#907aa9",
		},
		{name: "pill label is text", color: func(th Theme) string { return string(th.PhaseStatus.Pill.Fg) }, rosePine: "#e0def4", rosePineDawn: "#464261"},
		{name: "tree enumerator is muted", color: func(th Theme) string { return string(th.Tree.Enumerator.Fg) }, rosePine: "#6e6a86", rosePineDawn: "#9893a5"}, //nolint:lll // large role mapping table
		{name: "footer help key is text", color: func(th Theme) string { return string(th.Footer.HelpKey.Fg) }, rosePine: "#e0def4", rosePineDawn: "#464261"},   //nolint:lll // large role mapping table
		{
			name:         "footer help desc is muted",
			color:        func(th Theme) string { return string(th.Footer.HelpDesc.Fg) },
			rosePine:     "#6e6a86",
			rosePineDawn: "#9893a5",
		},
		{
			name:         "footer help separator is muted",
			color:        func(th Theme) string { return string(th.Footer.HelpSeparator.Fg) },
			rosePine:     "#6e6a86",
			rosePineDawn: "#9893a5",
		},
		{
			name:         "footer help selected key is iris",
			color:        func(th Theme) string { return string(th.Footer.HelpSelectedKey.Fg) },
			rosePine:     "#c4a7e7",
			rosePineDawn: "#907aa9",
		},
		{
			name:         "footer help selected desc is pine",
			color:        func(th Theme) string { return string(th.Footer.HelpSelectedDesc.Fg) },
			rosePine:     "#31748f",
			rosePineDawn: "#286983",
		},
		{
			name:         "notification is subtle",
			color:        func(th Theme) string { return string(th.Notification.DefaultFgColor) },
			rosePine:     "#908caa",
			rosePineDawn: "#797593",
		},
		{name: "flake is gold", color: func(th Theme) string { return string(th.Flake.Color) }, rosePine: "#f6c177", rosePineDawn: "#ea9d34"},
		{name: "installable is gold", color: func(th Theme) string { return string(th.Installable.Color) }, rosePine: "#f6c177", rosePineDawn: "#ea9d34"},
		{name: "output type is gold", color: func(th Theme) string { return string(th.OutputType.Color) }, rosePine: "#f6c177", rosePineDawn: "#ea9d34"},
		{name: "machine is foam", color: func(th Theme) string { return string(th.Machine.Color) }, rosePine: "#9ccfd8", rosePineDawn: "#56949f"},
		{name: "phase is rose", color: func(th Theme) string { return string(th.Phase.Color) }, rosePine: "#ebbcba", rosePineDawn: "#d7827e"},
		{name: "command is iris", color: func(th Theme) string { return string(th.Command.Color) }, rosePine: "#c4a7e7", rosePineDawn: "#907aa9"},
		{name: "error is love", color: func(th Theme) string { return string(th.Error.Color) }, rosePine: "#eb6f92", rosePineDawn: "#b4637a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.rosePine, tt.color(themes["rose-pine"]))
			assert.Equal(t, tt.rosePineDawn, tt.color(themes["rose-pine-dawn"]))
		})
	}
}

// TestBuiltinThemesRosePineGradientPairs asserts the gradient pairs use the
// documented adjacent-hue pairs.
func TestBuiltinThemesRosePineGradientPairs(t *testing.T) {
	t.Parallel()

	themes := builtinThemes()

	tests := []struct {
		name         string
		pair         func(Theme) ThemePair
		rosePine     ThemePair
		rosePineDawn ThemePair
	}{
		{
			name:         "running is iris+foam",
			pair:         func(th Theme) ThemePair { return th.PhaseStatus.Running },
			rosePine:     ThemePair{"#c4a7e7", "#9ccfd8"},
			rosePineDawn: ThemePair{"#907aa9", "#56949f"},
		},
		{
			name:         "failed is love+rose",
			pair:         func(th Theme) ThemePair { return th.PhaseStatus.Failed },
			rosePine:     ThemePair{"#eb6f92", "#ebbcba"},
			rosePineDawn: ThemePair{"#b4637a", "#d7827e"},
		},
		{
			name:         "done is pine+foam",
			pair:         func(th Theme) ThemePair { return th.PhaseStatus.Done },
			rosePine:     ThemePair{"#31748f", "#9ccfd8"},
			rosePineDawn: ThemePair{"#286983", "#56949f"},
		},
		{
			name:         "default is muted+subtle",
			pair:         func(th Theme) ThemePair { return th.PhaseStatus.Default },
			rosePine:     ThemePair{"#6e6a86", "#908caa"},
			rosePineDawn: ThemePair{"#9893a5", "#797593"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.rosePine, tt.pair(themes["rose-pine"]))
			assert.Equal(t, tt.rosePineDawn, tt.pair(themes["rose-pine-dawn"]))
		})
	}
}

// TestBuiltinThemesRosePineStyle pins the full curated style of the Rosé Pine
// themes. Bold flags are explicit: roles omitted from the curated bold list stay
// non-bold even where the default theme is bold (the flake entity).
func TestBuiltinThemesRosePineStyle(t *testing.T) {
	t.Parallel()

	rosePine, ok := builtinTheme("rose-pine")
	require.True(t, ok)

	assert.Equal(t, ThemeBorder{Table: "rounded", Overlay: "rounded", Viewport: "rounded", Notification: "rounded"}, clearDerived(rosePine.Border))
	assert.Equal(t, ThemeChars{
		HeaderSeparator: "│", SnapshotIcon: "❖", ErrorIcon: "✕",
		RowSpanMarker: "↪", HeaderTitleSep: "•", PhaseArrow: "➜",
	}, clearDerived(rosePine.Chars))
	assert.Equal(t, ThemeStatusIcons{OK: "✓", Failed: "✗", Running: "🔄"}, clearDerived(rosePine.Status.Icons))
	assert.Equal(t, [7]string{"❄", "📦", "🧬", "🖥", "📋", "⚙", "⚠"}, entityIcons(rosePine))
	assert.Equal(t, new(true), rosePine.PhaseStatus.Pill.Bold)
	assert.Equal(t, new(1), rosePine.PhaseStatus.Pill.PadX)
	assert.Equal(t, new(true), rosePine.Footer.HelpSelectedKey.Bold)
	assert.Equal(t, new(false), rosePine.Flake.Bold, "the flake bold flag must be explicitly false")
	assert.Equal(t, new(false), rosePine.Header.Title.Bold, "the header title bold flag must be explicitly false")
	assert.Equal(t, []string{"◇", "◈", "◆", "◈"}, rosePine.Spinner.Frames)
	assert.Equal(t, yamlx.Duration(150*time.Millisecond), rosePine.Spinner.Interval)

	dawn, ok := builtinTheme("rose-pine-dawn")
	require.True(t, ok)

	assert.Equal(t, ThemeBorder{Table: "blank", Overlay: "rounded", Viewport: "blank", Notification: "rounded"}, clearDerived(dawn.Border))
	assert.Equal(t, ThemeChars{
		HeaderSeparator: "•", SnapshotIcon: "✿", ErrorIcon: "✗",
		RowSpanMarker: "↪", HeaderTitleSep: "·", PhaseArrow: "▶",
	}, clearDerived(dawn.Chars))
	assert.Equal(t, ThemeStatusIcons{OK: "✅", Failed: "✖", Running: "⏳"}, clearDerived(dawn.Status.Icons))
	assert.Equal(t, [7]string{"📁", "📦", "🗂", "🖥", "🪜", "⚙", "⚠"}, entityIcons(dawn))
	assert.Equal(t, new(true), dawn.PhaseStatus.Pill.Bold)
	assert.Equal(t, new(2), dawn.PhaseStatus.Pill.PadX)
	assert.Equal(t, new(false), dawn.Flake.Bold, "the flake bold flag must be explicitly false")
	assert.Equal(t, new(false), dawn.Header.Title.Bold, "the header title bold flag must be explicitly false")
	assert.Equal(t, new(false), dawn.Footer.HelpSelectedKey.Bold, "the footer help selected key bold flag must be explicitly false")
	assert.Equal(t, []string{"°", "☼", "☀", "☼"}, dawn.Spinner.Frames)
	assert.Equal(t, yamlx.Duration(170*time.Millisecond), dawn.Spinner.Interval)
}

// entityIcons returns the log entity icons in a fixed order (flake, installable,
// output type, machine, phase, command, error).
func entityIcons(theme Theme) [7]string {
	return [7]string{
		theme.Flake.Icon,
		theme.Installable.Icon,
		theme.OutputType.Icon,
		theme.Machine.Icon,
		theme.Phase.Icon,
		theme.Command.Icon,
		theme.Error.Icon,
	}
}

// TestRosePineDawnForegroundsReadableOnLightBase asserts every foreground role of
// the merged rose-pine-dawn theme uses the dark palette: each must be darker than
// the light base background.
func TestRosePineDawnForegroundsReadableOnLightBase(t *testing.T) {
	t.Parallel()

	dawn := defaultTheme().Merge(builtinThemes()["rose-pine-dawn"])

	baseLuminance, err := relativeLuminance(rosePineBaseDawn)
	require.NoError(t, err)

	foregrounds := map[string]string{
		"header title":              string(dawn.Header.Title.Fg),
		"status ok":                 string(dawn.Status.OK.Fg),
		"status warning":            string(dawn.Status.Warning.Fg),
		"status failed":             string(dawn.Status.Failed.Fg),
		"status running":            string(dawn.Status.Running.Fg),
		"table border":              string(dawn.Table.Border.Fg),
		"table row":                 string(dawn.Table.Row.Fg),
		"pill label":                string(dawn.PhaseStatus.Pill.Fg),
		"tree enumerator":           string(dawn.Tree.Enumerator.Fg),
		"footer help key":           string(dawn.Footer.HelpKey.Fg),
		"footer help desc":          string(dawn.Footer.HelpDesc.Fg),
		"footer help separator":     string(dawn.Footer.HelpSeparator.Fg),
		"footer help selected key":  string(dawn.Footer.HelpSelectedKey.Fg),
		"footer help selected desc": string(dawn.Footer.HelpSelectedDesc.Fg),
		"notification":              string(dawn.Notification.DefaultFgColor),
		"flake":                     string(dawn.Flake.Color),
		"installable":               string(dawn.Installable.Color),
		"output type":               string(dawn.OutputType.Color),
		"machine":                   string(dawn.Machine.Color),
		"phase":                     string(dawn.Phase.Color),
		"command":                   string(dawn.Command.Color),
		"error":                     string(dawn.Error.Color),
	}

	for role, foreground := range foregrounds {
		t.Run(role, func(t *testing.T) {
			t.Parallel()

			foregroundLuminance, parseErr := relativeLuminance(foreground)
			require.NoError(t, parseErr)

			assert.Less(t, foregroundLuminance, baseLuminance,
				"foreground %s (%s) must be darker than the light base %s", role, foreground, rosePineBaseDawn)
		})
	}
}

// TestBuiltinThemesRenderCanonicalColors asserts the merged themes render the
// canonical colors into the runtime styles, end to end through derivation.
func TestBuiltinThemesRenderCanonicalColors(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	tests := []struct {
		name     string
		theme    string
		okColor  string
		rowColor string
	}{
		{name: "rose-pine renders pine and text", theme: "rose-pine", okColor: "#31748f", rowColor: "#e0def4"},
		{name: "rose-pine-dawn renders pine and text", theme: "rose-pine-dawn", okColor: "#286983", rowColor: "#464261"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme, ok := reg.Build(tt.theme)
			require.True(t, ok)

			assert.Equal(t, style.Color(tt.okColor), scheme.Status.OK.Style.GetForeground())
			assert.Equal(t, style.Color(tt.rowColor), scheme.Table.Row.Style.GetForeground())
		})
	}
}

// relativeLuminance computes the WCAG relative luminance of a hex color.
func relativeLuminance(hex string) (float64, error) {
	parsed, err := style.ParseColor(hex)
	if err != nil {
		return 0, errors.Wrapf(err, "parse color %q", hex)
	}

	red, green, blue := style.ColorToRGB8(parsed)

	lin := func(channel uint8) float64 {
		value := float64(channel) / 255.0 //nolint:mnd // 8-bit channel scaling
		if value <= 0.03928 {             //nolint:mnd // WCAG threshold
			return value / 12.92 //nolint:mnd // WCAG slope
		}

		return math.Pow((value+0.055)/1.055, 2.4) //nolint:mnd // WCAG gamma
	}

	return 0.2126*lin(red) + 0.7152*lin(green) + 0.0722*lin(blue), nil //nolint:mnd // WCAG coefficients
}
