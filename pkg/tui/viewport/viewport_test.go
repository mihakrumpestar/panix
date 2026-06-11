package viewport

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func strLines(ss ...string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}

	return out
}

func splitLines(s string) [][]byte {
	if s == "" {
		return nil
	}

	return bytes.Split([]byte(s), []byte("\n"))
}

func TestNew(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(24))
	assert.Equal(t, 80, mdl.Width())
	assert.Equal(t, 24, mdl.Height())
}

func TestSetContentLines(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("a", "b", "c"))

	assert.Equal(t, 3, mdl.TotalLineCount())

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	assert.Equal(t, "a", strings.TrimSpace(lines[0]))
	assert.Equal(t, "b", strings.TrimSpace(lines[1]))
	assert.Equal(t, "c", strings.TrimSpace(lines[2]))
}

func TestSetContentLinesKeepsScroll(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.setLines(strLines("a", "b", "c", "d", "e"))
	assert.Equal(t, 2, mdl.YOffset(), "YOffset should stay 2")

	mdl.setLines(strLines("x", "y"))
	assert.Equal(t, 0, mdl.YOffset(), "YOffset should be 0 after shrink")
}

func TestView(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2", "line3", "line4", "line5"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, "line1", strings.TrimSpace(lines[0]))
}

func TestViewPaddedToWidth(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3))
	mdl.setLines(strLines("short", "a bit longer line", "x"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	for i, line := range lines {
		assert.Equal(t, 20, style.CellWidth([]byte(line)), "line %d visible width", i)
	}
}

func TestViewHeightChanges(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.Equal(t, 2, strings.Count(view, "\n"), "height 3 should have 2 newlines (3 lines)")

	mdl.SetHeight(2)

	view = buffer.LinesBufToStringForTests(mdl.Render())
	assert.Equal(t, 1, strings.Count(view, "\n"), "height 2 should have 1 newline (2 lines)")

	mdl.SetHeight(10)

	view = buffer.LinesBufToStringForTests(mdl.Render())
	assert.Equal(t, 10, strings.Count(view, "\n")+1, "height 10 should have 10 lines")
}

func TestScrollPercent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2", "line3", "line4", "line5"))

		assert.InDelta(t, float64(0), mdl.ScrollPercent(), 0.001, "ScrollPercent at top")

	mdl.GotoBottom()

	assert.InEpsilon(t, float64(1), mdl.ScrollPercent(), 0.001, "ScrollPercent at bottom")

	mdl.SetYOffset(1)

	pct := mdl.ScrollPercent()
	assert.True(t, pct > 0 && pct < 1, "ScrollPercent mid = %f, want between 0 and 1", pct)
}

func TestScrollPercentShortContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("a", "b"))

	assert.InEpsilon(t, float64(1), mdl.ScrollPercent(), 0.001, "short content ScrollPercent")
}

func TestScrollPercentEmpty(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines())

	assert.InEpsilon(t, float64(1), mdl.ScrollPercent(), 0.001, "empty ScrollPercent")
}

func TestGotoBottom(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2", "line3", "line4", "line5"))
	mdl.GotoBottom()

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	assert.Equal(t, "line3", strings.TrimSpace(lines[0]), "first visible line after GotoBottom")
	assert.Equal(t, "line5", strings.TrimSpace(lines[2]), "last visible line")
}

func TestGotoBottomShorterContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("a", "b"))
	mdl.GotoBottom()

	assert.Equal(t, 0, mdl.YOffset(), "short content GotoBottom")
}

func TestSetYOffset(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2", "line3", "line4", "line5"))
	mdl.SetYOffset(2)

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	assert.Equal(t, "line3", strings.TrimSpace(lines[0]), "first line")
}

func TestSetYOffsetClamped(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2", "line3", "line4", "line5"))

	mdl.SetYOffset(100)

	assert.Equal(t, 2, mdl.YOffset(), "YOffset after clamp")

	mdl.SetYOffset(-5)

	assert.Equal(t, 0, mdl.YOffset(), "YOffset after negative clamp")
}

func TestContentShorterThanViewport(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("line1", "line2"))

	view := buffer.LinesBufToStringForTests(mdl.Render())

	count := strings.Count(view, "\n") + 1
	assert.Equal(t, 10, count)
	assert.True(t, strings.HasPrefix(view, "line1"), "content incorrect: %q", view)
}

func TestEmptyContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines())

	assert.Equal(t, 0, mdl.TotalLineCount(), "TotalLineCount for empty")
}

func TestEmptyView(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines())

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.Empty(t, view, "empty view")
}

func TestViewNoContentYet(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.Empty(t, view, "uninitialized view")
}

func TestSetWidthPreservesContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("a", "b", "c"))

	mdl.SetWidth(40)

	assert.Equal(t, 3, mdl.TotalLineCount(), "after width change, TotalLineCount")
	assert.Equal(t, 40, mdl.Width())
}

func TestSetWidthResetsScrollIfNeeded(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.setLines(strLines("a", "b", "c", "d", "e"))

	assert.Equal(t, 2, mdl.YOffset())
}

func TestViewAtBottomWithResize(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.GotoBottom()

	mdl.SetHeight(10)

	assert.Equal(t, 2, mdl.YOffset(), "YOffset = %d, want 2 (was at bottom)", mdl.YOffset())

	mdl.SetHeight(3)
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	assert.Len(t, lines, 3)
}

func TestViewBeyondContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("line1", "line2"))
	mdl.SetYOffset(10)

	assert.Equal(t, 0, mdl.YOffset(), "offset clamped to %d, want 0", mdl.YOffset())
}

func TestViewWithANSI(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3))
	mdl.setLines(strLines("\x1b[31mred\x1b[0m", "plain", "\x1b[1mbold\x1b[0m"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	for i, line := range lines {
		visibleLen := style.CellWidth([]byte(line))
		assert.Equal(t, 20, visibleLen, "line %d visible width", i)
	}
}

func TestStringWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"\x1b[31mred\x1b[0m", 3},
		{"\x1b[1;32mgreen\x1b[0m", 5},
		{"", 0},
		{"no escapes", 10},
		{"\x1b[38;5;196mcolored\x1b[0m", 7},
		{"█", 1},
		{"│", 1},
		{"l1                 █", 20},
		{"hi       █", 10},
		{"\x1b[31m█\x1b[0m", 1},
		{"\x1b[34m│\x1b[0m", 1},
		{"📁", 2},
		{"📦💻", 4},
	}

	for _, tt := range tests {
		got := style.CellWidth([]byte(tt.input))
		assert.Equal(t, tt.want, got, "style.CellWidth(%q)", tt.input)
	}
}

func TestViewWidthZero(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(0), WithHeight(10))
	mdl.setLines(strLines("a", "b"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.Empty(t, view, "zero-width view")
}

func TestScrollDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	mdl.ScrollDown(1)

	assert.Equal(t, 1, mdl.YOffset(), "YOffset after ScrollDown(1)")

	mdl.ScrollDown(10)

	assert.Equal(t, 2, mdl.YOffset(), "YOffset after ScrollDown(10) (clamped)")
}

func TestScrollUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.ScrollUp(1)

	assert.Equal(t, 1, mdl.YOffset(), "YOffset after ScrollUp(1)")

	mdl.ScrollUp(10)

	assert.Equal(t, 0, mdl.YOffset(), "YOffset after ScrollUp(10) (clamped)")
}

func TestPageDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"))

	mdl.PageDown()

	assert.Equal(t, 3, mdl.YOffset(), "YOffset after PageDown")
}

func TestPageUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"))
	mdl.SetYOffset(5)

	mdl.PageUp()

	assert.Equal(t, 2, mdl.YOffset(), "YOffset after PageUp")
}

func TestHalfPageDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"))

	mdl.HalfPageDown()

	assert.Equal(t, 5, mdl.YOffset(), "YOffset after HalfPageDown")
}

func TestHalfPageUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"))
	mdl.SetYOffset(5)

	mdl.HalfPageUp()

	assert.Equal(t, 0, mdl.YOffset(), "YOffset after HalfPageUp")
}

func TestAtTopAtBottom(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	assert.True(t, mdl.AtTop(), "should be at top initially")

	mdl.GotoBottom()

	assert.True(t, mdl.AtBottom(), "should be at bottom after GotoBottom")

	mdl.SetYOffset(1)

	assert.False(t, mdl.AtTop() || mdl.AtBottom(), "should be neither top nor bottom in the middle")
}

// ---- Comprehensive scrollbar tests ----

