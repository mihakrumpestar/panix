package style

import (
	"strings"
	"testing"
)

func TestNewStyle_ZeroValue(t *testing.T) {
	t.Parallel()

	s := NewStyle()

	if s.bold || s.hasBorder || s.width != 0 || s.maxWidth != 0 {
		t.Error("NewStyle() should produce zero-value style")
	}

	if s.fgPrefix != "" || s.bgPrefix != "" {
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

	s := NewStyle().Foreground(Color("#FF0000")).Background(Color("#00FF00"))

	if s.fgPrefix != "\x1b[38;2;255;0;0m" {
		t.Errorf("Foreground prefix = %q, want true-color red", s.fgPrefix)
	}

	if s.bgPrefix != "\x1b[48;2;0;255;0m" {
		t.Errorf("Background prefix = %q, want true-color green bg", s.bgPrefix)
	}
}

func TestStyle_Width(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(20)

	if s.width != 20 {
		t.Errorf("Width = %d, want 20", s.width)
	}
}

func TestStyle_MaxWidth(t *testing.T) {
	t.Parallel()

	s := NewStyle().MaxWidth(40)

	if s.maxWidth != 40 {
		t.Errorf("MaxWidth = %d, want 40", s.maxWidth)
	}
}

func TestStyle_Align(t *testing.T) {
	t.Parallel()

	s := NewStyle().Align(Center)

	if s.align != Center {
		t.Errorf("Align = %v, want Center", s.align)
	}
}

func TestStyle_Padding(t *testing.T) {
	t.Parallel()

	s := NewStyle().Padding(2, 3)

	if s.padTop != 2 || s.padBottom != 2 || s.padLeft != 3 || s.padRight != 3 {
		t.Errorf("Padding(2,3) = (top=%d, right=%d, bottom=%d, left=%d), want (2,3,2,3)",
			s.padTop, s.padRight, s.padBottom, s.padLeft)
	}
}

func TestStyle_PaddingIndividual(t *testing.T) {
	t.Parallel()

	s := NewStyle().PaddingTop(1).PaddingRight(2).PaddingBottom(3).PaddingLeft(4)

	if s.padTop != 1 || s.padRight != 2 || s.padBottom != 3 || s.padLeft != 4 {
		t.Errorf("Individual padding = (top=%d, right=%d, bottom=%d, left=%d), want (1,2,3,4)",
			s.padTop, s.padRight, s.padBottom, s.padLeft)
	}
}

func TestStyle_Border_AllSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	s := NewStyle().Border(b)

	if !s.hasBorder || !s.borderTop || !s.borderRight || !s.borderBottom || !s.borderLeft {
		t.Error("Border(b) with no sides should enable all four sides")
	}
}

func TestStyle_Border_SelectiveSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	s := NewStyle().Border(b, true, false, true, false)

	if !s.borderTop || s.borderRight || !s.borderBottom || s.borderLeft {
		t.Error("Border(b, true, false, true, false) should set top+bottom only")
	}
}

func TestStyle_GetForegroundGetBackground(t *testing.T) {
	t.Parallel()

	c := Color("#FF0000")
	s := NewStyle().Foreground(c).Background(c)

	if s.GetForeground() == nil {
		t.Error("GetForeground() = nil, want non-nil")
	}

	if s.GetBackground() == nil {
		t.Error("GetBackground() = nil, want non-nil")
	}
}

func TestStyle_String(t *testing.T) {
	t.Parallel()

	// No style set -> empty string
	if got := NewStyle().String(); got != "" {
		t.Errorf("NewStyle().String() = %q, want \"\"", got)
	}

	// Foreground only
	got := NewStyle().Foreground(Color("#FF0000")).String()
	if !strings.Contains(got, "\x1b[38;2;255;0;0m") || !strings.Contains(got, "\x1b[m") {
		t.Errorf("Foreground String() = %q, missing fg prefix or reset", got)
	}

	// Bold only
	got = NewStyle().Bold(true).String()
	if !strings.Contains(got, "\x1b[1m") || !strings.Contains(got, "\x1b[m") {
		t.Errorf("Bold String() = %q, missing bold or reset", got)
	}
}

func TestStyle_Render_Empty(t *testing.T) {
	t.Parallel()

	got := NewStyle().Render("")
	if got != "" {
		t.Errorf("Render(\"\") = %q, want \"\"", got)
	}
}

