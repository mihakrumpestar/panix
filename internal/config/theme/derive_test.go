package theme

import (
	"bytes"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecodeDerivesRenderFields proves the decode path ends derived: parsing a
// themes file that changes a color, glyph and border preset yields render-ready
// derived fields for the new values.
func TestDecodeDerivesRenderFields(t *testing.T) {
	t.Parallel()

	path := writeThemesFile(t, `
themes:
  custom:
    status:
      ok:
        fg: "#123456"
    chars:
      phase_arrow: "=>"
    border:
      table: heavy
`)

	themes, err := ParseThemesFile(path)
	require.NoError(t, err)

	theme := themes["custom"]

	assert.Equal(t, style.Color("#123456"), theme.Status.OK.Style.GetForeground(),
		"the derived style must reflect the decoded color")

	assert.Equal(t, []byte("=>"), theme.Chars.PhaseArrowBytes,
		"the derived glyph bytes must reflect the decoded glyph")

	heavy, ok := style.BorderByName("heavy")
	require.True(t, ok)

	assert.Equal(t, heavy, theme.Border.TableGlyphs,
		"the derived border must reflect the decoded preset")
}

// TestMergeRederivesRenderFields proves Merge skips derived fields and re-derives
// the result: poisoned derived fields on either side must not survive.
func TestMergeRederivesRenderFields(t *testing.T) {
	t.Parallel()

	poisonStyle := style.NewStyle().Faint(true)
	poisonBytes := []byte("POISON")

	base := defaultTheme()
	base.Status.OK.Style = poisonStyle
	base.Chars.PhaseArrowBytes = poisonBytes
	base.Border.TableGlyphs = style.HiddenBorder()

	override := Theme{
		Status: ThemeStatus{OK: ThemeStyle{Fg: "#31748f"}},
		Chars:  ThemeChars{PhaseArrow: "->"},
	}
	override.Status.OK.Style = poisonStyle
	override.Chars.PhaseArrowBytes = poisonBytes
	override.Border.TableGlyphs = style.HiddenBorder()

	merged := base.Merge(override)

	assert.Equal(t, style.NewStyle().Foreground("#31748f"), merged.Status.OK.Style,
		"merged derived style must be re-derived from the override fields, not the poisoned value")
	assert.Equal(t, style.Color("#31748f"), merged.Status.OK.Style.GetForeground(),
		"the override color must win")

	assert.Equal(t, []byte("->"), merged.Chars.PhaseArrowBytes,
		"merged derived glyph bytes must be re-derived from the override glyph")

	assert.Equal(t, buildBorder(merged.Border.Table, defaultBorderTable), merged.Border.TableGlyphs,
		"merged derived border must be re-derived")
}

// TestValidateSkipsDerivedFields proves validation only checks user-supplied
// serializable data: poisoned derived fields still validate, bad serializable
// values still fail.
func TestValidateSkipsDerivedFields(t *testing.T) {
	t.Parallel()

	theme := defaultTheme()
	theme.Status.OK.Style = style.NewStyle().Faint(true)
	theme.Chars.PhaseArrowBytes = []byte("POISON")
	theme.Border.TableGlyphs = style.HiddenBorder()
	theme.Spinner.FrameBytes = [][]byte{[]byte("POISON")}

	require.NoError(t, theme.Validate(), "poisoned derived fields must not fail validation")

	bad := defaultTheme()
	bad.Status.OK.Fg = "#zzzzzz"

	require.Error(t, bad.Validate(), "a bad serializable color must still fail validation")
}

// TestSerializationIgnoresDerivedFields proves the derived fields are yaml:"-":
// encoding a built-in theme emits none of the derived keys, and decode plus
// re-derive reproduces the same values.
func TestSerializationIgnoresDerivedFields(t *testing.T) {
	t.Parallel()

	derivedKeys := []string{
		"style", "colors", "okbytes", "failedbytes", "runningbytes",
		"headerseparatorbytes", "snapshoticonbytes", "erroriconbytes",
		"rowspanmarkerbytes", "headertitlesepbytes", "phasearrowbytes",
		"tableglyphs", "overlayglyphs", "viewportglyphs", "notificationglyphs",
		"branchbytes", "branchlastbytes", "verticalbytes", "horizontalbytes",
		"framebytes", "iconbytes",
	}

	for name, theme := range builtinThemes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, yamlx.Encode(theme, &buf))

			var generic map[string]any
			require.NoError(t, yamlx.Decode(buf.Bytes(), &generic))

			keys := map[string]bool{}
			collectKeys(generic, keys)

			for _, key := range derivedKeys {
				assert.False(t, keys[key], "derived key %q must not be serialized", key)
			}

			var decoded Theme
			require.NoError(t, yamlx.Decode(buf.Bytes(), &decoded))
			decoded = decoded.derive()

			assert.Equal(t, theme, decoded, "derived values must round-trip through re-derivation")
		})
	}
}

// collectKeys records every map key in a decoded YAML tree.
func collectKeys(value any, out map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			out[key] = true

			collectKeys(child, out)
		}
	case []any:
		for _, child := range typed {
			collectKeys(child, out)
		}
	default:
	}
}