func TestScrollbarVisibleWidthWithOverflow(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}
}

func TestScrollbarContentWidthSmallerThanTotal(t *testing.T) {
	t.Parallel()

	vpWidth := 10
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("hi", "there", "x", "overflow1", "overflow2"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, vpWidth, visibleW, "line %d", idx)

		contentPart := stripScrollbar(line)
		contentVisibleW := style.CellWidth([]byte(contentPart))
		assert.Equal(t, vpWidth-scrollbarColWidth, contentVisibleW,
			"line %d: content visible width: content=%q full=%q", idx, contentPart, line)
	}
}

func TestScrollbarHiddenContentFits(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(10), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d (no scrollbar)", idx)
		assert.False(t, strings.Contains(line, "█") || strings.Contains(line, "│"),
			"line %d: scrollbar chars should not appear when content fits: %q", idx, line)
	}
}

func TestScrollbarNoScrollbarOption(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(3))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}
}

func TestScrollbarThumbAtTop(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 3
	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbLine := -1

	for idx, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbLine = idx

			break
		}
	}

	assert.Equal(t, 0, thumbLine, "at top, thumb should be at line 0, got line %d", thumbLine)
}

func TestScrollbarThumbAtBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 3
	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))
	mdl.GotoBottom()

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbLine := -1

	for idx, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbLine = idx
		}
	}

	assert.Equal(t, vpHeight-1, thumbLine, "at bottom, thumb should be at line %d, got line %d", vpHeight-1, thumbLine)
}

func TestScrollbarThumbSizeProportional(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbCount := 0

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbCount++
		}
	}

	assert.Equal(t, 1, thumbCount, "thumb count (9 lines, height 3)")

	mdl2 := New(WithWidth(20), WithHeight(10), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	longContent := make([][]byte, 30)
	for idx := range longContent {
		longContent[idx] = []byte("line")
	}

	mdl2.setLines(longContent)

	view = buffer.LinesBufToStringForTests(mdl2.Render())
	viewLines = strings.Split(view, "\n")

	thumbCount = 0

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbCount++
		}
	}

	assert.Equal(t, 3, thumbCount, "thumb count (30 lines, height 10)")
}

func TestScrollbarWithANSIStyles(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color("1"), style.Color("4")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, 20, visibleW, "line %d: visible width (styled scrollbar)", idx)
	}

	assert.True(t, strings.Contains(viewLines[0], "█") && strings.Contains(viewLines[0], "\x1b["),
		"line 0 should have styled red thumb: %q", viewLines[0])
	assert.True(t, strings.Contains(viewLines[1], "│") && strings.Contains(viewLines[1], "\x1b["),
		"line 1 should have styled blue track: %q", viewLines[1])
}

func TestScrollbarFillLinesAlsoGetScrollbar(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(10), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	require.Len(t, viewLines, 5, "expected 5 lines")

	for idx, line := range viewLines {
		hasScrollbar := strings.Contains(line, "█") || strings.Contains(line, "│")
		assert.True(t, hasScrollbar, "line %d: missing scrollbar in fill line: %q", idx, line)

		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, 10, visibleW, "line %d", idx)
	}
}

func TestScrollbarNoScrollbarWhenContentFitsExactly(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())

	assert.False(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"scrollbar should not appear when content fits exactly: %q", view)

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, 20, style.CellWidth([]byte(line)), "line %d (no scrollbar)", idx)
	}
}

func TestScrollbarContentWidth(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("a", "b"))

	assert.Equal(t, 20, mdl.ContentWidth(), "ContentWidth no overflow")

	mdl2 := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl2.setLines(strLines("a", "b", "c", "d", "e"))

	assert.Equal(t, 18, mdl2.ContentWidth(), "ContentWidth with overflow")

	mdl3 := New(WithWidth(20), WithHeight(3))
	assert.Equal(t, 20, mdl3.ContentWidth(), "ContentWidth without scrollbar")
}

