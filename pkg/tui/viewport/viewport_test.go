package viewport

import (
	"bytes"
	"strings"
	"testing"

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
	if mdl.Width() != 80 {
		t.Errorf("width = %d, want 80", mdl.Width())
	}

	if mdl.Height() != 24 {
		t.Errorf("height = %d, want 24", mdl.Height())
	}
}

func TestSetContentLines(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("a", "b", "c"))

	if mdl.TotalLineCount() != 3 {
		t.Errorf("TotalLineCount = %d, want 3", mdl.TotalLineCount())
	}

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if strings.TrimSpace(lines[0]) != "a" || strings.TrimSpace(lines[1]) != "b" || strings.TrimSpace(lines[2]) != "c" {
		t.Errorf("view = %q", view)
	}
}

func TestSetContentLinesKeepsScroll(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.SetContentLines(strLines("a", "b", "c", "d", "e"))

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset should stay 2, got %d", mdl.YOffset())
	}

	mdl.SetContentLines(strLines("x", "y"))

	if mdl.YOffset() != 0 {
		t.Errorf("YOffset should be 0 after shrink, got %d", mdl.YOffset())
	}
}

func TestView(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4", "line5"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("view lines = %d, want 3", len(lines))
	}

	if strings.TrimSpace(lines[0]) != "line1" {
		t.Errorf("first line = %q, want line1", lines[0])
	}
}

func TestViewPaddedToWidth(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3))
	mdl.SetContentLines(strLines("short", "a bit longer line", "x"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	for i, line := range lines {
		if style.CellWidth([]byte(line)) != 20 {
			t.Errorf("line %d visible width = %d, want 20: %q", i, style.CellWidth([]byte(line)), line)
		}
	}
}

func TestViewHeightChanges(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if strings.Count(view, "\n") != 2 {
		t.Errorf("height 3 should have 2 newlines (3 lines), got %d in %q",
			strings.Count(view, "\n"), view)
	}

	mdl.SetHeight(2)

	view = buffer.LinesBufToStringForTests(mdl.Render())
	if strings.Count(view, "\n") != 1 {
		t.Errorf("height 2 should have 1 newline (2 lines), got %d in %q",
			strings.Count(view, "\n"), view)
	}

	mdl.SetHeight(10)

	view = buffer.LinesBufToStringForTests(mdl.Render())
	if strings.Count(view, "\n")+1 != 10 {
		t.Errorf("height 10 should have 10 lines, got %d in %q",
			strings.Count(view, "\n")+1, view)
	}
}

func TestScrollPercent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4", "line5"))

	if mdl.ScrollPercent() != 0 {
		t.Errorf("ScrollPercent at top = %f, want 0", mdl.ScrollPercent())
	}

	mdl.GotoBottom()

	if mdl.ScrollPercent() != 1 {
		t.Errorf("ScrollPercent at bottom = %f, want 1", mdl.ScrollPercent())
	}

	mdl.SetYOffset(1)

	pct := mdl.ScrollPercent()
	if pct <= 0 || pct >= 1 {
		t.Errorf("ScrollPercent mid = %f, want between 0 and 1", pct)
	}
}

func TestScrollPercentShortContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("a", "b"))

	if mdl.ScrollPercent() != 1 {
		t.Errorf("short content ScrollPercent = %f, want 1", mdl.ScrollPercent())
	}
}

func TestScrollPercentEmpty(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines())

	if mdl.ScrollPercent() != 1 {
		t.Errorf("empty ScrollPercent = %f, want 1", mdl.ScrollPercent())
	}
}

func TestGotoBottom(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4", "line5"))
	mdl.GotoBottom()

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if strings.TrimSpace(lines[0]) != "line3" {
		t.Errorf("first visible line after GotoBottom = %q, want line3", lines[0])
	}

	if strings.TrimSpace(lines[2]) != "line5" {
		t.Errorf("last visible line = %q, want line5", lines[2])
	}
}

func TestGotoBottomShorterContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("a", "b"))
	mdl.GotoBottom()

	if mdl.YOffset() != 0 {
		t.Errorf("short content GotoBottom = %d, want 0", mdl.YOffset())
	}
}

func TestSetYOffset(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4", "line5"))
	mdl.SetYOffset(2)

	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if strings.TrimSpace(lines[0]) != "line3" {
		t.Errorf("first line = %q, want line3", lines[0])
	}
}

func TestSetYOffsetClamped(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4", "line5"))

	mdl.SetYOffset(100)

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset after clamp = %d, want 2", mdl.YOffset())
	}

	mdl.SetYOffset(-5)

	if mdl.YOffset() != 0 {
		t.Errorf("YOffset after negative clamp = %d, want 0", mdl.YOffset())
	}
}

func TestContentShorterThanViewport(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("line1", "line2"))

	view := buffer.LinesBufToStringForTests(mdl.Render())

	count := strings.Count(view, "\n") + 1
	if count != 10 {
		t.Errorf("view length = %d, want 10: %q", count, view)
	}

	if !strings.HasPrefix(view, "line1") {
		t.Errorf("content incorrect: %q", view)
	}
}

func TestEmptyContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines())

	if mdl.TotalLineCount() != 0 {
		t.Errorf("TotalLineCount for empty = %d, want 0", mdl.TotalLineCount())
	}
}

