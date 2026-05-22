package style

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestCellWidth_Equivalence(t *testing.T) {
	t.Parallel()

	tests := []string{
		"hello",
		"📋BUILD",
		"1 ",
		"",
		"📋 📦 💻 📋 ⚙ ✗",
		"nix build .#nixosConfigurations.machine",
		// Multi-line: should return max line width, not sum
		"line1\nline2longer\nline3",
		"╭──────╮\n│ text │\n╰──────╯\n",
		"short\nthis is a much longer line\nmid",
	}

	for _, tc := range tests {
		assert.Equal(t, lipgloss.Width(tc), CellWidth([]byte(tc)), "CellWidth(%q)", tc)
	}
}

func TestCellWidth_ZoneMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		wantWidth int
	}{
		{"\x1b[1001zhello\x1b[1001z", 5},
		{"\x1b[1001zhello world\x1b[1001z", 11},
		{"\x1b[1001z\x1b[38;2;255;0;0mred\x1b[m\x1b[1001z", 3},
		{"prefix\x1b[1001zmiddle\x1b[1001zsuffix", 18},
		{"\x1b[1001z\x1b[1002zdouble\x1b[1002z\x1b[1001z", 6},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.wantWidth, CellWidth([]byte(tc.input)), "CellWidth(%q)", tc.input)
	}
}

func TestCellWidth_OtherCSISequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		wantWidth int
	}{
		{"\x1b[1Ahello", 5},
		{"\x1b[2Chello", 5},
		{"\x1b[?25lhello", 5},
		{"\x1b[?25h\x1b[2Jhello", 5},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.wantWidth, CellWidth([]byte(tc.input)), "CellWidth(%q)", tc.input)
	}
}

func TestSkipANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"\x1b[31m", 5},
		{"\x1b[1;32m", 7},
		{"\x1b[38;2;255;0;0m", 15},
		{"\x1b[?25l", 6},
		{"a", 0},
		{"\x1b", 1},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, skipANSI([]byte(tc.input), 0), "skipANSI(%q)", tc.input)
	}
}

func TestStripANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"no ansi", "no ansi"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1mbold\x1b[0m", "bold"},
		{"\x1b[38;2;255;0;0mRGBcolor\x1b[0m", "RGBcolor"},
		{"mix\x1b[31m of\x1b[0m ansi", "mix of ansi"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, string(StripANSI([]byte(tc.input))), "StripANSI(%q)", tc.input)
	}
}

func TestStripANSI_RendersToEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, string(StripANSI([]byte(""))))
	assert.Empty(t, string(StripANSI([]byte("\x1b[31m\x1b[0m"))))
	assert.Equal(t, "x", string(StripANSI([]byte("\x1b[31mx\x1b[0m"))))
}