func TestScrollbarExactLineComposition(t *testing.T) {
	t.Parallel()

	vpWidth := 10
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("hi", "there", "x", "y", "z"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	contentW := vpWidth - scrollbarColWidth

	for idx, line := range viewLines {
		scrollbarStart := strings.LastIndex(line, " ")
		if !assert.GreaterOrEqual(t, scrollbarStart, 0, "line %d: no space before scrollbar: %q", idx, line) {
			continue
		}

		contentPart := line[:scrollbarStart]
		scrollbarPart := line[scrollbarStart:]

		contentVisibleW := style.CellWidth([]byte(contentPart))
		assert.Equal(t, contentW, contentVisibleW, "line %d: content part visible width: content=%q", idx, contentPart)

		assert.True(t, scrollbarPart == " █" || scrollbarPart == " │",
			"line %d: unexpected scrollbar part %q", idx, scrollbarPart)
	}
}

func TestScrollbarScrollingMovesThumb(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("line")
	}

	mdl.setLines(content)

	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	assert.Contains(t, viewLines[0], "█", "at top, line 0 should have thumb")

	mdl.SetYOffset(5)
	view = buffer.LinesBufToStringForTests(mdl.Render())
	viewLines = strings.Split(view, "\n")

	assert.NotContains(t, viewLines[0], "█", "after scrolling, line 0 should not have thumb anymore")

	thumbFound := false

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbFound = true

			break
		}
	}

	assert.True(t, thumbFound, "thumb should be visible somewhere after scrolling")
}

// stripScrollbar removes the trailing " │" or " █" from a line,
// handling both plain and ANSI-styled scrollbar chars.
func stripScrollbar(line string) string {
	for _, suffix := range []string{" │", " █"} {
		if strings.HasSuffix(line, suffix) {
			return line[:len(line)-len(suffix)]
		}
	}

	for _, char := range []string{"█", "│"} {
		target := char + "\x1b[0m"

		idx := strings.LastIndex(line, target)
		if idx >= 0 {
			styleStart := idx
			for styleStart > 0 && line[styleStart-1] != ' ' {
				styleStart--
			}

			return line[:styleStart]
		}
	}

	return line
}

func TestBorderBasic(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(6), WithHeight(3), WithBorder(style.Color("")))
	mdl.setLines(strLines("AB"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	expected := "╭────╮\n│AB  │\n╰────╯"
	assert.Equal(t, expected, view, "bordered view")
}

func TestBorderOutputSize(t *testing.T) {
	t.Parallel()

	width := 10
	height := 4
	mdl := New(WithWidth(width), WithHeight(height), WithBorder(style.Color("")))
	mdl.setLines(strLines("line1", "line2"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(view, "\n")
	assert.Len(t, lines, height, "bordered view has %d lines, want %d", len(lines), height)

	for i, line := range lines {
		assert.Equal(t, width, style.CellWidth([]byte(line)), "line %d visible width", i)
	}
}

func TestBorderWithOverflowAndScrollbar(t *testing.T) {
	t.Parallel()

	width := 12
	height := 5
	mdl := New(WithWidth(width), WithHeight(height), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("a", "b", "c", "d", "e", "f", "g", "h"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(view, "\n")
	assert.Len(t, lines, height, "bordered+scrollbar view has %d lines, want %d", len(lines), height)

	for i, line := range lines {
		assert.Equal(t, width, style.CellWidth([]byte(line)), "line %d visible width", i)
	}
}

func TestBorderContentWidth(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(10), WithHeight(3), WithBorder(style.Color("")))
	assert.Equal(t, 8, mdl.ContentWidth(), "bordered ContentWidth")

	mdl2 := New(WithWidth(10), WithHeight(5), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl2.setLines(strLines("a", "b"))

	assert.Equal(t, 8, mdl2.ContentWidth(), "bordered+scrollbar (fits) ContentWidth")

	mdl3 := New(WithWidth(10), WithHeight(5), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl3.setLines(strLines("a", "b", "c", "d", "e", "f"))

	assert.Equal(t, 6, mdl3.ContentWidth(), "bordered+scrollbar (overflows) ContentWidth")
}

func TestBorderScrolling(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(8), WithHeight(5), WithBorder(style.Color("")))
	mdl.setLines(strLines("a", "b", "c", "d", "e", "f"))

	assert.False(t, mdl.AtBottom(), "should not be at bottom")

	mdl.ScrollDown(3)

	assert.True(t, mdl.AtBottom(), "should be at bottom after scrolling 3, yOffset=%d, maxYOffset=%d", mdl.YOffset(), mdl.linesLen-3)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	lines := strings.Split(view, "\n")

	assert.Contains(t, lines[1], "d", "expected line 1 to contain 'd', got %q", lines[1])
}

func TestBorderStyled(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(6), WithHeight(3), WithBorder(style.Color("1")))
	mdl.setLines(strLines("AB"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	assert.True(t, strings.Contains(view, "\x1b[") && strings.Contains(view, "╭"),
		"border top-left should be styled")
	assert.True(t, strings.Contains(view, "\x1b[") && strings.Contains(view, "│"),
		"border sides should be styled")
	assert.True(t, strings.Contains(view, "\x1b[") && strings.Contains(view, "╰"),
		"border bottom-left should be styled")
}

func TestBorderIsBordered(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(10), WithHeight(3), WithBorder(style.Color("")))
	assert.True(t, mdl.IsBordered(), "IsBordered should be true")

	mdl2 := New(WithWidth(10), WithHeight(3))
	assert.False(t, mdl2.IsBordered(), "IsBordered should be false")
}

func TestBorderOverheadFunc(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, BorderOverhead())
}

// ---- Scrollbar reservation and consistency tests ----

func TestScrollbarAlwaysReservesWidth(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 10

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	mdl.setLines(strLines("short", "content"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "content-fits line %d", idx)
	}

	mdl.setLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12"))
	view = buffer.LinesBufToStringForTests(mdl.Render())

	viewLines = strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "content-overflows line %d", idx)
	}
}

func TestScrollbarVisibleAtBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("line")
	}

	mdl.setLines(content)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"at top: scrollbar should be visible\n%q", view)

	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"at bottom: scrollbar should be visible\n%q", view)

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "at-bottom line %d", idx)
	}
}

