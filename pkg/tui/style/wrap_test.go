package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func TestWrap_Equivalence(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := NewANSIStyle(sty)

	ansiInput := func(s string) string {
		b := buffer.NewLinesBuf()
		ansi.Render(b, [][]byte{[]byte(s)})
		line := string(b.Line(0))
		b.Release()

		return line
	}

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
		{"WithANSI", ansiInput("📋 flake1 ") + "nix build .#machine " + ansiInput("(1.23s)"), 30, ""},
		{"ANSIMultiLine", ansiInput("line1") + "\n" + ansiInput("line2") + "\n" + ansiInput("line3"), 40, ""},
		{"TabCharacters", "hello\tworld", 20, ""},
		{"TrailingSpaces", "hello   world   ", 20, ""},
		{"ZoneMarkerWrapped", "\x1b[1001zhello world\x1b[1001z", 8, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			expected := lipgloss.Wrap(testCase.input, testCase.width, testCase.breakpoints)

			inputLines := strings.Split(testCase.input, "\n")

			inputBytes := make([][]byte, len(inputLines))
			for i, l := range inputLines {
				inputBytes[i] = []byte(l)
			}

			buf := buffer.NewLinesBuf()
			Wrap(buf, inputBytes, testCase.width, testCase.breakpoints)

			gotStrs := make([]string, buf.Len())
			for i := range buf.Len() {
				gotStrs[i] = string(buf.Line(i))
			}

			buf.Release()

			got := strings.Join(gotStrs, "\n")

			if expected != got {
				t.Errorf("Mismatch for %s:\n  expected: %q\n  got:      %q", testCase.name, expected, got)
			}
		})
	}
}

func TestWrap_StyleCarryOver(t *testing.T) {
	t.Parallel()

	red := "\x1b[31m"
	green := "\x1b[32m"
	bold := "\x1b[1m"
	reset := "\x1b[m"

	wrapStr := func(input string, width int) string {
		lines := strings.Split(input, "\n")

		inputBytes := make([][]byte, len(lines))
		for i, l := range lines {
			inputBytes[i] = []byte(l)
		}

		buf := buffer.NewLinesBuf()
		Wrap(buf, inputBytes, width, "")

		result := make([]string, buf.Len())
		for i := range buf.Len() {
			result[i] = string(buf.Line(i))
		}

		buf.Release()

		return strings.Join(result, "\n")
	}

	t.Run("ColoredTextCarriesOver", func(t *testing.T) {
		t.Parallel()

		// "red text that is long enough to wrap" — all red
		input := red + "red text that is long enough to wrap" + reset
		got := wrapStr(input, 20)

		// First line should end with reset (before the wrap break)
		// Second line should start with the red prefix
		if !strings.Contains(got, red) {
			t.Errorf("expected red style to carry over, got: %q", got)
		}

		// Verify lipgloss equivalence
		expected := lipgloss.Wrap(input, 20, "")
		if expected != got {
			t.Errorf("mismatch:\n  expected: %q\n  got:      %q", expected, got)
		}
	})

	t.Run("ResetBeforeWrapNoCarry", func(t *testing.T) {
		t.Parallel()

		// Style resets before the wrap point — no carry expected
		input := red + "short" + reset + " plain text that is long enough to wrap"
		got := wrapStr(input, 20)

		expected := lipgloss.Wrap(input, 20, "")
		if expected != got {
			t.Errorf("mismatch:\n  expected: %q\n  got:      %q", expected, got)
		}
	})

	t.Run("BoldCarriesOver", func(t *testing.T) {
		t.Parallel()

		input := bold + "bold text that is quite long and wraps" + reset
		got := wrapStr(input, 15)

		expected := lipgloss.Wrap(input, 15, "")
		if expected != got {
			t.Errorf("mismatch:\n  expected: %q\n  got:      %q", expected, got)
		}
	})

	t.Run("MultipleStylesCarryOver", func(t *testing.T) {
		t.Parallel()

		// Bold + red. Our wrap emits separate SGR sequences (\x1b[1m\x1b[31m)
		// while lipgloss merges them (\x1b[1;31m). Both are semantically identical.
		input := bold + red + "styled text that is quite long and will wrap" + reset
		got := wrapStr(input, 15)

		expected := lipgloss.Wrap(input, 15, "")

		// Verify semantic equivalence: strip ANSI and compare visible text.
		gotVisible := string(StripANSI([]byte(got)))

		expectedVisible := string(StripANSI([]byte(expected)))
		if expectedVisible != gotVisible {
			t.Errorf("visible text mismatch:\n  expected: %q\n  got:      %q", expectedVisible, gotVisible)
		}

		// Verify each wrapped line (except the last) ends with reset and
		// each continuation line (except the first) starts with style.
		gotLines := strings.Split(got, "\n")
		for i, line := range gotLines {
			if i > 0 && !strings.HasPrefix(line, bold) && !strings.HasPrefix(line, red) {
				t.Errorf("line %d missing carry-over style: %q", i, line)
			}
		}
	})

	t.Run("StyleChangeInMiddleOfLine", func(t *testing.T) {
		t.Parallel()

		// Red text, then green text — if wrap happens during green part,
		// green should carry, not red.
		input := red + "red part " + reset + green + "green part that is quite long" + reset
		got := wrapStr(input, 20)

		expected := lipgloss.Wrap(input, 20, "")
		if expected != got {
			t.Errorf("mismatch:\n  expected: %q\n  got:      %q", expected, got)
		}
	})

	t.Run("NoStyleNoCarry", func(t *testing.T) {
		t.Parallel()

		input := "plain text that is long enough to wrap across lines"
		got := wrapStr(input, 20)

		expected := lipgloss.Wrap(input, 20, "")
		if expected != got {
			t.Errorf("mismatch:\n  expected: %q\n  got:      %q", expected, got)
		}
	})
}