func TestEmptyView(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines())

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if view != "" {
		t.Errorf("empty view = %q, want empty", view)
	}
}

func TestViewNoContentYet(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if view != "" {
		t.Errorf("uninitialized view = %q, want empty", view)
	}
}

func TestSetWidthPreservesContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("a", "b", "c"))

	mdl.SetWidth(40)

	if mdl.TotalLineCount() != 3 {
		t.Errorf("after width change, TotalLineCount = %d, want 3", mdl.TotalLineCount())
	}

	if mdl.Width() != 40 {
		t.Errorf("width = %d, want 40", mdl.Width())
	}
}

func TestSetWidthResetsScrollIfNeeded(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.SetContentLines(strLines("a", "b", "c", "d", "e"))

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset = %d, want 2", mdl.YOffset())
	}
}

func TestViewAtBottomWithResize(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.GotoBottom()

	mdl.SetHeight(10)

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset = %d, want 2 (was at bottom)", mdl.YOffset())
	}

	mdl.SetHeight(3)
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("view has %d lines, want 3", len(lines))
	}
}

func TestViewBeyondContent(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("line1", "line2"))
	mdl.SetYOffset(10)

	if mdl.YOffset() != 0 {
		t.Errorf("offset clamped to %d, want 0", mdl.YOffset())
	}
}

func TestViewWithANSI(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3))
	mdl.SetContentLines(strLines("\x1b[31mred\x1b[0m", "plain", "\x1b[1mbold\x1b[0m"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	for i, line := range lines {
		visibleLen := style.CellWidth([]byte(lines[i]))
		if visibleLen != 20 {
			t.Errorf("line %d visible width = %d, want 20: %q", i, visibleLen, line)
		}
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
		if got != tt.want {
			t.Errorf("style.CellWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestViewWidthZero(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(0), WithHeight(10))
	mdl.SetContentLines(strLines("a", "b"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if view != "" {
		t.Errorf("zero-width view = %q, want empty", view)
	}
}

func TestScrollDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	mdl.ScrollDown(1)

	if mdl.YOffset() != 1 {
		t.Errorf("YOffset after ScrollDown(1) = %d, want 1", mdl.YOffset())
	}

	mdl.ScrollDown(10)

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset after ScrollDown(10) = %d, want 2 (clamped)", mdl.YOffset())
	}
}

func TestScrollUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))
	mdl.SetYOffset(2)

	mdl.ScrollUp(1)

	if mdl.YOffset() != 1 {
		t.Errorf("YOffset after ScrollUp(1) = %d, want 1", mdl.YOffset())
	}

	mdl.ScrollUp(10)

	if mdl.YOffset() != 0 {
		t.Errorf("YOffset after ScrollUp(10) = %d, want 0 (clamped)", mdl.YOffset())
	}
}

func TestPageDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"))

	mdl.PageDown()

	if mdl.YOffset() != 3 {
		t.Errorf("YOffset after PageDown = %d, want 3", mdl.YOffset())
	}
}

func TestPageUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"))
	mdl.SetYOffset(5)

	mdl.PageUp()

	if mdl.YOffset() != 2 {
		t.Errorf("YOffset after PageUp = %d, want 2", mdl.YOffset())
	}
}

func TestHalfPageDown(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"))

	mdl.HalfPageDown()

	if mdl.YOffset() != 5 {
		t.Errorf("YOffset after HalfPageDown = %d, want 5", mdl.YOffset())
	}
}

func TestHalfPageUp(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(10))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"))
	mdl.SetYOffset(5)

	mdl.HalfPageUp()

	if mdl.YOffset() != 0 {
		t.Errorf("YOffset after HalfPageUp = %d, want 0", mdl.YOffset())
	}
}

func TestAtTopAtBottom(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(80), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	if !mdl.AtTop() {
		t.Error("should be at top initially")
	}

	mdl.GotoBottom()

	if !mdl.AtBottom() {
		t.Error("should be at bottom after GotoBottom")
	}

	mdl.SetYOffset(1)

	if mdl.AtTop() || mdl.AtBottom() {
		t.Error("should be neither top nor bottom in the middle")
	}
}

// ---- Comprehensive scrollbar tests ----

func TestScrollbarVisibleWidthWithOverflow(t *testing.T) {
	t.Parallel()

	// When content overflows and scrollbar is enabled, every line's visible width
	// must equal the viewport's configured width (content + scrollbar).
	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: visible width = %d, want %d (width): %q", idx, visibleW, vpWidth, line)
		}
	}
}

func TestScrollbarContentWidthSmallerThanTotal(t *testing.T) {
	t.Parallel()

	// Content area is width-2 when scrollbar is shown. Short content should be
	// padded to contentWidth, then scrollbar appended, totaling width.
	vpWidth := 10
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("hi", "there", "x", "overflow1", "overflow2"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: visible width = %d, want %d: %q", idx, visibleW, vpWidth, line)
		}

		// Content portion (before scrollbar) should be exactly contentWidth=8
		// Scrollbar is " │" or " █" (2 visible chars)
		contentPart := stripScrollbar(line)

		contentVisibleW := style.CellWidth([]byte(contentPart))
		if contentVisibleW != vpWidth-scrollbarColWidth {
			t.Errorf("line %d: content visible width = %d, want %d: content=%q full=%q",
				idx, contentVisibleW, vpWidth-scrollbarColWidth, contentPart, line)
		}
	}
}