func TestScrollbarVisibleAtEveryScrollPosition(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 30)
	for idx := range content {
		content[idx] = []byte("line")
	}

	mdl.setLines(content)

	for offset := 0; offset <= mdl.maxYOffset(); offset++ {
		mdl.SetYOffset(offset)

		view := buffer.LinesBufToStringForTests(mdl.Render())
		assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
			"at yOffset=%d: scrollbar should be visible\n%q", offset, view)

		viewLines := strings.Split(view, "\n")
		for idx, line := range viewLines {
			assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "yOffset=%d line %d", offset, idx)
		}
	}
}

func TestScrollbarColumnIsSpacesWhenContentFits(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 10

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("a", "b"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.False(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"when content fits, no scrollbar chars should appear\n%q", view)

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}
}

func TestBorderedScrollbarWidthConsistency(t *testing.T) {
	t.Parallel()

	vpWidth := 16
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("x")
	}

	mdl.setLines(content)

	for offset := 0; offset <= mdl.maxYOffset(); offset++ {
		mdl.SetYOffset(offset)
		view := buffer.LinesBufToStringForTests(mdl.Render())
		viewLines := strings.Split(view, "\n")

		assert.Len(t, viewLines, vpHeight, "yOffset=%d", offset)

		for idx, line := range viewLines {
			assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "yOffset=%d line %d", offset, idx)
		}

		assert.True(t, strings.HasPrefix(viewLines[0], "╭"),
			"yOffset=%d: missing top-left border", offset)
		assert.True(t, strings.HasPrefix(viewLines[vpHeight-1], "╰"),
			"yOffset=%d: missing bottom-left border", offset)
	}
}

func TestBorderedScrollbarVisibleAtBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 14
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("x")
	}

	mdl.setLines(content)

	mdl.GotoBottom()
	view := buffer.LinesBufToStringForTests(mdl.Render())

	assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"at bottom: scrollbar should be visible in bordered viewport\n%q", view)

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "at-bottom line %d", idx)
	}
}

func TestSetContentWrapsAtScrollbarWidth(t *testing.T) {
	t.Parallel()

	vpWidth := 10
	vpHeight := 3

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	_ = mdl.SetContent(splitLines("abcdefghij"))

	assert.Equal(t, 1, mdl.TotalLineCount(), "short content should not rewrap, got %d lines", mdl.TotalLineCount())

	mdl2 := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	_ = mdl2.SetContent(splitLines(strings.Repeat("abcdefghij", 5)))

	assert.GreaterOrEqual(t, mdl2.TotalLineCount(), 5, "long content should wrap at narrow width, got %d lines", mdl2.TotalLineCount())

	view := buffer.LinesBufToStringForTests(mdl2.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}
}

