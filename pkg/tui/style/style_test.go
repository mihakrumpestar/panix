package style

import (
	"bytes"
	"strings"
	"testing"
)

func bytesJoinLines(lines [][]byte) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = string(l)
	}

	return strings.Join(parts, "\n")
}

func TestNewStyle_ZeroValue(t *testing.T) {
	t.Parallel()

	sty := NewStyle()

	if sty.bold || sty.hasBorder || sty.width != 0 || sty.maxWidth != 0 {
		t.Error("NewStyle() should produce zero-value style")
	}

	if len(sty.fgPrefix) != 0 || len(sty.bgPrefix) != 0 {
		t.Error("NewStyle() should have no color prefixes")
	}
}

func TestStyle_ChainMethodsReturnCopy(t *testing.T) {
	t.Parallel()

	original := NewStyle()
	modified := original.Bold(true)

	if original.bold {
		t.Error("Bold(true) mutated original style — methods must return copies")
	}

	if !modified.bold {
		t.Error("Bold(true) did not set bold on returned copy")
	}
}

func TestStyle_ForegroundAndBackground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#FF0000")).Background(Color("#00FF00"))

	fgExpected := []byte("\x1b[38;2;255;0;0m")
	if !bytes.Equal(sty.fgPrefix, fgExpected) {
		t.Errorf("Foreground prefix = %q, want true-color red", sty.fgPrefix)
	}

	bgExpected := []byte("\x1b[48;2;0;255;0m")
	if !bytes.Equal(sty.bgPrefix, bgExpected) {
		t.Errorf("Background prefix = %q, want true-color green bg", sty.bgPrefix)
	}
}

func TestStyle_Width(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20)

	if sty.width != 20 {
		t.Errorf("Width = %d, want 20", sty.width)
	}
}

func TestStyle_MaxWidth(t *testing.T) {
	t.Parallel()

	sty := NewStyle().MaxWidth(40)

	if sty.maxWidth != 40 {
		t.Errorf("MaxWidth = %d, want 40", sty.maxWidth)
	}
}

func TestStyle_Align(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Align(Center)

	if sty.align != Center {
		t.Errorf("Align = %v, want Center", sty.align)
	}
}

func TestStyle_Padding(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Padding(2, 3)

	if sty.padTop != 2 || sty.padBottom != 2 || sty.padLeft != 3 || sty.padRight != 3 {
		t.Errorf("Padding(2,3) = (top=%d, right=%d, bottom=%d, left=%d), want (2,3,2,3)",
			sty.padTop, sty.padRight, sty.padBottom, sty.padLeft)
	}
}

func TestStyle_PaddingIndividual(t *testing.T) {
	t.Parallel()

	sty := NewStyle().PaddingTop(1).PaddingRight(2).PaddingBottom(3).PaddingLeft(4)

	if sty.padTop != 1 || sty.padRight != 2 || sty.padBottom != 3 || sty.padLeft != 4 {
		t.Errorf("Individual padding = (top=%d, right=%d, bottom=%d, left=%d), want (1,2,3,4)",
			sty.padTop, sty.padRight, sty.padBottom, sty.padLeft)
	}
}

func TestStyle_Border_AllSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	sty := NewStyle().Border(b)

	if !sty.hasBorder || !sty.borderTop || !sty.borderRight || !sty.borderBottom || !sty.borderLeft {
		t.Error("Border(b) with no sides should enable all four sides")
	}
}

func TestStyle_Border_SelectiveSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	sty := NewStyle().Border(b, true, false, true, false)

	if !sty.borderTop || sty.borderRight || !sty.borderBottom || sty.borderLeft {
		t.Error("Border(b, true, false, true, false) should set top+bottom only")
	}
}

func TestStyle_GetForegroundGetBackground(t *testing.T) {
	t.Parallel()

	c := Color("#FF0000")
	sty := NewStyle().Foreground(c).Background(c)

	if sty.GetForeground() == "" {
		t.Error("GetForeground() = \"\", want non-empty")
	}

	if sty.GetBackground() == "" {
		t.Error("GetBackground() = \"\", want non-empty")
	}
}

