package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalBorder(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	assert.Equal(t, []byte("┌"), brd.TopLeft)
	assert.Equal(t, []byte("┐"), brd.TopRight)
	assert.Equal(t, []byte("└"), brd.BottomLeft)
	assert.Equal(t, []byte("┘"), brd.BottomRight)
	assert.Equal(t, []byte("─"), brd.Horizontal)
	assert.Equal(t, []byte("│"), brd.Vertical)
	assert.Equal(t, []byte("┬"), brd.TopMid)
	assert.Equal(t, []byte("┴"), brd.BottomMid)
	assert.Equal(t, []byte("├"), brd.LeftMid)
	assert.Equal(t, []byte("┤"), brd.RightMid)
	assert.Equal(t, []byte("┼"), brd.MidMid)
}

func TestRoundedBorder(t *testing.T) {
	t.Parallel()

	brd := RoundedBorder()

	assert.Equal(t, []byte("╭"), brd.TopLeft)
	assert.Equal(t, []byte("╮"), brd.TopRight)
	assert.Equal(t, []byte("╰"), brd.BottomLeft)
	assert.Equal(t, []byte("╯"), brd.BottomRight)
	assert.Equal(t, []byte("─"), brd.Horizontal)
	assert.Equal(t, []byte("│"), brd.Vertical)
}

func TestHiddenBorder(t *testing.T) {
	t.Parallel()

	brd := HiddenBorder()

	assert.Empty(t, brd.TopLeft)
	assert.Empty(t, brd.Horizontal)
	assert.Empty(t, brd.Vertical)
}

func TestMarkdownBorder(t *testing.T) {
	t.Parallel()

	brd := MarkdownBorder()

	assert.Equal(t, []byte("|"), brd.TopLeft)
	assert.Equal(t, []byte("|"), brd.TopRight)
	assert.Equal(t, []byte("|"), brd.BottomLeft)
	assert.Equal(t, []byte("|"), brd.BottomRight)
	assert.Equal(t, []byte("-"), brd.Horizontal)
	assert.Equal(t, []byte("|"), brd.Vertical)
}

func TestBorder_NoPerSideColorByDefault(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	assert.Empty(t, brd.topFg)
	assert.Empty(t, brd.rightFg)
	assert.Empty(t, brd.bottomFg)
	assert.Empty(t, brd.leftFg)
}