func TestScrollbarHiddenContentFits(t *testing.T) {
	t.Parallel()

	// When content fits in viewport, no scrollbar should appear even if
	// WithScrollbar was specified. Width should be full contentWidth.
	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(10), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: visible width = %d, want %d (no scrollbar): %q",
				idx, visibleW, vpWidth, line)
		}

		if strings.Contains(line, "█") || strings.Contains(line, "│") {
			t.Errorf("line %d: scrollbar chars should not appear when content fits: %q", idx, line)
		}
	}
}

func TestScrollbarNoScrollbarOption(t *testing.T) {
	t.Parallel()

	// Without WithScrollbar, even overflowing content should have no scrollbar.
	vpWidth := 20
	mdl := New(WithWidth(vpWidth), WithHeight(3))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: visible width = %d, want %d: %q", idx, visibleW, vpWidth, line)
		}
	}
}

func TestScrollbarThumbAtTop(t *testing.T) {
	t.Parallel()

	// At yOffset=0, thumb should be in the first visible lines.
	vpWidth := 20
	vpHeight := 3
	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbLine := -1

	for idx, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbLine = idx

			break
		}
	}

	if thumbLine != 0 {
		t.Errorf("at top, thumb should be at line 0, got line %d", thumbLine)
	}
}

func TestScrollbarThumbAtBottom(t *testing.T) {
	t.Parallel()

	vpWidth := 20
	vpHeight := 3
	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))
	mdl.GotoBottom()

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbLine := -1

	for idx, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbLine = idx
		}
	}

	if thumbLine != vpHeight-1 {
		t.Errorf("at bottom, thumb should be at line %d, got line %d", vpHeight-1, thumbLine)
	}
}

func TestScrollbarThumbSizeProportional(t *testing.T) {
	t.Parallel()

	// With 9 lines and height 3: thumb = max(1, 3*3/9) = 1
	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	thumbCount := 0

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbCount++
		}
	}

	if thumbCount != 1 {
		t.Errorf("thumb count = %d, want 1 (9 lines, height 3)", thumbCount)
	}

	// With 30 lines and height 10: thumb = max(1, 10*10/30) = 3
	mdl2 := New(WithWidth(20), WithHeight(10), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	longContent := make([][]byte, 30)
	for idx := range longContent {
		longContent[idx] = []byte("line")
	}

	mdl2.SetContentLines(longContent)

	view = buffer.LinesBufToStringForTests(mdl2.Render())
	viewLines = strings.Split(view, "\n")

	thumbCount = 0

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbCount++
		}
	}

	if thumbCount != 3 {
		t.Errorf("thumb count = %d, want 3 (30 lines, height 10)", thumbCount)
	}
}

func TestScrollbarWithANSIStyles(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color("1"), style.Color("4")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	// Each line's visible width must still be exactly vpWidth
	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != 20 {
			t.Errorf("line %d: visible width = %d, want 20 (styled scrollbar): %q", idx, visibleW, line)
		}
	}

	// Check thumb has red style
	if !strings.Contains(viewLines[0], "█") || !strings.Contains(viewLines[0], "\x1b[") {
		t.Errorf("line 0 should have styled red thumb: %q", viewLines[0])
	}

	if !strings.Contains(viewLines[1], "│") || !strings.Contains(viewLines[1], "\x1b[") {
		t.Errorf("line 1 should have styled blue track: %q", viewLines[1])
	}

	if !strings.Contains(viewLines[1], "│") || !strings.Contains(viewLines[1], "\x1b[") {
		t.Errorf("line 1 should have styled blue track: %q", viewLines[1])
	}
}

func TestScrollbarFillLinesAlsoGetScrollbar(t *testing.T) {
	t.Parallel()

	// When content is shorter than height but still overflows (2 content, 3 height, 5 total lines),
	// the fill lines should also have the scrollbar.
	mdl := New(WithWidth(10), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	if len(viewLines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(viewLines))
	}

	// Every line should have a scrollbar character
	for idx, line := range viewLines {
		hasScrollbar := strings.Contains(line, "█") || strings.Contains(line, "│")
		if !hasScrollbar {
			t.Errorf("line %d: missing scrollbar in fill line: %q", idx, line)
		}

		visibleW := style.CellWidth([]byte(line))
		if visibleW != 10 {
			t.Errorf("line %d: visible width = %d, want 10: %q", idx, visibleW, line)
		}
	}
}

func TestScrollbarNoScrollbarWhenContentFitsExactly(t *testing.T) {
	t.Parallel()

	// When totalLines == height, content fits exactly — no scrollbar
	mdl := New(WithWidth(20), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5"))

	view := buffer.LinesBufToStringForTests(mdl.Render())

	if strings.Contains(view, "█") || strings.Contains(view, "│") {
		t.Errorf("scrollbar should not appear when content fits exactly: %q", view)
	}

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != 20 {
			t.Errorf("line %d: visible width = %d, want 20 (no scrollbar): %q", idx, visibleW, line)
		}
	}
}