func TestStyle_Bytes(t *testing.T) {
	t.Parallel()

	// No style set -> nil
	got := NewStyle().Bytes()
	if len(got) != 0 {
		t.Errorf("NewStyle().Bytes() = %q, want empty", got)
	}

	// Foreground only
	got = NewStyle().Foreground(Color("#FF0000")).Bytes()
	if !bytes.Contains(got, []byte("\x1b[38;2;255;0;0m")) || !bytes.Contains(got, ansiReset) {
		t.Errorf("Foreground Bytes() = %q, missing fg prefix or reset", got)
	}

	// Bold only
	got = NewStyle().Bold(true).Bytes()
	if !bytes.Contains(got, ansiBold) || !bytes.Contains(got, ansiReset) {
		t.Errorf("Bold Bytes() = %q, missing bold or reset", got)
	}
}

func TestStyle_Render_Empty(t *testing.T) {
	t.Parallel()

	got := NewStyle().Render(nil)
	if len(got) != 0 {
		t.Errorf("Render(nil) = %v, want empty", got)
	}
}

func TestStyle_Render_NoFormatting(t *testing.T) {
	t.Parallel()

	got := NewStyle().Render([][]byte{[]byte("hello")})
	if len(got) != 1 || string(got[0]) != "hello" {
		t.Errorf("No-style Render = %q, want \"hello\"", got)
	}
}

func TestStyle_Render_ForegroundOnly(t *testing.T) {
	t.Parallel()

	got := NewStyle().Foreground(Color("#8BE9FD")).Render([][]byte{[]byte("hello")})
	visible := StripANSI(got[0])

	if string(visible) != "hello" {
		t.Errorf("Visible content = %q, want \"hello\"", visible)
	}

	if !bytes.Contains(got[0], []byte("\x1b[")) {
		t.Error("Missing ANSI sequences in rendered output")
	}
}

func TestStyle_Render_MultiLineColor(t *testing.T) {
	t.Parallel()

	got := NewStyle().Foreground(Color("#8BE9FD")).Render([][]byte{[]byte("line1\nline2")})
	// Render takes lines, so "line1\nline2" is one input line
	// The color should be applied to each output line
	for lineIdx, line := range got {
		if !bytes.Contains(line, []byte("\x1b[38;2;")) {
			t.Errorf("Line %d missing fg prefix: %q", lineIdx, line)
		}
	}
}

func TestStyle_Render_WithBorder(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder())
	got := sty.Render([][]byte{[]byte("hi")})

	gotStr := bytesJoinLines(got)
	if !strings.Contains(gotStr, "┌") || !strings.Contains(gotStr, "└") {
		t.Errorf("Border Render = %q, missing corner chars", gotStr)
	}

	visible := StripANSI([]byte(gotStr))
	if !bytes.Contains(visible, []byte("│")) {
		t.Errorf("Border Render visible = %q, missing vertical bars", visible)
	}
}

func TestStyle_Render_BorderTopOnly(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder(), true, false, false, false)
	got := sty.Render([][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	if !strings.Contains(gotStr, "┌") {
		t.Error("Missing top-left corner")
	}

	if strings.Contains(gotStr, "└") {
		t.Error("Should not contain bottom-left corner")
	}
}

func TestStyle_Render_PaddingHorizontal(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Padding(0, 2)
	got := sty.Render([][]byte{[]byte("hi")})

	visible := StripANSI(got[0])
	// "hi" padded left 2, right 2 = "  hi      " (width 10)
	if strings.TrimSpace(string(visible)) != "hi" {
		t.Errorf("Visible = %q, expected hi padded", visible)
	}
}

func TestStyle_Render_PaddingVertical(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).PaddingTop(1).PaddingBottom(1)
	got := sty.Render([][]byte{[]byte("hi")})

	if len(got) < 3 {
		t.Errorf("Expected 3+ lines (top pad + content + bottom pad), got %d: %q", len(got), got)
	}
}

