package theme

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/tui/spinner"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestThemeBorderValidateTagsMatchRegistry pins the schema-facing validate tags on
// ThemeBorder to the border preset registry. The generated schema enum and the
// runtime Theme.Validate check must agree on exactly style.BorderNames().
func TestThemeBorderValidateTagsMatchRegistry(t *testing.T) {
	t.Parallel()

	expected := style.BorderNames()
	borderReflectType := reflect.TypeFor[ThemeBorder]()

	for field := range borderReflectType.Fields() {
		if field.Tag.Get("yaml") == "-" {
			continue // derived render field: no validate tag
		}

		t.Run(field.Name, func(t *testing.T) {
			t.Parallel()

			got := oneofValues(t, field.Tag.Get("validate"))

			assert.ElementsMatch(t, expected, got,
				"validate tag enum must match style.BorderNames() exactly")
		})
	}
}

// oneofValues extracts the values of the first "oneof=" segment from a validate
// tag (the grammar pkg/yamlschema consumes).
func oneofValues(t *testing.T, validateTag string) []string {
	t.Helper()

	for segment := range strings.SplitSeq(validateTag, ",") {
		value, ok := strings.CutPrefix(strings.TrimSpace(segment), "oneof=")
		if ok {
			return strings.Fields(value)
		}
	}

	require.FailNow(t, "validate tag carries no oneof constraint", "tag: %q", validateTag)

	return nil
}