func TestScrollbarContentWidth(t *testing.T) {
	t.Parallel()

	// With scrollbar but no content: ContentWidth returns full width (no overflow yet)
	mdl := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("a", "b")) // 2 lines, fits in height=3

	if mdl.ContentWidth() != 20 {
		t.Errorf("ContentWidth no overflow = %d, want 20", mdl.ContentWidth())
	}

	// With scrollbar and content that overflows: ContentWidth deducts scrollbar
	mdl2 := New(WithWidth(20), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl2.SetContentLines(strLines("a", "b", "c", "d", "e")) // 5 lines, overflows height=3

	if mdl2.ContentWidth() != 18 {
		t.Errorf("ContentWidth with overflow = %d, want 18", mdl2.ContentWidth())
	}

	// Without scrollbar: ContentWidth() == width
	mdl3 := New(WithWidth(20), WithHeight(3))
	if mdl3.ContentWidth() != 20 {
		t.Errorf("ContentWidth without scrollbar = %d, want 20", mdl3.ContentWidth())
	}
}

func TestScrollbarExactLineComposition(t *testing.T) {
	t.Parallel()

	// Verify exact structure of each line:
	// content (padded to contentWidth=8) + " " + scrollbarChar
	vpWidth := 10
	mdl := New(WithWidth(vpWidth), WithHeight(3), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("hi", "there", "x", "y", "z"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	contentW := vpWidth - scrollbarColWidth // 8

	for idx, line := range viewLines {
		// Strip ANSI (none here) and check structure
		// Content part should be exactly contentWidth chars
		// Then " │" or " █"
		scrollbarStart := strings.LastIndex(line, " ")
		if scrollbarStart < 0 {
			t.Errorf("line %d: no space before scrollbar: %q", idx, line)

			continue
		}

		contentPart := line[:scrollbarStart]
		scrollbarPart := line[scrollbarStart:]

		contentVisibleW := style.CellWidth([]byte(contentPart))
		if contentVisibleW != contentW {
			t.Errorf("line %d: content part visible width = %d, want %d: content=%q",
				idx, contentVisibleW, contentW, contentPart)
		}

		if scrollbarPart != " █" && scrollbarPart != " │" {
			t.Errorf("line %d: unexpected scrollbar part %q", idx, scrollbarPart)
		}
	}
}

func TestScrollbarScrollingMovesThumb(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(20), WithHeight(5), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("line")
	}

	mdl.SetContentLines(content)

	// At top: thumb at line 0
	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	if !strings.Contains(viewLines[0], "█") {
		t.Errorf("at top, line 0 should have thumb")
	}

	// Scroll down 5: thumb should have moved
	mdl.SetYOffset(5)
	view = buffer.LinesBufToStringForTests(mdl.Render())
	viewLines = strings.Split(view, "\n")

	// Thumb should NOT be at line 0 anymore
	if strings.Contains(viewLines[0], "█") {
		t.Errorf("after scrolling, line 0 should not have thumb anymore")
	}

	// Thumb should be somewhere in the middle
	thumbFound := false

	for _, line := range viewLines {
		if strings.Contains(line, "█") {
			thumbFound = true

			break
		}
	}

	if !thumbFound {
		t.Errorf("thumb should be visible somewhere after scrolling")
	}
}

// stripScrollbar removes the trailing " │" or " █" from a line,
// handling both plain and ANSI-styled scrollbar chars.
func stripScrollbar(line string) string {
	// Try plain first
	for _, suffix := range []string{" │", " █"} {
		if strings.HasSuffix(line, suffix) {
			return line[:len(line)-len(suffix)]
		}
	}

	// Try ANSI-styled: ends with \x1b[0m after the char
	for _, char := range []string{"█", "│"} {
		target := char + "\x1b[0m"

		idx := strings.LastIndex(line, target)
		if idx >= 0 {
			// Go back to find the space before the style
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
	mdl.SetContentLines(strLines("AB"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	expected := "╭────╮\n│AB  │\n╰────╯"
	if view != expected {
		t.Errorf("bordered view = %q, want %q", view, expected)
	}
}

func TestBorderOutputSize(t *testing.T) {
	t.Parallel()

	width := 10
	height := 4
	mdl := New(WithWidth(width), WithHeight(height), WithBorder(style.Color("")))
	mdl.SetContentLines(strLines("line1", "line2"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Errorf("bordered view has %d lines, want %d", len(lines), height)
	}

	for i, line := range lines {
		if style.CellWidth([]byte(line)) != width {
			t.Errorf("line %d visible width = %d, want %d: %q", i, style.CellWidth([]byte(line)), width, line)
		}
	}
}

func TestBorderWithOverflowAndScrollbar(t *testing.T) {
	t.Parallel()

	width := 12
	height := 5
	mdl := New(WithWidth(width), WithHeight(height), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("a", "b", "c", "d", "e", "f", "g", "h"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Errorf("bordered+scrollbar view has %d lines, want %d", len(lines), height)
	}

	for i, line := range lines {
		if style.CellWidth([]byte(line)) != width {
			t.Errorf("line %d visible width = %d, want %d: %q", i, style.CellWidth([]byte(line)), width, line)
		}
	}
}

func TestBorderContentWidth(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(10), WithHeight(3), WithBorder(style.Color("")))
	if mdl.ContentWidth() != 8 {
		t.Errorf("bordered ContentWidth = %d, want 8", mdl.ContentWidth())
	}

	// Bordered+scrollbar with content that fits: no scrollbar deduction
	mdl2 := New(WithWidth(10), WithHeight(5), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl2.SetContentLines(strLines("a", "b")) // fits in contentH=3

	if mdl2.ContentWidth() != 8 {
		t.Errorf("bordered+scrollbar (fits) ContentWidth = %d, want 8", mdl2.ContentWidth())
	}

	// Bordered+scrollbar with content that overflows: scrollbar deducted
	mdl3 := New(WithWidth(10), WithHeight(5), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl3.SetContentLines(strLines("a", "b", "c", "d", "e", "f")) // 6 lines > contentH=3

	if mdl3.ContentWidth() != 6 {
		t.Errorf("bordered+scrollbar (overflows) ContentWidth = %d, want 6", mdl3.ContentWidth())
	}
}

func TestBorderScrolling(t *testing.T) {
	t.Parallel()

	// height=5 → contentH=3 (5-2 for border), 6 lines → maxYOffset=3
	mdl := New(WithWidth(8), WithHeight(5), WithBorder(style.Color("")))
	mdl.SetContentLines(strLines("a", "b", "c", "d", "e", "f"))

	if mdl.AtBottom() {
		t.Error("should not be at bottom")
	}

	mdl.ScrollDown(3)

	if !mdl.AtBottom() {
		t.Errorf("should be at bottom after scrolling 3, yOffset=%d, maxYOffset=%d", mdl.YOffset(), len(mdl.lines)-3)
	}

	view := buffer.LinesBufToStringForTests(mdl.Render())
	lines := strings.Split(view, "\n")

	// yOffset=3, contentH=3 → shows lines d, e, f
	if !strings.Contains(lines[1], "d") {
		t.Errorf("expected line 1 to contain 'd', got %q", lines[1])
	}
}

func TestBorderStyled(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(6), WithHeight(3), WithBorder(style.Color("1")))
	mdl.SetContentLines(strLines("AB"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "╭") {
		t.Error("border top-left should be styled")
	}

	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "│") {
		t.Error("border sides should be styled")
	}

	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "╰") {
		t.Error("border bottom-left should be styled")
	}
}

func TestBorderIsBordered(t *testing.T) {
	t.Parallel()

	mdl := New(WithWidth(10), WithHeight(3), WithBorder(style.Color("")))
	if !mdl.IsBordered() {
		t.Error("IsBordered should be true")
	}

	mdl2 := New(WithWidth(10), WithHeight(3))
	if mdl2.IsBordered() {
		t.Error("IsBordered should be false")
	}
}

func TestBorderOverheadFunc(t *testing.T) {
	t.Parallel()

	if BorderOverhead() != 2 {
		t.Errorf("BorderOverhead() = %d, want 2", BorderOverhead())
	}
}

// ---- Scrollbar reservation and consistency tests ----

func TestScrollbarAlwaysReservesWidth(t *testing.T) {
	t.Parallel()

	// Every output line must be exactly `width` visible chars wide,
	// regardless of whether content overflows or not. When content overflows,
	// scrollbar column takes 2 chars. When content fits, no scrollbar column.
	vpWidth := 20
	vpHeight := 10

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	// Content fits — no scrollbar drawn, but 2-char area is spaces
	mdl.SetContentLines(strLines("short", "content"))
	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("content-fits line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}

	// Content overflows — scrollbar drawn
	mdl.SetContentLines(strLines("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12"))
	view = buffer.LinesBufToStringForTests(mdl.Render())

	viewLines = strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("content-overflows line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestScrollbarVisibleAtBottom(t *testing.T) {
	t.Parallel()

	// When scrolled to bottom, scrollbar must still show track/thumb,
	// not disappear or turn into spaces.
	vpWidth := 20
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("line")
	}

	mdl.SetContentLines(content)

	// At top
	view := buffer.LinesBufToStringForTests(mdl.Render())
	if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
		t.Errorf("at top: scrollbar should be visible\n%q", view)
	}

	// At bottom
	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
		t.Errorf("at bottom: scrollbar should be visible\n%q", view)
	}

	// Verify every line is exactly vpWidth wide at bottom
	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("at-bottom line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
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

	mdl.SetContentLines(content)

	for offset := 0; offset <= mdl.maxYOffset(); offset++ {
		mdl.SetYOffset(offset)

		view := buffer.LinesBufToStringForTests(mdl.Render())
		if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
			t.Errorf("at yOffset=%d: scrollbar should be visible\n%q", offset, view)
		}

		viewLines := strings.Split(view, "\n")
		for idx, line := range viewLines {
			if style.CellWidth([]byte(line)) != vpWidth {
				t.Errorf("yOffset=%d line %d: width = %d, want %d: %q", offset, idx, style.CellWidth([]byte(line)), vpWidth, line)
			}
		}
	}
}

func TestScrollbarColumnIsSpacesWhenContentFits(t *testing.T) {
	t.Parallel()

	// When content fits, no scrollbar column is rendered at all.
	// Content fills the full width.
	vpWidth := 20
	vpHeight := 10

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("a", "b"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if strings.Contains(view, "█") || strings.Contains(view, "│") {
		t.Errorf("when content fits, no scrollbar chars should appear\n%q", view)
	}

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestBorderedScrollbarWidthConsistency(t *testing.T) {
	t.Parallel()

	// Bordered + scrollbar: every line must be exactly `width` visible chars.
	// Border takes 2, scrollbar takes 2, content gets width-4.
	vpWidth := 16
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte("x")
	}

	mdl.SetContentLines(content)

	// Scroll through every position
	for offset := 0; offset <= mdl.maxYOffset(); offset++ {
		mdl.SetYOffset(offset)
		view := buffer.LinesBufToStringForTests(mdl.Render())
		viewLines := strings.Split(view, "\n")

		if len(viewLines) != vpHeight {
			t.Errorf("yOffset=%d: %d lines, want %d", offset, len(viewLines), vpHeight)
		}

		for idx, line := range viewLines {
			if style.CellWidth([]byte(line)) != vpWidth {
				t.Errorf("yOffset=%d line %d: width = %d, want %d: %q", offset, idx, style.CellWidth([]byte(line)), vpWidth, line)
			}
		}

		// Border chars must be present
		if !strings.HasPrefix(viewLines[0], "╭") {
			t.Errorf("yOffset=%d: missing top-left border", offset)
		}

		if !strings.HasPrefix(viewLines[vpHeight-1], "╰") {
			t.Errorf("yOffset=%d: missing bottom-left border", offset)
		}
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

	mdl.SetContentLines(content)

	mdl.GotoBottom()
	view := buffer.LinesBufToStringForTests(mdl.Render())

	// Scrollbar must be visible (track or thumb chars)
	if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
		t.Errorf("at bottom: scrollbar should be visible in bordered viewport\n%q", view)
	}

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("at-bottom line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestSetContentWrapsAtScrollbarWidth(t *testing.T) {
	t.Parallel()

	// SetContent first wraps at full content width, then rewraps at
	// scrollbar-reduced width if content overflows.
	vpWidth := 10
	vpHeight := 3

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	// 10 chars at full content width (10) fits in 1 line, no overflow → no rewrap
	_ = mdl.SetContent(splitLines("abcdefghij"))

	if mdl.TotalLineCount() != 1 {
		t.Errorf("short content should not rewrap, got %d lines", mdl.TotalLineCount())
	}

	// 50 chars at full content width (10) = 5 lines > height 3 → overflow → rewrap at 8
	mdl2 := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	_ = mdl2.SetContent(splitLines(strings.Repeat("abcdefghij", 5)))

	if mdl2.TotalLineCount() < 5 {
		t.Errorf("long content should wrap at narrow width, got %d lines", mdl2.TotalLineCount())
	}

	// Every line in View() must be exactly vpWidth wide
	view := buffer.LinesBufToStringForTests(mdl2.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestBorderedSetContentWrapsCorrectly(t *testing.T) {
	t.Parallel()

	// Bordered + scrollbar: full content width = width - 2(border)
	// When content overflows, rewrap at width - 2(border) - 2(scrollbar)
	vpWidth := 14
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	fullContentWidth := vpWidth - 2       // 12, no scrollbar initially
	narrowContentWidth := vpWidth - 2 - 2 // 10, with scrollbar

	// Content that overflows at full width triggers rewrap at narrow width
	longContent := strings.Repeat("x", 60) // 60 chars
	_ = mdl.SetContent(splitLines(longContent))

	// Each wrapped line must fit within the narrow content width (scrollbar will appear)
	for idx, line := range mdl.lines {
		if style.CellWidth([]byte(line)) > narrowContentWidth {
			t.Errorf("wrapped line %d: width = %d, want <= %d: %q", idx, style.CellWidth([]byte(line)), narrowContentWidth, line)
		}
	}

	// Content that fits at full width should NOT rewrap
	mdl2 := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	shortContent := strings.Repeat("x", fullContentWidth) // 12 chars, fits in 1 line
	_ = mdl2.SetContent(splitLines(shortContent))

	if mdl2.TotalLineCount() > 1 {
		t.Errorf("short content should not wrap, got %d lines", mdl2.TotalLineCount())
	}
}

func TestScrollbarNoScrollbarOptionStillFullWidth(t *testing.T) {
	t.Parallel()

	// Without scrollbar, full width is used for content
	vpWidth := 20
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight))

	content := make([][]byte, 20)
	for idx := range content {
		content[idx] = []byte(strings.Repeat("x", vpWidth))
	}

	mdl.SetContentLines(content)

	view := buffer.LinesBufToStringForTests(mdl.Render())

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("no-scrollbar line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestBorderedScrollbarFitsContentFitsExactly(t *testing.T) {
	t.Parallel()

	// contentH = height - 2(border) = 3, content has exactly 3 lines.
	// Content fits, so no scrollbar column at all.
	vpWidth := 12
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("line1", "line2", "line3"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	if len(viewLines) != vpHeight {
		t.Errorf("lines = %d, want %d", len(viewLines), vpHeight)
	}

	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}

	// No scrollbar thumb when content fits exactly.
	if strings.Contains(view, "█") {
		t.Errorf("scrollbar thumb should not appear when content fits exactly\n%q", view)
	}
}

func TestBorderedScrollbarContentOverflowsByOne(t *testing.T) {
	t.Parallel()

	// contentH = height - 2(border) = 3, content has 4 lines (overflows by 1).
	// Scrollbar should appear.
	vpWidth := 12
	vpHeight := 5

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))
	mdl.SetContentLines(strLines("line1", "line2", "line3", "line4"))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
		t.Errorf("scrollbar should appear when content overflows by 1\n%q", view)
	}

	viewLines := strings.Split(view, "\n")
	for idx, line := range viewLines {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("line %d: width = %d, want %d: %q", idx, style.CellWidth([]byte(line)), vpWidth, line)
		}
	}
}

func TestSetContentScrollbarAtTopAndBottom(t *testing.T) {
	t.Parallel()

	// Non-bordered, scrollbar, content that wraps due to being too wide.
	vpWidth := 10
	vpHeight := 3

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	// 50 chars, wraps to ~7 lines at contentWidth=8, overflows viewport
	_ = mdl.SetContent(splitLines(strings.Repeat("abcdefghij", 5)))

	if mdl.TotalLineCount() <= vpHeight {
		t.Fatalf("expected content to overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)
	}

	// At top: scrollbar must show
	view := buffer.LinesBufToStringForTests(mdl.Render())
	if !strings.Contains(view, "█") {
		t.Errorf("at top: scrollbar thumb should be visible\n%q", view)
	}

	for i, line := range strings.Split(view, "\n") {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("at top line %d: width=%d want %d", i, style.CellWidth([]byte(line)), vpWidth)
		}
	}

	// At bottom: scrollbar must still show
	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	if !strings.Contains(view, "█") && !strings.Contains(view, "│") {
		t.Errorf("at bottom: scrollbar should be visible\n%q", view)
	}

	for i, line := range strings.Split(view, "\n") {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("at bottom line %d: width=%d want %d", i, style.CellWidth([]byte(line)), vpWidth)
		}
	}
}

func TestBorderedSetContentScrollbarAtTopAndBottom(t *testing.T) {
	t.Parallel()

	// Bordered + scrollbar, content wraps due to width.
	vpWidth := 14
	vpHeight := 6

	mdl := New(WithWidth(vpWidth), WithHeight(vpHeight), WithBorder(style.Color("")), WithScrollbar("█", "│", style.Color(""), style.Color("")))

	// At full content width (12): 50 chars → 5 lines > contentH=4 → overflow → rewrap at 10
	_ = mdl.SetContent(splitLines(strings.Repeat("x", 50)))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	for i, line := range strings.Split(view, "\n") {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("line %d: width=%d want %d", i, style.CellWidth([]byte(line)), vpWidth)
		}
	}

	// At bottom
	mdl.GotoBottom()

	view = buffer.LinesBufToStringForTests(mdl.Render())
	for i, line := range strings.Split(view, "\n") {
		if style.CellWidth([]byte(line)) != vpWidth {
			t.Errorf("at bottom line %d: width=%d want %d", i, style.CellWidth([]byte(line)), vpWidth)
		}
	}

	if !strings.Contains(view, "█") {
		t.Errorf("at bottom: scrollbar thumb should be visible\n%q", view)
	}
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

	if mdl.TotalLineCount() <= vpHeight {
		t.Fatalf("expected overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)
	}

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: width=%d want %d: %q", idx, visibleW, vpWidth, line)
		}

		hasScrollbarTrack := strings.Contains(line, "│")

		hasScrollbarThumb := strings.Contains(line, "█")
		if !hasScrollbarTrack && !hasScrollbarThumb {
			t.Errorf("line %d: MISSING scrollbar (width=%d): %q", idx, visibleW, line)
		}
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

	// Simulate syncItem: set width, then height, then content
	mdl.SetWidth(vpWidth)
	mdl.SetHeight(vpHeight)
	_ = mdl.SetContent(splitLines(content.String()))

	t.Logf("TotalLineCount=%d contentH=%d ContentWidth=%d", mdl.TotalLineCount(), vpHeight, mdl.ContentWidth())

	if mdl.TotalLineCount() <= vpHeight {
		t.Fatalf("expected overflow, got %d lines <= %d height", mdl.TotalLineCount(), vpHeight)
	}

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		hasTrack := strings.Contains(line, "│")
		hasThumb := strings.Contains(line, "█")

		if visibleW != vpWidth {
			t.Errorf("line %d: width=%d want %d track=%v thumb=%v", idx, visibleW, vpWidth, hasTrack, hasThumb)
		}

		if !hasTrack && !hasThumb {
			t.Errorf("line %d: MISSING scrollbar (width=%d)", idx, visibleW)
		}
	}
}

func TestScrollbarReserveContentFits(t *testing.T) {
	t.Parallel()

	// When scrollbarReserve is set and content fits, every line must be
	// exactly vpWidth chars with a 2-space scrollbar column.
	vpWidth := 20
	vpHeight := 10

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	_ = mdl.SetContent(splitLines("short content"))

	if mdl.TotalLineCount() > vpHeight {
		t.Fatalf("content should fit, got %d lines", mdl.TotalLineCount())
	}

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: width=%d want %d: %q", idx, visibleW, vpWidth, line)
		}
	}
}

