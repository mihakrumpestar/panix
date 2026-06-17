package style

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
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

	assert.False(t, sty.bold || sty.hasBorder || sty.width != 0 || sty.maxWidth != 0, "NewStyle() should produce zero-value style")
	assert.Empty(t, sty.fgPrefix, "NewStyle() should have no color prefixes")
	assert.Empty(t, sty.bgPrefix, "NewStyle() should have no color prefixes")
}

func TestStyle_ChainMethodsReturnCopy(t *testing.T) {
	t.Parallel()

	original := NewStyle()
	modified := original.Bold(true)

	assert.False(t, original.bold, "Bold(true) mutated original style — methods must return copies")
	assert.True(t, modified.bold, "Bold(true) did not set bold on returned copy")
}

func TestStyle_ForegroundAndBackground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#FF0000")).Background(Color("#00FF00"))

	assert.Equal(t, []byte("\x1b[38;2;255;0;0m"), sty.fgPrefix, "Foreground prefix")
	assert.Equal(t, []byte("\x1b[48;2;0;255;0m"), sty.bgPrefix, "Background prefix")
}

func TestStyle_Width(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20)

	assert.Equal(t, 20, sty.width)
}

func TestStyle_MaxWidth(t *testing.T) {
	t.Parallel()

	sty := NewStyle().MaxWidth(40)

	assert.Equal(t, 40, sty.maxWidth)
}

func TestStyle_Align(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Align(Center)

	assert.InDelta(t, float64(Center), float64(sty.align), 0.01)
}

func TestStyle_Padding(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Padding(2, 3)

	assert.Equal(t, 2, sty.padTop)
	assert.Equal(t, 2, sty.padBottom)
	assert.Equal(t, 3, sty.padLeft)
	assert.Equal(t, 3, sty.padRight)
}

func TestStyle_PaddingIndividual(t *testing.T) {
	t.Parallel()

	sty := NewStyle().PaddingTop(1).PaddingRight(2).PaddingBottom(3).PaddingLeft(4)

	assert.Equal(t, 1, sty.padTop)
	assert.Equal(t, 2, sty.padRight)
	assert.Equal(t, 3, sty.padBottom)
	assert.Equal(t, 4, sty.padLeft)
}

func TestStyle_Border_AllSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	sty := NewStyle().Border(b)

	assert.True(t, sty.hasBorder && sty.borderTop && sty.borderRight && sty.borderBottom && sty.borderLeft,
		"Border(b) with no sides should enable all four sides")
}

func TestStyle_Border_SelectiveSides(t *testing.T) {
	t.Parallel()

	b := NormalBorder()
	sty := NewStyle().Border(b, true, false, true, false)

	assert.True(t, sty.borderTop && !sty.borderRight && sty.borderBottom && !sty.borderLeft,
		"Border(b, true, false, true, false) should set top+bottom only")
}

func TestStyle_GetForegroundGetBackground(t *testing.T) {
	t.Parallel()

	c := Color("#FF0000")
	sty := NewStyle().Foreground(c).Background(c)

	assert.NotEmpty(t, sty.GetForeground())
	assert.NotEmpty(t, sty.GetBackground())
}

func TestStyle_Bytes(t *testing.T) {
	t.Parallel()

	assert.Empty(t, NewStyle().Bytes(), "NewStyle().Bytes() should be empty")

	got := NewStyle().Foreground(Color("#FF0000")).Bytes()
	assert.True(t, bytes.Contains(got, []byte("\x1b[38;2;255;0;0m")), "missing fg prefix")
	assert.True(t, bytes.Contains(got, ansiReset), "missing reset")

	got = NewStyle().Bold(true).Bytes()
	assert.True(t, bytes.Contains(got, ansiBold), "missing bold")
	assert.True(t, bytes.Contains(got, ansiReset), "missing reset")
}

func TestStyle_Render_Empty(t *testing.T) {
	t.Parallel()

	assert.Nil(t, renderForTest(NewStyle(), nil))
}

func TestStyle_Render_NoFormatting(t *testing.T) {
	t.Parallel()

	got := renderForTest(NewStyle(), [][]byte{[]byte("hello")})
	assert.Len(t, got, 1)
	assert.Equal(t, "hello", string(got[0]))
}

func TestStyle_Render_ForegroundOnly(t *testing.T) {
	t.Parallel()

	got := renderForTest(NewStyle().Foreground(Color("#8BE9FD")), [][]byte{[]byte("hello")})
	visible := StripANSI(got[0])

	assert.Equal(t, "hello", string(visible))
	assert.True(t, bytes.Contains(got[0], []byte("\x1b[")))
}

func TestStyle_Render_MultiLineColor(t *testing.T) {
	t.Parallel()

	got := renderForTest(NewStyle().Foreground(Color("#8BE9FD")), [][]byte{[]byte("line1\nline2")})
	for lineIdx, line := range got {
		assert.True(t, bytes.Contains(line, []byte("\x1b[38;2;")), "Line %d missing fg prefix", lineIdx)
	}
}

func TestStyle_Render_WithBorder(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder())
	got := renderForTest(sty, [][]byte{[]byte("hi")})

	gotStr := bytesJoinLines(got)
	assert.Contains(t, gotStr, "┌")
	assert.Contains(t, gotStr, "└")

	visible := StripANSI([]byte(gotStr))
	assert.True(t, bytes.Contains(visible, []byte("│")))
}

func TestStyle_Render_BorderTopOnly(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder(), true, false, false, false)
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	assert.Contains(t, gotStr, "┌", "Missing top-left corner")
	assert.NotContains(t, gotStr, "└", "Should not contain bottom-left corner")
}

