package style

import (
	"testing"

	"charm.land/lipgloss/v2"
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
		expected := lipgloss.Width(tc)
		got := CellWidth(tc)

		if expected != got {
			t.Errorf("CellWidth(%q) = %d, want %d", tc, got, expected)
		}
	}
}

func TestCellWidth_ZoneMarkers(t *testing.T) {
	t.Parallel()

	// bubblezone zone markers use CSI private sequences: \x1b[<id>z
	// These are zero-width and must not be counted as visible cells.
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
		got := CellWidth(tc.input)
		if got != tc.wantWidth {
			t.Errorf("CellWidth(%q) = %d, want %d", tc.input, got, tc.wantWidth)
		}
	}
}

func TestCellWidth_OtherCSISequences(t *testing.T) {
	t.Parallel()

	// Non-SGR CSI sequences (cursor movement, etc.) are also zero-width.
	tests := []struct {
		input     string
		wantWidth int
	}{
		{"\x1b[1Ahello", 5},           // Cursor Up
		{"\x1b[2Chello", 5},           // Cursor Forward
		{"\x1b[?25lhello", 5},         // Private CSI (hide cursor)
		{"\x1b[?25h\x1b[2Jhello", 5}, // Private + Erase Display
	}

	for _, tc := range tests {
		got := CellWidth(tc.input)
		if got != tc.wantWidth {
			t.Errorf("CellWidth(%q) = %d, want %d", tc.input, got, tc.wantWidth)
		}
	}
}

func TestSkipANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		start int
		want  int
	}{
		// SGR sequences
		{"\x1b[m", 0, 3},
		{"\x1b[38;2;255;0;0m", 0, 15},
		{"\x1b[0m", 0, 4},
		// CSI private (zone markers)
		{"\x1b[1001z", 0, 7},
		{"\x1b[1001zhello", 0, 7},
		// CSI with ? prefix (private mode)
		{"\x1b[?25l", 0, 6},
		{"\x1b[?25h", 0, 6},
		// OSC sequences (BEL-terminated)
		{"\x1b]0;title\x07", 0, 10},
		// OSC sequences (ST-terminated)
		{"\x1b]0;title\x1b\\", 0, 11},
		// Bare ESC + byte
		{"\x1bO", 0, 2},
		// Incomplete sequences
		{"\x1b", 0, 1},
		{"\x1b[", 0, 2},
		{"\x1b[38", 0, 4},
	}

	for _, tc := range tests {
		got := skipANSI(tc.input, tc.start)
		if got != tc.want {
			t.Errorf("skipANSI(%q, %d) = %d, want %d", tc.input, tc.start, got, tc.want)
		}
	}
}

func TestCountLines(t *testing.T) {
	t.Parallel()

	tests := []string{
		"hello",
		"line1\nline2\nline3",
		"",
		"\n\n\n",
		"single",
	}

	for _, tc := range tests {
		expected := lipgloss.Height(tc)
		got := CountLines(tc)

		if expected != got {
			t.Errorf("CountLines(%q) = %d, want %d", tc, got, expected)
		}
	}
}

func TestCellWidth_EmojiGrapheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"👍🏽", 2},     // skin tone modifier: single grapheme, width 2
		{"👨‍👩‍👧", 2},  // ZWJ family: single grapheme, width 2
		{"🚀", 2},     // simple emoji, width 2
		{"👍🏽Hi", 4},   // emoji(2) + H(1) + i(1)
		{"🇺🇸", 2},     // flag: single grapheme, width 2
	}

	for _, tc := range tests {
		got := CellWidth(tc.input)
		if got != tc.want {
			t.Errorf("CellWidth(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