func TestStyle_Render_NoFormatting(t *testing.T) {
	t.Parallel()

	got := NewStyle().Render("hello")
	if got != "hello" {
		t.Errorf("No-style Render = %q, want \"hello\"", got)
	}
}

func TestStyle_Render_ForegroundOnly(t *testing.T) {
	t.Parallel()

	got := NewStyle().Foreground(Color("#8BE9FD")).Render("hello")
	visible := stripANSI(got)

	if visible != "hello" {
		t.Errorf("Visible content = %q, want \"hello\"", visible)
	}

	if !strings.Contains(got, "\x1b[") {
		t.Error("Missing ANSI sequences in rendered output")
	}
}

func TestStyle_Render_MultipleArgs(t *testing.T) {
	t.Parallel()

	got := NewStyle().Foreground(Color("#8BE9FD")).Render("hello", " ", "world")
	visible := stripANSI(got)

	if visible != "hello world" {
		t.Errorf("Multiple args visible = %q, want \"hello world\"", visible)
	}
}

func TestStyle_Render_MultiLineColor(t *testing.T) {
	t.Parallel()

	got := NewStyle().Foreground(Color("#8BE9FD")).Render("line1\nline2")
	lines := strings.Split(got, "\n")

	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if !strings.Contains(line, "\x1b[38;2;") {
			t.Errorf("Line %d missing fg prefix: %q", i, line)
		}

		if !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("Line %d missing reset suffix: %q", i, line)
		}
	}
}

func TestStyle_Render_WithBorder(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).Border(NormalBorder())
	got := s.Render("hi")

	if !strings.Contains(got, "┌") || !strings.Contains(got, "└") {
		t.Errorf("Border Render = %q, missing corner chars", got)
	}

	visible := stripANSI(got)
	if !strings.Contains(visible, "│") {
		t.Errorf("Border Render visible = %q, missing vertical bars", visible)
	}
}

func TestStyle_Render_BorderTopOnly(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).Border(NormalBorder(), true, false, false, false)
	got := s.Render("hi")

	if !strings.Contains(got, "┌") {
		t.Error("Missing top-left corner")
	}

	if strings.Contains(got, "└") {
		t.Error("Should not contain bottom-left corner")
	}
}

func TestStyle_Render_PaddingHorizontal(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(10).Padding(0, 2)
	got := s.Render("hi")

	visible := stripANSI(got)
	// "hi" padded left 2, right 2 = "  hi      " (width 10)
	if strings.TrimSpace(visible) != "hi" {
		t.Errorf("Visible = %q, expected hi padded", visible)
	}
}

func TestStyle_Render_PaddingVertical(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).PaddingTop(1).PaddingBottom(1)
	got := s.Render("hi")
	lines := strings.Split(got, "\n")

	if len(lines) < 3 {
		t.Errorf("Expected 3+ lines (top pad + content + bottom pad), got %d: %q", len(lines), got)
	}
}

func TestStyle_Render_AlignLeft(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(10).Align(Left)
	got := s.Render("hi")
	visible := stripANSI(got)

	if visible != "hi        " {
		t.Errorf("Left align = %q, want \"hi        \"", visible)
	}
}

func TestStyle_Render_AlignCenter(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(10).Align(Center)
	got := s.Render("hi")
	visible := stripANSI(got)

	// "hi" is 2 chars, pad=8 -> left=4, right=4
	if visible != "    hi    " {
		t.Errorf("Center align = %q, want \"    hi    \"", visible)
	}
}

func TestStyle_Render_AlignRight(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(10).Align(Right)
	got := s.Render("hi")
	visible := stripANSI(got)

	if visible != "        hi" {
		t.Errorf("Right align = %q, want \"        hi\"", visible)
	}
}

func TestStyle_Render_MaxWidth_Truncate(t *testing.T) {
	t.Parallel()

	s := NewStyle().MaxWidth(5)
	got := s.Render("hello world")
	visible := stripANSI(got)

	if CellWidth(visible) > 5 {
		t.Errorf("MaxWidth(5) visible width = %d, content = %q", CellWidth(visible), visible)
	}
}

func TestStyle_BorderForeground(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).Border(NormalBorder()).BorderForeground(Color("#FF0000"))
	got := s.Render("hi")

	if !strings.Contains(got, "\x1b[38;2;255;0;0m") {
		t.Errorf("BorderForeground Render = %q, missing border fg color", got)
	}
}