func TestScrollbarReserveWidthMatchesOverflow(t *testing.T) {
	t.Parallel()

	// ContentWidth must be the same whether content overflows or not
	// when scrollbarReserve is set.
	vpWidth := 20
	vpHeight := 3

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	// Short content: fits
	_ = mdl.SetContent(splitLines("short"))
	cwFits := mdl.ContentWidth()

	// Long content: overflows
	_ = mdl.SetContent(splitLines(strings.Repeat("x", 200)))
	cwOverflows := mdl.ContentWidth()

	if cwFits != cwOverflows {
		t.Errorf("ContentWidth changed: fits=%d overflows=%d, should be same with scrollbarReserve", cwFits, cwOverflows)
	}
}

func TestScrollbarReserveWithWideContent(t *testing.T) {
	t.Parallel()

	// Content that's wider than ContentWidth should be wrapped,
	// and every line in View() should be exactly vpWidth.
	vpWidth := 20
	vpHeight := 5

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	// Content with a line exactly vpWidth chars (wider than ContentWidth=18)
	content := strings.Repeat("x", vpWidth) + "\nshort\n" + strings.Repeat("y", vpWidth) + "\n"
	_ = mdl.SetContent(splitLines(content))

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: width=%d want %d: %q", idx, visibleW, vpWidth, line)
		}
	}
}

