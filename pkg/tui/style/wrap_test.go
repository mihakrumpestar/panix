package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
)

//nolint:funlen
func TestWrap_Equivalence(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := newANSIStyle(sty)

	ansiInput := func(s string) string {
		b := buffer.NewLinesBuf()
		ansi.render(b, [][]byte{[]byte(s)})
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

			contentBuf := buffer.NewLinesBuf()
			for _, l := range inputLines {
				contentBuf.WriteLine([]byte(l))
			}

			buf := buffer.NewLinesBuf()
			WrapBuf(buf, contentBuf, testCase.width, testCase.breakpoints)

			gotStrs := make([]string, buf.Len())
			for i := range buf.Len() {
				gotStrs[i] = string(buf.Line(i))
			}

			buf.Release()
			contentBuf.Release()

			got := strings.Join(gotStrs, "\n")

			assert.Equal(t, expected, got, "Mismatch for %s", testCase.name)
		})
	}
}

// TestWrap_BreakpointOverflow verifies that a breakpoint character (like '-')
// at the end of a line does not cause the wrapped line to exceed the width
// limit. Before the fix, handleBreakpoint accumulated the breakpoint without
// triggering a newline, and flushRemaining flushed it into lineBuf, producing
// a line wider than the limit.
func TestWrap_BreakpointOverflow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		width int
	}{
		// Word fills line to exactly limit, then a '-' follows with no
		// subsequent word rune to trigger a newline.
		{"WordAtLimitThenBreakpoint", "foo-", 3},
		// Word fills line, breakpoint, then a space and more content.
		{"WordAtLimitBreakpointSpace", "foo- bar", 3},
		// Simulates the kexec-style line: long path, 3 spaces, then --flag
		{"PathSpacesThenDoubleDash", "abcdefabc   --kexec", 10},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := wrapStr(tt.input, tt.width)

			for i, line := range strings.Split(got, "\n") {
				visible := string(StripANSI([]byte(line)))
				w := CellWidth([]byte(visible))
				assert.LessOrEqualf(t, w, tt.width,
					"wrapped line %d width=%d exceeds limit=%d: %q",
					i, w, tt.width, visible)
			}
		})
	}
}

func wrapStr(input string, width int) string {
	lines := strings.Split(input, "\n")

	contentBuf := buffer.NewLinesBuf()
	for _, l := range lines {
		contentBuf.WriteLine([]byte(l))
	}

	buf := buffer.NewLinesBuf()
	WrapBuf(buf, contentBuf, width, "")

	result := make([]string, buf.Len())
	for i := range buf.Len() {
		result[i] = string(buf.Line(i))
	}

	buf.Release()
	contentBuf.Release()

	return strings.Join(result, "\n")
}

//nolint:funlen
func TestWrap_StyleCarryOver(t *testing.T) {
	t.Parallel()

	red := "\x1b[31m"
	bold := "\x1b[1m"

	tests := []struct {
		name  string
		input string
		width int
		check func(t *testing.T, got string)
	}{
		{
			name:  "ColoredTextCarriesOver",
			input: red + "red text that is long enough to wrap" + "\x1b[m",
			width: 20,
			check: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, red, "expected red style to carry over")
			},
		},
		{
			name:  "ResetBeforeWrapNoCarry",
			input: red + "short" + "\x1b[m" + " plain text that is long enough to wrap",
			width: 20,
		},
		{
			name:  "BoldCarriesOver",
			input: bold + "bold text that is quite long and wraps" + "\x1b[m",
			width: 15,
		},
		{
			name:  "MultipleStylesCarryOver",
			input: bold + red + "styled text that is quite long and will wrap" + "\x1b[m",
			width: 15,
			check: func(t *testing.T, got string) {
				t.Helper()

				gotLines := strings.Split(got, "\n")
				for i, line := range gotLines {
					if i > 0 {
						assert.True(t, strings.HasPrefix(line, bold) || strings.HasPrefix(line, red),
							"line %d missing carry-over style: %q", i, line)
					}
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := wrapStr(testCase.input, testCase.width)
			expected := lipgloss.Wrap(testCase.input, testCase.width, "")

			gotVisible := string(StripANSI([]byte(got)))
			expectedVisible := string(StripANSI([]byte(expected)))

			assert.Equal(t, expectedVisible, gotVisible, "visible text mismatch")

			if testCase.check != nil {
				testCase.check(t, got)
			}
		})
	}
}