func TestStyle_BorderSideForeground(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).Border(NormalBorder()).
		BorderTopForeground(Color("#FF0000")).
		BorderRightForeground(Color("#00FF00")).
		BorderBottomForeground(Color("#0000FF")).
		BorderLeftForeground(Color("#FFFF00"))
	got := s.Render("hi")

	// Verify each side color is present in output
	for _, prefix := range []string{
		"\x1b[38;2;255;0;0m",   // red top
		"\x1b[38;2;0;255;0m",   // green right
		"\x1b[38;2;0;0;255m",   // blue bottom
		"\x1b[38;2;255;255;0m", // yellow left
	} {
		if !strings.Contains(got, prefix) {
			t.Errorf("Border side color %q not found in output: %q", prefix, got)
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

	for _, tc := range cases {
		got := truncateToWidth(tc.input, tc.maxW, false)

		if got != tc.want {
			t.Errorf("truncateToWidth(%q, %d, false) = %q, want %q", tc.input, tc.maxW, got, tc.want)
		}
	}
}

func TestTruncateToWidth_WithANSI(t *testing.T) {
	t.Parallel()

	// ANSI sequences should be preserved but not count toward width
	colored := "\x1b[38;2;255;0;0mhello\x1b[m"
	got := truncateToWidth(colored, 3, false)

	visible := stripANSI(got)
	if visible != "hel" {
		t.Errorf("truncateToWidth with ANSI visible = %q, want \"hel\"", visible)
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

	for _, tc := range cases {
		got := truncateToWidth(tc.input, tc.maxW, true)

		if got != tc.want {
			t.Errorf("truncateToWidth(%q, %d, true) = %q, want %q", tc.input, tc.maxW, got, tc.want)
		}
	}
}

func TestTruncateToWidth_EllipsisWithANSI(t *testing.T) {
	t.Parallel()

	colored := "\x1b[38;2;255;0;0mhello world\x1b[m"
	got := truncateToWidth(colored, 8, true)

	visible := stripANSI(got)
	if visible != "hello .." {
		t.Errorf("truncateToWidth with ANSI+ellipsis visible = %q, want \"hello ..\"", visible)
	}
}

func TestStyle_TruncateEllipsis(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(5).MaxWidth(5).TruncateEllipsis(true)
	got := s.Render("hello world")
	visible := stripANSI(got)

	if visible != "hel.." {
		t.Errorf("TruncateEllipsis(true) visible = %q, want \"hel..\"", visible)
	}
}

// maxLineWidth returns the maximum CellWidth across all lines in s.
func maxLineWidth(s string) int {
	maxW := 0
	for line := range strings.SplitSeq(s, "\n") {
		if w := CellWidth(line); w > maxW {
			maxW = w
		}
	}

	return maxW
}

func TestWidthWithMaxWidth_ContentClipped(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(20).MaxWidth(10)
	got := s.Render("hello world and more")

	if w := maxLineWidth(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) produced width %d, want <= 10: %q", w, stripANSI(got))
	}
}

func TestWidthWithMaxWidth_ShortContentNotOverPadded(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(30).MaxWidth(10)
	got := s.Render("hi")

	if w := maxLineWidth(got); w > 10 {
		t.Errorf("Width(30).MaxWidth(10) on short content produced width %d, want <= 10: %q", w, stripANSI(got))
	}
}

func TestWidthWithMaxWidth_WithBorder(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(20).MaxWidth(10).Border(RoundedBorder())
	got := s.Render("hello world and more")

	if w := maxLineWidth(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) with border produced width %d, want <= 10: %q", w, stripANSI(got))
	}
}

func TestWidthWithMaxWidth_WithPadding(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(20).MaxWidth(10).Padding(0, 1)
	got := s.Render("hello world and more")

	if w := maxLineWidth(got); w > 10 {
		t.Errorf("Width(20).MaxWidth(10) with padding produced width %d, want <= 10: %q", w, stripANSI(got))
	}
}

func TestWidthWithMaxWidth_EqualValues(t *testing.T) {
	t.Parallel()

	s := NewStyle().Width(10).MaxWidth(10)
	got := s.Render("hi")

	if w := maxLineWidth(got); w != 10 {
		t.Errorf("Width(10).MaxWidth(10) on short content produced width %d, want 10: %q", w, stripANSI(got))
	}
}