func TestScrollbarReserveSyncItemSequence(t *testing.T) {
	t.Parallel()

	// Simulate exact syncItem sequence for main viewport:
	// SetWidth → SetHeight(prelim) → SetContent → SetHeight(final)
	vpWidth := 20
	vpHeight := 5

	mdl := New(
		WithWidth(vpWidth),
		WithHeight(vpHeight),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
		WithScrollbarReserve(),
	)

	// Content fits (3 lines < 5 height)
	content := "line1\nline2\nline3\n"

	// syncItem sequence
	mdl.SetWidth(vpWidth)
	mdl.SetHeight(vpHeight) // preliminary
	_ = mdl.SetContent(splitLines(content))
	// finalHeight with explicitHeight: same as preliminary
	mdl.SetHeight(vpHeight)

	view := buffer.LinesBufToStringForTests(mdl.Render())
	viewLines := strings.Split(view, "\n")

	t.Logf("ContentWidth=%d TotalLines=%d contentH=%d", mdl.ContentWidth(), mdl.TotalLineCount(), vpHeight)

	for idx, line := range viewLines {
		visibleW := style.CellWidth([]byte(line))
		if visibleW != vpWidth {
			t.Errorf("line %d: width=%d want %d: %q", idx, visibleW, vpWidth, line)
		}
	}

	// Now set overflowing content
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

		if visibleW != vpWidth {
			t.Errorf("overflow line %d: width=%d want %d", idx, visibleW, vpWidth)
		}

		if !hasTrack && !hasThumb {
			t.Errorf("overflow line %d: MISSING scrollbar", idx)
		}
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
	if err != nil {
		t.Fatalf("non-main viewport should not error: %v", err)
	}
}