// TestThemeValidate is the value rules table for Theme.Validate: every error names
// the YAML field path and offending value, while zero/empty values stay valid.
//
//nolint:funlen,maintidx // large validation table
func TestThemeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		theme          Theme
		wantErrSubstrs []string
	}{
		{
			name:  "empty theme is valid",
			theme: Theme{},
		},
		{
			name: "valid colors and glyphs",
			theme: Theme{
				Status: ThemeStatus{OK: ThemeStyle{Fg: "#31748f", Bg: "139"}},
				Chars:  ThemeChars{PhaseArrow: "=>"},
			},
		},
		{
			name: "valid gradient pair",
			theme: Theme{
				PhaseStatus: ThemePhaseStatus{Running: ThemePair{"#c4a7e7", "#9ccfd8"}},
			},
		},
		{
			name: "pair with empty side is invalid",
			theme: Theme{
				PhaseStatus: ThemePhaseStatus{Running: ThemePair{"#c4a7e7", ""}},
			},
			wantErrSubstrs: []string{"phase_status.running[1]", "incomplete"},
		},
		{
			name: "valid border presets",
			theme: Theme{
				Border: ThemeBorder{Table: "sharp", Overlay: "rounded", Viewport: "double", Notification: "blank"},
			},
		},
		{
			name: "valid spinner frames",
			theme: Theme{
				Spinner: ThemeSpinner{Frames: []string{"⣾", "⣽"}, Interval: yamlx.Duration(120 * time.Millisecond)},
			},
		},
		{
			name: "valid spinner interval bounds",
			theme: Theme{
				Spinner: ThemeSpinner{Interval: yamlx.Duration(spinner.MinInterval)},
			},
		},
		{
			name: "valid maximum spinner interval",
			theme: Theme{
				Spinner: ThemeSpinner{Interval: yamlx.Duration(spinner.MaxInterval)},
			},
		},
		{
			name: "valid mode",
			theme: Theme{
				Mode: "light",
			},
		},
		{
			name: "entities and notification with valid colors",
			theme: Theme{
				Notification: ThemeNotification{DefaultFgColor: "139"},
				Machine:      ThemeLogEntity{Color: "#9ccfd8", Icon: "🖥"},
			},
		},
		{
			name: "icons and glyphs are free-form",
			theme: Theme{
				Chars:   ThemeChars{HeaderSeparator: "anything"},
				Status:  ThemeStatus{Icons: ThemeStatusIcons{OK: "anything"}},
				Flake:   ThemeLogEntity{Icon: "anything"},
				Spinner: ThemeSpinner{Frames: []string{"anything"}},
			},
		},
		{
			name: "bad foreground color",
			theme: Theme{
				Status: ThemeStatus{OK: ThemeStyle{Fg: "#zzzzzz"}},
			},
			wantErrSubstrs: []string{"status.ok.fg", `"#zzzzzz"`},
		},
		{
			name: "bad nested header color",
			theme: Theme{
				Header: ThemeHeader{Title: ThemeStyle{Fg: "#zzzzzz"}},
			},
			wantErrSubstrs: []string{"header.title.fg", `"#zzzzzz"`},
		},
		{
			name: "bad background color",
			theme: Theme{
				Table: ThemeTableAndLogs{Row: ThemeStyle{Bg: "999"}},
			},
			wantErrSubstrs: []string{"table.row.bg", `"999"`},
		},
		{
			name: "bad gradient pair side",
			theme: Theme{
				PhaseStatus: ThemePhaseStatus{Done: ThemePair{"#14532D", "#zzzzzz"}},
			},
			wantErrSubstrs: []string{"phase_status.done[1]", `"#zzzzzz"`},
		},
		{
			name: "bad notification color",
			theme: Theme{
				Notification: ThemeNotification{DefaultFgColor: "#zzzzzz"},
			},
			wantErrSubstrs: []string{"notification.default_fg_color", `"#zzzzzz"`},
		},
		{
			name: "bad entity color",
			theme: Theme{
				Error: ThemeLogEntity{Color: "#zzzzzz"},
			},
			wantErrSubstrs: []string{"error.color", `"#zzzzzz"`},
		},
		{
			name: "unknown border preset",
			theme: Theme{
				Border: ThemeBorder{Overlay: "squiggly"},
			},
			wantErrSubstrs: []string{"border.overlay", "squiggly"},
		},
		{
			name: "empty spinner frame",
			theme: Theme{
				Spinner: ThemeSpinner{Frames: []string{"|", ""}},
			},
			wantErrSubstrs: []string{"spinner.frames[1]", "must not be empty"},
		},
		{
			name: "mixed-width spinner frames",
			theme: Theme{
				Spinner: ThemeSpinner{Frames: []string{"--", "|"}},
			},
			wantErrSubstrs: []string{"spinner.frames[1]", "width"},
		},
		{
			name: "zero-width spinner frame",
			theme: Theme{
				Spinner: ThemeSpinner{Frames: []string{"\u0301"}},
			},
			wantErrSubstrs: []string{"spinner.frames[0]", "at least one cell"},
		},
		{
			name: "negative spinner interval",
			theme: Theme{
				Spinner: ThemeSpinner{Interval: yamlx.Duration(-time.Second)},
			},
			wantErrSubstrs: []string{"spinner.interval", "-1s", "between"},
		},
		{
			name: "nanosecond spinner interval",
			theme: Theme{
				Spinner: ThemeSpinner{Interval: yamlx.Duration(120 * time.Nanosecond)},
			},
			wantErrSubstrs: []string{"spinner.interval", "120ns", "between"},
		},
		{
			name: "too short spinner interval",
			theme: Theme{
				Spinner: ThemeSpinner{Interval: yamlx.Duration(5 * time.Millisecond)},
			},
			wantErrSubstrs: []string{"spinner.interval", "5ms", "between"},
		},
		{
			name: "padding above the theme limit",
			theme: Theme{
				PhaseStatus: ThemePhaseStatus{Pill: ThemeStyle{PadX: new(5000)}},
			},
			wantErrSubstrs: []string{"phase_status.pill.pad_x", "between 0 and 16", "5000"},
		},
		{
			name: "negative padding",
			theme: Theme{
				Table: ThemeTableAndLogs{Row: ThemeStyle{PadY: new(-5)}},
			},
			wantErrSubstrs: []string{"table.row.pad_y", "between 0 and 16", "-5"},
		},
		{
			name: "unknown mode",
			theme: Theme{
				Mode: "sepia",
			},
			wantErrSubstrs: []string{"mode", "sepia"},
		},
		{
			name: "control character in a glyph",
			theme: Theme{
				Chars: ThemeChars{PhaseArrow: "->\x1b[31m"},
			},
			wantErrSubstrs: []string{"chars.phase_arrow", "control characters"},
		},
		{
			name: "newline in a status icon",
			theme: Theme{
				Status: ThemeStatus{Icons: ThemeStatusIcons{OK: "ok\n"}},
			},
			wantErrSubstrs: []string{"status.icons.ok", "control characters"},
		},
		{
			name: "control character in a log icon",
			theme: Theme{
				Flake: ThemeLogEntity{Icon: "\x1b]0;title\x07"},
			},
			wantErrSubstrs: []string{"flake.icon", "control characters"},
		},
		{
			name: "control character in a spinner frame",
			theme: Theme{
				Spinner: ThemeSpinner{Frames: []string{"|\n"}},
			},
			wantErrSubstrs: []string{"spinner.frames[0]", "control characters"},
		},
		{
			name: "valid tree glyphs",
			theme: Theme{
				Tree: ThemeTree{Branch: "├", BranchLast: "╰", Vertical: "│", Horizontal: "─"},
			},
		},
		{
			name: "empty tree glyph when the others are set",
			theme: Theme{
				Tree: ThemeTree{BranchLast: "╰", Vertical: "│", Horizontal: "─"},
			},
			wantErrSubstrs: []string{"tree.branch", "must not be empty"},
		},
		{
			name: "wide tree glyph",
			theme: Theme{
				Tree: ThemeTree{Branch: "ab", BranchLast: "╰", Vertical: "│", Horizontal: "─"},
			},
			wantErrSubstrs: []string{"tree.branch", "one cell"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.theme.Validate()

			if len(tt.wantErrSubstrs) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, substr := range tt.wantErrSubstrs {
				assert.Contains(t, err.Error(), substr)
			}
		})
	}
}

// TestDefaultAndBuiltinThemesAreValid guards the built-in themes against the value
// rules: they must never fail theme validation.
func TestDefaultAndBuiltinThemesAreValid(t *testing.T) {
	t.Parallel()

	require.NoError(t, defaultTheme().Validate())

	for name, theme := range builtinThemes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, theme.Validate())
		})
	}
}