func TestStyle_Render_AlignLeft(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Left)
	got := sty.Render([][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	if string(visible) != "hi        " {
		t.Errorf("Left align = %q, want \"hi        \"", visible)
	}
}

func TestStyle_Render_AlignCenter(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Center)
	got := sty.Render([][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	// "hi" is 2 chars, pad=8 -> left=4, right=4
	if string(visible) != "    hi    " {
		t.Errorf("Center align = %q, want \"    hi    \"", visible)
	}
}

func TestStyle_Render_AlignRight(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Right)
	got := sty.Render([][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	if string(visible) != "        hi" {
		t.Errorf("Right align = %q, want \"        hi\"", visible)
	}
}

func TestStyle_Render_MaxWidth_Truncate(t *testing.T) {
	t.Parallel()

	sty := NewStyle().MaxWidth(5)
	got := sty.Render([][]byte{[]byte("hello world")})
	visible := StripANSI(got[0])

	if CellWidth(visible) > 5 {
		t.Errorf("MaxWidth(5) visible width = %d, content = %q", CellWidth(visible), visible)
	}
}

func TestStyle_BorderForeground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder()).BorderForeground(Color("#FF0000"))
	got := sty.Render([][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	if !strings.Contains(gotStr, "\x1b[38;2;255;0;0m") {
		t.Errorf("BorderForeground Render = %q, missing border fg color", gotStr)
	}
}

func TestStyle_BorderSideForeground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder()).
		BorderTopForeground(Color("#FF0000")).
		BorderRightForeground(Color("#00FF00")).
		BorderBottomForeground(Color("#0000FF")).
		BorderLeftForeground(Color("#FFFF00"))
	got := sty.Render([][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	// Verify each side color is present in output
	for _, prefix := range []string{
		"\x1b[38;2;255;0;0m",   // red top
		"\x1b[38;2;0;255;0m",   // green right
		"\x1b[38;2;0;0;255m",   // blue bottom
		"\x1b[38;2;255;255;0m", // yellow left
	} {
		if !strings.Contains(gotStr, prefix) {
			t.Errorf("Border side color %q not found in output: %q", prefix, gotStr)
		}
	}
}

func TestTruncateToWidth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		maxW  int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"hello", 5, "hello"},
		{"hello", 0, ""},
		{"", 5, ""},
	}

	for _, testCase := range cases {
		got := truncateToWidth([]byte(testCase.input), testCase.maxW, false)

		if string(got) != testCase.want {
			t.Errorf("truncateToWidth(%q, %d, false) = %q, want %q", testCase.input, testCase.maxW, got, testCase.want)
		}
	}
}

func TestTruncateToWidth_WithANSI(t *testing.T) {
	t.Parallel()

	// ANSI sequences should be preserved but not count toward width
	colored := []byte("\x1b[38;2;255;0;0mhello\x1b[m")
	got := truncateToWidth(colored, 3, false)

	visible := StripANSI(got)
	if string(visible) != "hel" {
		t.Errorf("truncateToWidth with ANSI visible = %q, want \"hel\"", visible)
	}
}

func TestTruncateToWidth_Emoji(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		maxW  int
		want  string
	}{
		{"emoji fits", "📁test", 6, "📁test"},
		{"truncate after emoji", "📁test", 5, "📁tes"},
		{"truncate inside emoji", "📁test", 1, ""},
		{"emoji at end", "hi📁", 4, "hi📁"},
		{"emoji at end truncate", "hi📁", 3, "hi"},
		{"multiple emoji", "📁📦💻", 6, "📁📦💻"},
		{"multiple emoji truncate", "📁📦💻", 4, "📁📦"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := truncateToWidth([]byte(testCase.input), testCase.maxW, false)
			if string(got) != testCase.want {
				t.Errorf("truncateToWidth(%q, %d, false) = %q, want %q", testCase.input, testCase.maxW, got, testCase.want)
			}
		})
	}
}

