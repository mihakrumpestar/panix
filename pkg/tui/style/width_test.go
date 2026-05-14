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
		got := CellWidth([]byte(tc))

		if expected != got {
			t.Errorf("CellWidth(%q) = %d, want %d", tc, got, expected)
		}
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
		got := CellWidth([]byte(tc.input))
		if got != tc.wantWidth {
			t.Errorf("CellWidth(%q) = %d, want %d", tc.input, got, tc.wantWidth)
		}
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
		got := CellWidth([]byte(tc.input))
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
		{"\x1b[m", 0, 3},
		{"\x1b[38;2;255;0;0m", 0, 15},
		{"\x1b[0m", 0, 4},
		{"\x1b[1001z", 0, 7},
		{"\x1b[1001zhello", 0, 7},
		{"\x1b[?25l", 0, 6},
		{"\x1b[?25h", 0, 6},
		{"\x1b]0;title\x07", 0, 10},
		{"\x1b]0;title\x1b\\", 0, 11},
		{"\x1bO", 0, 2},
		{"\x1b", 0, 1},
		{"\x1b[", 0, 2},
		{"\x1b[38", 0, 4},
	}

	for _, tc := range tests {
		got := skipANSI([]byte(tc.input), tc.start)
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
		{"👍🏽", 2},
		{"👨‍👩‍👧", 2},
		{"🚀", 2},
		{"👍🏽Hi", 4},
		{"🇺🇸", 2},
	}

	for _, tc := range tests {
		got := CellWidth([]byte(tc.input))
		if got != tc.want {
			t.Errorf("CellWidth(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ansi", "plain text", "plain text"},
		{"single SGR", "\x1b[31mred\x1b[0m", "red"},
		{"multiple SGR", "\x1b[1;32mgreen\x1b[0m \x1b[34mblue\x1b[0m", "green blue"},
		{"RGB SGR", "\x1b[38;2;255;0;0mRGB\x1b[0m", "RGB"},
		{"empty", "", ""},
		{"only ansi", "\x1b[31m\x1b[0m", ""},
		{"CSI erase line", "before\x1b[Kafter", "beforeafter"},
		{"OSC title", "\x1b]0;window title\x07visible", "visible"},
		{"OSC ST terminator", "\x1b]0;title\x1b\\visible", "visible"},
		{"bare ESC sequence", "\x1bOvisible", "visible"},
		{"mixed", "\x1b[1mbold\x1b[0m and \x1b[32mgreen\x1b[0m", "bold and green"},
		{"no esc fast path", "hello world", "hello world"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := StripANSI([]byte(test.input))
			if string(got) != test.want {
				t.Errorf("StripANSI(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestStripANSIBytesZeroCopy(t *testing.T) {
	t.Parallel()

	data := []byte("no ansi here")

	result := StripANSI(data)
	// When no ESC bytes present, StripANSI returns the input as a sub-slice.
	if len(result) > 0 && len(data) > 0 && &result[0] != &data[0] {
		t.Error("expected zero-copy when no ESC bytes present")
	}
}
