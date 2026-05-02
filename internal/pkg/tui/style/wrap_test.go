package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestWrap_Equivalence(t *testing.T) {
	t.Parallel()

	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	ansi := NewANSIStyle(lgStyle)

	cases := []struct {
		name        string
		input       string
		width       int
		breakpoints string
	}{
		{"ShortLine_NoWrap", "hello", 20, ""},
		{"ShortLine_Wrap", "hello world foo bar", 10, ""},
		{"LongParagraph", strings.Repeat("hello world foo bar baz ", 5), 40, ""},
		{"ExistingNewlines", "line1\nline2 word line3", 15, ""},
		{"EmptyString", "", 20, ""},
		{"SingleChar", "x", 1, ""},
		{"Hyphens", "some-long-word here", 10, ""},
		{"CustomBreakpoints", "foo.bar.baz", 5, "."},
		{"WithANSI", ansi.Render("📋 flake1 ") + "nix build .#machine " + ansi.Render("(1.23s)"), 30, ""},
		{"ANSIMultiLine", ansi.Render("line1\nline2\nline3"), 40, ""},
		{"TabCharacters", "hello\tworld", 20, ""},
		{"TrailingSpaces", "hello   world   ", 20, ""},
		{"ZoneMarkerWrapped", "\x1b[1001zhello world\x1b[1001z", 8, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			expected := lipgloss.Wrap(testCase.input, testCase.width, testCase.breakpoints)
			got := Wrap(testCase.input, testCase.width, testCase.breakpoints)

			if expected != got {
				t.Errorf("Mismatch for %s:\n  expected: %q\n  got:      %q", testCase.name, expected, got)
			}
		})
	}
}