func TestBorderedSetContentWrapsCorrectly(t *testing.T) {
	t.Parallel()

	vpWidth := 14
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	fullContentWidth := vpWidth - 2
	narrowContentWidth := vpWidth - 2 - 2

	longContent := strings.Repeat("x", 60)
	_ = mdl.SetContent(splitLines(longContent))

	for idx := range mdl.linesLen {
		line := mdl.line(idx)
		assert.LessOrEqual(t, style.CellWidth(line), narrowContentWidth,
			"wrapped line %d: width = %d, want <= %d", idx, style.CellWidth(line), narrowContentWidth)
	}

	mdl2 := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	shortContent := strings.Repeat("x", fullContentWidth)
	_ = mdl2.SetContent(splitLines(shortContent))

	assert.LessOrEqual(t, mdl2.TotalLineCount(), 1, "short content should not wrap, got %d lines", mdl2.TotalLineCount())
}

func TestScrollbarNoScrollbarOptionStillFullWidth(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte(strings.Repeat("x", vpWidth))
	}

	mdl.setLines(content)

	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "no-scrollbar line %d", idx)
	}
}

func TestBorderedScrollbarFitsContentFitsExactly(t *testing.T) {
	t.Parallel()

	vpWidth := 12
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("line1", "line2", "line3"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	assert.Len(t, viewLines, vpHeight, "lines = %d, want %d", len(viewLines), vpHeight)

	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}

	assert.NotContains(t, view, "█",
		"scrollbar thumb should not appear when content fits exactly\n%q", view)
}

func TestBorderedScrollbarContentOverflowsByOne(t *testing.T) {
	t.Parallel()

	vpWidth := 12
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.setLines(strLines("line1", "line2", "line3", "line4"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"scrollbar should appear when content overflows by 1\n%q", view)

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", idx)
	}
}

func TestSetContentScrollbarAtTopAndBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 10
	vpHeight := 3

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	_ = mdl.SetContent(splitLines(strings.Repeat("abcdefghij", 5)))

	require.Greater(t, mdl.TotalLineCount(), vpHeight, "expected content to overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	assert.Contains(t, view, "█", "at top: scrollbar thumb should be visible\n%q", view)

	for i, line := range strings.Split(view, "\n") {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "at top line %d", i)
	}

	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "│"),
		"at bottom: scrollbar should be visible\n%q", view)

	for i, line := range strings.Split(view, "\n") {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "at bottom line %d", i)
	}
}

func TestBorderedSetContentScrollbarAtTopAndBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 14
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	_ = mdl.SetContent(splitLines(strings.Repeat("x", 50)))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	for i, line := range strings.Split(view, "\n") {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "line %d", i)
	}

	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	for i, line := range strings.Split(view, "\n") {
		assert.Equal(t, vpWidth, style.CellWidth([]byte(line)), "at bottom line %d", i)
	}

	assert.Contains(t, view, "█",
		"at bottom: scrollbar thumb should be visible\n%q", view)
}

func TestMainViewportScrollbarOnAllLines(t *testing.T) {
	t.Parallel()

	vpWidth := 80
	vpHeight := 10

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	var content strings.Builder
	for range 3 {
		content.WriteString(strings.Repeat("x", vpWidth-2))
		content.WriteString("\n")
	}

	content.WriteString("=== Build Logs ===\n")
	content.WriteString("\n")

	for range 5 {
		content.WriteString(strings.Repeat("y", vpWidth-6))
		content.WriteString("\n")
	}

	for range 20 {
		content.WriteString(strings.Repeat("z", vpWidth-2))
		content.WriteString("\n")
	}

	_ = mdl.SetContent(splitLines(content.String()))

	require.Greater(t, mdl.TotalLineCount(), vpHeight, "expected overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, vpWidth, visibleW, "line %d", idx)

		hasScrollbarTrack := strings.Contains(line, "│")
		hasScrollbarThumb := strings.Contains(line, "█")
		assert.True(t, hasScrollbarTrack || hasScrollbarThumb,
			"line %d: MISSING scrollbar (width=%d): %q", idx, visibleW, line)
	}
}