func TestSyncFixedHeight(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(5),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	err := mdl.Sync(splitLines("line1\nline2\nline3"), 20, 5)
	if err != nil {
		t.Fatalf("Sync should not error: %v", err)
	}

	if mdl.Width() != 20 {
		t.Errorf("Width=%d want 20", mdl.Width())
	}

	if mdl.Height() != 5 {
		t.Errorf("Height=%d want 5", mdl.Height())
	}

	if mdl.YOffset() != 0 {
		t.Errorf("YOffset=%d want 0", mdl.YOffset())
	}
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
	if err != nil {
		t.Fatalf("Sync should not error: %v", err)
	}

	if mdl.Height() != 4 { // 2 lines + borderOverhead(2)
		t.Errorf("Height=%d want 4 (2 lines + border)", mdl.Height())
	}
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
	if err != nil {
		t.Fatalf("Sync should not error: %v", err)
	}

	if mdl.Height() != 4 { // maxHeight(2) + borderOverhead(2)
		t.Errorf("Height=%d want 4 (maxHeight=2 + border)", mdl.Height())
	}
}

func TestSyncAutoScrollToBottom(t *testing.T) {
	t.Parallel()

	mdl := New(
		WithWidth(20),
		WithHeight(3),
		WithScrollbar("█", "│", style.Color(""), style.Color("")),
	)

	_ = mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5"), 20, 0)
	if mdl.ScrollPercent() != 1 {
		t.Errorf("should be at bottom after initial Sync, got %.2f", mdl.ScrollPercent())
	}

	err := mdl.Sync(splitLines("line1\nline2\nline3\nline4\nline5\nline6\nline7"), 20, 0)
	if err != nil {
		t.Fatalf("Sync should not error: %v", err)
	}

	if mdl.ScrollPercent() != 1 {
		t.Errorf("should stay at bottom after Sync, got %.2f", mdl.ScrollPercent())
	}
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
	if err != nil {
		t.Fatalf("Sync should not error: %v", err)
	}

	if mdl.YOffset() != 0 {
		t.Errorf("main viewport should not auto-scroll, got yOffset=%d", mdl.YOffset())
	}
}