func TestTruncateToWidth_EmojiEllipsis(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		maxW  int
		want  string
	}{
		{"emoji fits", "📁 FLAKE", 8, "📁 FLAKE"},
		{"emoji truncate with ellipsis", "📁 CONFIGURATION", 8, "📁 CON.."},
		{"emoji too wide for ellipsis", "📁 CONFIGURATION", 2, "📁"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := truncateToWidth([]byte(testCase.input), testCase.maxW, true)
			if string(got) != testCase.want {
				t.Errorf("truncateToWidth(%q, %d, true) = %q, want %q", testCase.input, testCase.maxW, got, testCase.want)
			}
		})
	}
}

func TestTruncateToWidth_Ellipsis(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		maxW  int
		want  string
	}{
		{"hello world", 8, "hello .."},
		{"hello", 5, "hello"},
		{"hello", 3, "h.."},
		{"abcdef", 4, "ab.."},
		{"hi", 4, "hi"},
		{"", 5, ""},
	}

	for _, testCase := range cases {
		got := truncateToWidth([]byte(testCase.input), testCase.maxW, true)

		if string(got) != testCase.want {
			t.Errorf("truncateToWidth(%q, %d, true) = %q, want %q", testCase.input, testCase.maxW, got, testCase.want)
		}
	}
}

func TestTruncateToWidth_EllipsisWithANSI(t *testing.T) {
	t.Parallel()

	colored := []byte("\x1b[38;2;255;0;0mhello world\x1b[m")
	got := truncateToWidth(colored, 8, true)

	visible := StripANSI(got)
	if string(visible) != "hello .." {
		t.Errorf("truncateToWidth with ANSI+ellipsis visible = %q, want \"hello ..\"", visible)
	}
}

func TestStyle_TruncateEllipsis(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).MaxWidth(5).TruncateEllipsis(true)
	got := sty.Render([][]byte{[]byte("hello world")})
	visible := StripANSI(got[0])

	if string(visible) != "hel.." {
		t.Errorf("TruncateEllipsis(true) visible = %q, want \"hel..\"", visible)
	}
}

// maxLineWidthBytes returns the maximum CellWidth across all lines.
func maxLineWidthBytes(lines [][]byte) int {
	maxW := 0
	for _, line := range lines {
		if w := CellWidth(line); w > maxW {
			maxW = w
		}
	}

	return maxW
}

func TestWidthWithMaxWidth_ContentClipped(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20).MaxWidth(10)
	got := sty.Render([][]byte{[]byte("hello world and more")})

	if w := maxLineWidthBytes(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) produced width %d, want <= 10: %q", w, StripANSI(got[0]))
	}
}

func TestWidthWithMaxWidth_ShortContentNotOverPadded(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(30).MaxWidth(10)
	got := sty.Render([][]byte{[]byte("hi")})

	if w := maxLineWidthBytes(got); w > 10 {
		t.Errorf("Width(30).MaxWidth(10) on short content produced width %d, want <= 10: %q", w, StripANSI(got[0]))
	}
}

func TestWidthWithMaxWidth_WithBorder(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20).MaxWidth(10).Border(RoundedBorder())
	got := sty.Render([][]byte{[]byte("hello world and more")})

	if w := maxLineWidthBytes(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) with border produced width %d, want <= 10: %q", w, bytesJoinLines(got))
	}
}

func TestWidthWithMaxWidth_WithPadding(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20).MaxWidth(10).Padding(0, 1)
	got := sty.Render([][]byte{[]byte("hello world and more")})

	if w := maxLineWidthBytes(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) with padding produced width %d, want <= 10: %q", w, StripANSI(got[0]))
	}
}

func TestWidthWithMaxWidth_EqualValues(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).MaxWidth(10)
	got := sty.Render([][]byte{[]byte("hi")})

	if w := maxLineWidthBytes(got); w != 10 {
		t.Errorf("Width(10).MaxWidth(10) on short content produced width %d, want 10: %q", w, StripANSI(got[0]))
	}
}