func TestMainViewportScrollbarWithANSIContent(t *testing.T) {
	t.Parallel()

	vpWidth := 80
	vpHeight := 10

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	ansiBold := "\x1b[1m"
	ansiReset := "\x1b[0m"

	var content strings.Builder
	for range 3 {
		content.WriteString(strings.Repeat("x", vpWidth-2))
		content.WriteString("\n")
	}

	content.WriteString(ansiBold + "=== Build Logs ===" + ansiReset + "\n")
	content.WriteString("\n")

	for range 5 {
		content.WriteString(ansiBold + "entity" + ansiReset + strings.Repeat("y", vpWidth-20))
		content.WriteString("\n")
	}

	for range 20 {
		content.WriteString(strings.Repeat("z", vpWidth-2))
		content.WriteString("\n")
	}

	mdl.SetWidth(vpWidth)
	mdl.SetHeight(vpHeight)
	_ = mdl.SetContent(splitLines(content.String()))

	t.Logf("TotalLineCount=%d contentH=%d ContentWidth=%d", mdl.TotalLineCount(), vpHeight, mdl.ContentWidth())

	require.Greater(t, mdl.TotalLineCount(), vpHeight, "expected overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		hasTrack := strings.Contains(line, "│")
		hasThumb := strings.Contains(line, "█")

		assert.Equal(t, vpWidth, visibleW, "line %d: width=%d want %d track=%v thumb=%v", idx, visibleW, vpWidth, hasTrack, hasThumb)
		assert.True(t, hasTrack || hasThumb, "line %d: MISSING scrollbar (width=%d)", idx, visibleW)
	}
}

func TestScrollbarReserveContentFits(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 10

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	_ = mdl.SetContent(splitLines("short content"))

	require.LessOrEqual(t, mdl.TotalLineCount(), vpHeight, "content should fit, got %d lines", mdl.TotalLineCount())

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, vpWidth, visibleW, "line %d", idx)
	}
}

func TestScrollbarReserveWidthMatchesOverflow(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 3

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	_ = mdl.SetContent(splitLines("short"))
	cwFits := mdl.ContentWidth()

	_ = mdl.SetContent(splitLines(strings.Repeat("x", 200)))
	cwOverflows := mdl.ContentWidth()

	assert.Equal(t, cwFits, cwOverflows, "ContentWidth changed: fits=%d overflows=%d, should be same with scrollbarReserve", cwFits, cwOverflows)
}

func TestScrollbarReserveWithWideContent(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 5

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	content := strings.Repeat("x", vpWidth) + "\nshort\n" + strings.Repeat("y", vpWidth) + "\n"
	_ = mdl.SetContent(splitLines(content))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, vpWidth, visibleW, "line %d", idx)
	}
}

func TestScrollbarReserveSyncItemSequence(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 5

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	content := "line1\nline2\nline3\n"

	mdl.SetWidth(vpWidth)
	mdl.SetHeight(vpHeight)
	_ = mdl.SetContent(splitLines(content))
	mdl.SetHeight(vpHeight)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	t.Logf("ContentWidth=%d TotalLines=%d contentH=%d", mdl.ContentWidth(), mdl.TotalLineCount(), vpHeight)

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		assert.Equal(t, vpWidth, visibleW, "line %d", idx)
	}

	longContent := ""

	var longContentSb1875 strings.Builder
	for range 20 {
		longContentSb1875.WriteString(strings.Repeat("z", vpWidth-2) + "\n")
	}

	longContent += longContentSb1875.String()

	mdl.SetWidth(vpWidth)
	mdl.SetHeight(vpHeight)
	_ = mdl.SetContent(splitLines(longContent))
	mdl.SetHeight(vpHeight)

	view = buffer.LinesBufToStringForTests(mdl.Render())
	viewLines = strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		hasTrack := strings.Contains(line, "│")
		hasThumb := strings.Contains(line, "█")

		assert.Equal(t, vpWidth, visibleW, "overflow line %d: width=%d want %d", idx, visibleW, vpWidth)
		assert.True(t, hasTrack || hasThumb, "overflow line %d: MISSING scrollbar", idx)
	}
}

func TestNonMainViewportSetContentReturnsNil(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(5),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	err := mdl.SetContent(splitLines(strings.Repeat("x", 500)))
	require.NoError(t, err, "non-main viewport should not error")
}

func TestSyncFixedHeight(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(5),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	err := mdl.Sync(splitLines("line1\nline2\nline3"), 20, 5)
	require.NoError(t, err, "Sync should not error")

	assert.Equal(t, 20, mdl.Width())
	assert.Equal(t, 5, mdl.Height())
	assert.Equal(t, 0, mdl.YOffset())
}