func TestStyle_Render_PaddingHorizontal(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Padding(0, 2)
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	assert.Equal(t, "hi", strings.TrimSpace(string(visible)))
}

func TestStyle_Render_PaddingVertical(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).PaddingTop(1).PaddingBottom(1)
	got := renderForTest(sty, [][]byte{[]byte("hi")})

	assert.GreaterOrEqual(t, len(got), 3, "Expected 3+ lines")
}

func TestStyle_Render_AlignLeft(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Left)
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	assert.Equal(t, "hi        ", string(visible))
}

func TestStyle_Render_AlignCenter(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Center)
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	assert.Equal(t, "    hi    ", string(visible))
}

func TestStyle_Render_AlignRight(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).Align(Right)
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	visible := StripANSI(got[0])

	assert.Equal(t, "        hi", string(visible))
}

func TestStyle_Render_MaxWidth_Truncate(t *testing.T) {
	t.Parallel()

	sty := NewStyle().MaxWidth(5)
	got := renderForTest(sty, [][]byte{[]byte("hello world")})
	visible := StripANSI(got[0])

	assert.LessOrEqual(t, CellWidth(visible), 5)
}

func TestStyle_BorderForeground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder()).BorderForeground(Color("#FF0000"))
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	assert.Contains(t, gotStr, "\x1b[38;2;255;0;0m")
}

func TestStyle_BorderSideForeground(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).Border(NormalBorder()).
		BorderTopForeground(Color("#FF0000")).
		BorderRightForeground(Color("#00FF00")).
		BorderBottomForeground(Color("#0000FF")).
		BorderLeftForeground(Color("#FFFF00"))
	got := renderForTest(sty, [][]byte{[]byte("hi")})
	gotStr := bytesJoinLines(got)

	for _, prefix := range []string{
		"\x1b[38;2;255;0;0m",
		"\x1b[38;2;0;255;0m",
		"\x1b[38;2;0;0;255m",
		"\x1b[38;2;255;255;0m",
	} {
		assert.Contains(t, gotStr, prefix, "Border side color %q not found", prefix)
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
		{"hello world", 5, "hello"},
		{"", 5, ""},
		{"  spaced  ", 4, "  sp"},
	}

	for _, testCase := range cases {
		got := truncateToWidth([]byte(testCase.input), testCase.maxW, false)
		assert.Equal(t, testCase.want, string(got), "truncateToWidth(%q, %d)", testCase.input, testCase.maxW)
	}
}

func TestTruncateToWidth_WithANSI(t *testing.T) {
	t.Parallel()

	colored := []byte("\x1b[38;2;255;0;0mhello\x1b[m")
	got := truncateToWidth(colored, 3, false)
	visible := StripANSI(got)

	assert.Equal(t, "hel", string(visible))
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
			assert.Equal(t, testCase.want, string(got))
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
			assert.Equal(t, testCase.want, string(got))
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
		assert.Equal(t, testCase.want, string(got))
	}
}

func TestTruncateToWidth_EllipsisWithANSI(t *testing.T) {
	t.Parallel()

	colored := []byte("\x1b[38;2;255;0;0mhello world\x1b[m")
	got := truncateToWidth(colored, 8, true)
	visible := StripANSI(got)

	assert.Equal(t, "hello ..", string(visible))
}

func TestStyle_TruncateEllipsis(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(5).MaxWidth(5).TruncateEllipsis(true)
	got := renderForTest(sty, [][]byte{[]byte("hello world")})
	visible := StripANSI(got[0])

	assert.Equal(t, "hel..", string(visible))
}

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
	got := renderForTest(sty, [][]byte{[]byte("hello world and more")})

	assert.LessOrEqual(t, maxLineWidthBytes(got), 10)
}

func TestWidthWithMaxWidth_ShortContentNotOverPadded(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(30).MaxWidth(10)
	got := renderForTest(sty, [][]byte{[]byte("hi")})

	assert.LessOrEqual(t, maxLineWidthBytes(got), 10)
}

func TestWidthWithMaxWidth_WithBorder(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20).MaxWidth(10).Border(RoundedBorder())
	got := renderForTest(sty, [][]byte{[]byte("hello world and more")})

	assert.LessOrEqual(t, maxLineWidthBytes(got), 10)
}

func TestWidthWithMaxWidth_WithPadding(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(20).MaxWidth(10).Padding(0, 1)
	got := renderForTest(sty, [][]byte{[]byte("hello world and more")})

	assert.LessOrEqual(t, maxLineWidthBytes(got), 10)
}

func TestWidthWithMaxWidth_EqualValues(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Width(10).MaxWidth(10)
	got := renderForTest(sty, [][]byte{[]byte("hi")})

	assert.Equal(t, 10, maxLineWidthBytes(got))
}

// Helpers

func renderForTest(sty Style, content [][]byte) [][]byte {
	if content == nil && !sty.hasBorder && sty.padTop == 0 && sty.padBottom == 0 && sty.width == 0 {
		return nil
	}

	contentBuf := buffer.NewLinesBuf()
	for _, line := range content {
		contentBuf.WriteLine(line)
	}

	resultBuf := buffer.NewLinesBuf()
	sty.RenderIntoBuf(resultBuf, contentBuf)

	result := resultBuf.Lines()

	resultCopy := make([][]byte, len(result))
	for i, line := range result {
		resultCopy[i] = bytes.Clone(line)
	}

	contentBuf.Release()
	resultBuf.Release()

	return resultCopy
}