// TestBorderPresets pins the full glyph table of every named preset in the
// registry: BorderByName must resolve each name to exactly these bytes.
//
//nolint:funlen // large preset glyph table
func TestBorderPresets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected Border
	}{
		{
			name: "sharp",
			expected: Border{
				TopLeft: []byte("┌"), TopRight: []byte("┐"),
				BottomLeft: []byte("└"), BottomRight: []byte("┘"),
				Horizontal: []byte("─"), Vertical: []byte("│"),
				TopMid: []byte("┬"), BottomMid: []byte("┴"),
				LeftMid: []byte("├"), RightMid: []byte("┤"), MidMid: []byte("┼"),
			},
		},
		{
			name: "rounded",
			expected: Border{
				TopLeft: []byte("╭"), TopRight: []byte("╮"),
				BottomLeft: []byte("╰"), BottomRight: []byte("╯"),
				Horizontal: []byte("─"), Vertical: []byte("│"),
				TopMid: []byte("┬"), BottomMid: []byte("┴"),
				LeftMid: []byte("├"), RightMid: []byte("┤"), MidMid: []byte("┼"),
			},
		},
		{
			name: "double",
			expected: Border{
				TopLeft: []byte("╔"), TopRight: []byte("╗"),
				BottomLeft: []byte("╚"), BottomRight: []byte("╝"),
				Horizontal: []byte("═"), Vertical: []byte("║"),
				TopMid: []byte("╦"), BottomMid: []byte("╩"),
				LeftMid: []byte("╠"), RightMid: []byte("╣"), MidMid: []byte("╬"),
			},
		},
		{
			name: "heavy",
			expected: Border{
				TopLeft: []byte("┏"), TopRight: []byte("┓"),
				BottomLeft: []byte("┗"), BottomRight: []byte("┛"),
				Horizontal: []byte("━"), Vertical: []byte("┃"),
				TopMid: []byte("┳"), BottomMid: []byte("┻"),
				LeftMid: []byte("┣"), RightMid: []byte("┫"), MidMid: []byte("╋"),
			},
		},
		{
			name: "ascii",
			expected: Border{
				TopLeft: []byte("+"), TopRight: []byte("+"),
				BottomLeft: []byte("+"), BottomRight: []byte("+"),
				Horizontal: []byte("-"), Vertical: []byte("|"),
				TopMid: []byte("+"), BottomMid: []byte("+"),
				LeftMid: []byte("+"), RightMid: []byte("+"), MidMid: []byte("+"),
			},
		},
		{
			name: "dashed",
			expected: Border{
				TopLeft: []byte("┌"), TopRight: []byte("┐"),
				BottomLeft: []byte("└"), BottomRight: []byte("┘"),
				Horizontal: []byte("╌"), Vertical: []byte("┊"),
				TopMid: []byte("┬"), BottomMid: []byte("┴"),
				LeftMid: []byte("├"), RightMid: []byte("┤"), MidMid: []byte("┼"),
			},
		},
		{
			name: "blank",
			expected: Border{
				TopLeft: []byte(" "), TopRight: []byte(" "),
				BottomLeft: []byte(" "), BottomRight: []byte(" "),
				Horizontal: []byte(" "), Vertical: []byte(" "),
				TopMid: []byte(" "), BottomMid: []byte(" "),
				LeftMid: []byte(" "), RightMid: []byte(" "), MidMid: []byte(" "),
			},
		},
		{
			name: "markdown",
			expected: Border{
				TopLeft: []byte("|"), TopRight: []byte("|"),
				BottomLeft: []byte("|"), BottomRight: []byte("|"),
				Horizontal: []byte("-"), Vertical: []byte("|"),
				TopMid: []byte("|"), BottomMid: []byte("|"),
				LeftMid: []byte("|"), RightMid: []byte("|"), MidMid: []byte("|"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := BorderByName(tt.name)

			require.True(t, ok, "preset %q must be known", tt.name)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestNamedConstructorsMatchPresets proves the named constructors resolve
// through the registry: no constructor may carry its own glyph table.
func TestNamedConstructorsMatchPresets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		get        func() Border
		presetName string
	}{
		{name: "sharp", get: NormalBorder, presetName: "sharp"},
		{name: "rounded", get: RoundedBorder, presetName: "rounded"},
		{name: "markdown", get: MarkdownBorder, presetName: "markdown"},
		{name: "hidden stays zero value", get: HiddenBorder, presetName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.presetName == "" {
				assert.Equal(t, Border{}, tt.get())

				return
			}

			expected, ok := BorderByName(tt.presetName)
			require.True(t, ok)
			assert.Equal(t, expected, tt.get())
		})
	}
}

// TestBorderByName_Unknown proves unknown names never resolve to glyphs:
// callers fall back to a default preset instead of guessing.
func TestBorderByName_Unknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		lookup string
	}{
		{name: "empty name", lookup: ""},
		{name: "unknown name", lookup: "no-such-border"},
		{name: "case sensitive", lookup: "SHARP"},
		{name: "hidden is not a preset", lookup: "hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := BorderByName(tt.lookup)

			assert.False(t, ok)
			assert.Empty(t, got.TopLeft)
			assert.Empty(t, got.Horizontal)
			assert.Empty(t, got.Vertical)
		})
	}
}

// TestBorderNames proves BorderNames is the sorted registry key set.
func TestBorderNames(t *testing.T) {
	t.Parallel()

	expected := []string{"ascii", "blank", "dashed", "double", "heavy", "markdown", "rounded", "sharp"}

	assert.Equal(t, expected, BorderNames())
}

// TestBorderPresets_SingleCellGlyphs guards the invariant every border layout
// site relies on: each registry glyph occupies exactly one terminal cell, so
// border math can count glyphs and cells interchangeably.
func TestBorderPresets_SingleCellGlyphs(t *testing.T) {
	t.Parallel()

	for _, name := range BorderNames() {
		brd, ok := BorderByName(name)
		require.True(t, ok, "preset %q must resolve", name)

		glyphs := map[string][]byte{
			"TopLeft":     brd.TopLeft,
			"TopRight":    brd.TopRight,
			"BottomLeft":  brd.BottomLeft,
			"BottomRight": brd.BottomRight,
			"Horizontal":  brd.Horizontal,
			"Vertical":    brd.Vertical,
			"TopMid":      brd.TopMid,
			"BottomMid":   brd.BottomMid,
			"LeftMid":     brd.LeftMid,
			"RightMid":    brd.RightMid,
			"MidMid":      brd.MidMid,
		}

		for field, glyph := range glyphs {
			assert.Equal(t, 1, CellWidth(glyph),
				"preset %q field %s glyph %q must be exactly one cell", name, field, glyph)
		}
	}
}