func TestSyncAutoHeightWithMaxHeight(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(1),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithMaxHeight(3),
		WithBorder(style.Color("")),
	)

	err := mdl.Sync(splitLines("line1\nline2"), 20, 0)
	require.NoError(t, err, "Sync should not error")

	assert.Equal(t, 4, mdl.Height(), "Height=%d want 4 (2 lines + border)", mdl.Height())
}

func TestSyncAutoHeightClampedByMaxHeight(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(1),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithMaxHeight(2),
		WithBorder(style.Color("")),
	)

	err := mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5"), 20, 0)
	require.NoError(t, err, "Sync should not error")

	assert.Equal(t, 4, mdl.Height(), "Height=%d want 4 (maxHeight=2 + border)", mdl.Height())
}

func TestSyncAutoScrollToBottom(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(3),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	_ = mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5"), 20, 0)
	assert.InEpsilon(t, float64(1), mdl.ScrollPercent(), 0.001, "should be at bottom after initial Sync, got %.2f", mdl.ScrollPercent())

	err := mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5\nline6\nline7"), 20, 0)
	require.NoError(t, err, "Sync should not error")

	assert.InEpsilon(t, float64(1), mdl.ScrollPercent(), 0.001, "should stay at bottom after Sync, got %.2f", mdl.ScrollPercent())
}

func TestSyncMainNoAutoScroll(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(3),
		WithMain(),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	_ = mdl.Sync(splitLines("line1\nline2\nline3"), 20, 3)
	mdl.SetYOffset(0)

	err := mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5"), 20, 3)
	require.NoError(t, err, "Sync should not error")

	assert.Equal(t, 0, mdl.YOffset(), "main viewport should not auto-scroll, got yOffset=%d", mdl.YOffset())
}

func TestResizeWrappingPreservesLineWidth(t *testing.T) {
	t.Parallel()

	longContent := strLines(
		"setting up /etc...",
		"reloading user units for krumpy-miha...",
		"restarting the following user units: nixos-activation.service",
		"reloading user units for root...",
		"restarting the following user units: nixos-activation.service",
		"restarting sysinit-reactivation.target",
		"the following new units were started: NetworkManager-dispatcher.service",
	)

	borderColor := style.Color("")

	// Create viewport at wide width where content fits without wrapping.
	mdl := New(
		WithWidth(80),
		WithHeight(15),
		WithBorder(borderColor),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	_ = mdl.SetContent(longContent)
	_ = mdl.Render()

	// Resize to a narrower width that forces wrapping.
	err := mdl.Sync(longContent, 62, 15)
	require.NoError(t, err)

	rendered := mdl.Render()

	for i := range rendered.Len() {
		line := rendered.Line(i)
		cw := style.CellWidth(line)
		assert.Equal(t, 62, cw, "line %d width after resize: got %d, want 62", i, cw)
	}
}

func TestResizeWrappingSweep(t *testing.T) {
	t.Parallel()

	longContent := strLines(
		"setting up /etc...",
		"reloading user units for krumpy-miha...",
		"restarting the following user units: nixos-activation.service",
		"reloading user units for root...",
		"restarting the following user units: nixos-activation.service",
		"restarting sysinit-reactivation.target",
		"the following new units were started: NetworkManager-dispatcher.service",
	)

	borderColor := style.Color("")

	// Sweep: create at width startW, resize to targetW, verify all lines.
	for _, startW := range []int{50, 60, 70, 80} {
		for _, targetW := range []int{40, 50, 55, 60, 62, 65, 70, 75, 80} {
			mdl := New(
				WithWidth(startW),
				WithHeight(15),
				WithBorder(borderColor),
				WithScrollbar("█", "│", style.Color(""), style.Color("")),
			)

			_ = mdl.SetContent(longContent)
			_ = mdl.Render()

			err := mdl.Sync(longContent, targetW, 15)
			require.NoError(t, err, "Sync startW=%d targetW=%d", startW, targetW)

			rendered := mdl.Render()
			for i := range rendered.Len() {
				line := rendered.Line(i)
				cw := style.CellWidth(line)
				assert.Equal(t, targetW, cw, "startW=%d targetW=%d line %d", startW, targetW, i)
			}
		}
	}
}
