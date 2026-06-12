package viewports

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

func splitLines(s string) *buffer.LinesBuf {
	lb := buffer.NewLinesBuf()
	if s != "" {
		lb.WriteLines(bytes.Split([]byte(s), []byte("\n")))
	}

	return lb
}

func TestNew(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	highlight := style.NewStyle().Background(style.Color("2"))
	highlightBorder := style.NewStyle().Background(style.Color("3"))

	viewports := New(dims, 10, border, highlight, highlightBorder)

	assert.Equal(t, 100, viewports.dimensions.Width)
	assert.Equal(t, 50, viewports.dimensions.Height)
	assert.Equal(t, "main", viewports.mainXpath.String(), "mainXpath = %q, want 'main'", viewports.mainXpath.String())
	assert.Equal(t, viewports.mainXpath, viewports.activeXpath, "activeXpath should be mainXpath initially")
	assert.False(t, viewports.IsFullscreen(), "should not be fullscreen initially")
}

func TestContentWidth(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	assert.Equal(t, 98, viewports.ContentWidth())
}

func TestFullscreen(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.SetFullscreen(xpath_)

	assert.True(t, viewports.IsFullscreen(), "should be fullscreen after SetFullscreen")
	assert.Equal(t, xpath_, viewports.GetFullscreenXpath())

	viewports.ExitFullscreen()

	assert.False(t, viewports.IsFullscreen(), "should not be fullscreen after ExitFullscreen")
}

func TestHasActiveInner(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	assert.False(t, viewports.HasActiveInner(), "should not have active inner initially (main is active)")

	xpath_ := xpath.New("test", "item")
	viewports.activeXpath = xpath_

	assert.True(t, viewports.HasActiveInner(), "should have active inner after setting non-main xpath")

	viewports.DeselectAll()

	assert.False(t, viewports.HasActiveInner(), "should not have active inner after DeselectAll")
}

func TestGetActiveInnerViewportContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	_, ok := viewports.GetActiveInnerViewportContent()
	assert.False(t, ok, "should not have content when no active inner")

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("test content"), 1, 0)

	viewports.activeXpath = xpath_

	content, ok := viewports.GetActiveInnerViewportContent()
	require.True(t, ok, "should have content with active inner")

	assert.Equal(t, "test content", string(content[0]))
}

func TestGetActiveInnerViewportXpath(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	assert.Empty(t, viewports.GetActiveInnerViewportXpath().String(),
		"should return empty xpath when no active inner")

	xpath_ := xpath.New("test", "item")
	viewports.activeXpath = xpath_

	assert.Equal(t, xpath_, viewports.GetActiveInnerViewportXpath())
}

func TestGetViewportContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)

	content := viewports.GetViewportContent(xpath_)
	assert.Equal(t, "content", string(content.Line(0)))

	missing := viewports.GetViewportContent(xpath.New("missing"))
	assert.Nil(t, missing)
}

func TestReset(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xp1 := xpath.New("test", "item1")
	xp2 := xpath.New("test", "item2")

	viewports.RenderViewportVersioned(xp1, splitLines("content1"), 1, 0)
	viewports.RenderViewportVersioned(xp2, splitLines("content2"), 1, 0)

	viewports.SetFullscreen(xp1)
	viewports.activeXpath = xp2

	viewports.Reset()

	assert.False(t, viewports.IsFullscreen(), "should not be fullscreen after reset")
	assert.Equal(t, viewports.mainXpath, viewports.activeXpath, "activeXpath should be main after reset")

	content := viewports.GetViewportContent(xp1)
	assert.Nil(t, content, "viewport should be removed after reset")
}

func TestRemoveIfExistsViewport(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)

	viewports.activeXpath = xpath_

	viewports.RemoveIfExistsViewport(xpath_)

	content := viewports.GetViewportContent(xpath_)
	assert.Nil(t, content, "viewport should be removed")
	assert.Equal(t, viewports.mainXpath, viewports.activeXpath, "activeXpath should be main after removing active viewport")
}

func TestGetOrCreateViewportVersioned(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("line1\nline2\nline3"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	assert.True(t, strings.Contains(output, "╭") || strings.Contains(output, "│"),
		"versioned viewport should have border")

	result = viewports.RenderViewportVersioned(xpath_, splitLines("line1\nline2\nline3"), 1, 0)
	output2 := buffer.LinesBufToStringForTests(result)

	assert.Equal(t, output, output2, "cached output should match")
}

func TestGetOrCreateLabelViewport(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "label")
	result := viewports.RenderLabelViewport(xpath_, splitLines("label text"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	assert.False(t, strings.Contains(output, "╭") || strings.Contains(output, "│"),
		"label viewport should not have border")
	assert.Contains(t, output, "\x1b[",
		"should contain ANSI zone marker")
}

func TestGetOrCreateMainViewport(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	content := strings.Repeat("line\n", 100)
	result := viewports.RenderMainViewport(splitLines(content), 1, 5)
	output := buffer.LinesBufToStringForTests(result)

	assert.True(t, strings.Contains(output, "█") || strings.Contains(output, "░"),
		"main viewport should have scrollbar when content overflows")
	assert.Contains(t, output, "\x1b[",
		"should contain ANSI zone marker")
}

func TestRenderFullscreenViewport_AlwaysBordered(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "fullscreen")
	content := strings.Repeat("line\n", 100)
	result := viewports.RenderFullscreenViewport(xpath_, splitLines(content), 1, 5)
	output := buffer.LinesBufToStringForTests(result)

	assert.True(t, strings.Contains(output, "╭") || strings.Contains(output, "╮"),
		"fullscreen viewport MUST have border (top corners)")
	assert.True(t, strings.Contains(output, "╰") || strings.Contains(output, "╯"),
		"fullscreen viewport MUST have border (bottom corners)")
	assert.Contains(t, output, "│",
		"fullscreen viewport MUST have border (sides)")
	assert.True(t, strings.Contains(output, "█") || strings.Contains(output, "░"),
		"fullscreen viewport should have scrollbar when content overflows")

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	assert.Len(t, lines, 45, "fullscreen height = %d lines, want 45", len(lines))
}

func TestFullscreenLabel_EnablesBorder(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "label")

	result := viewports.RenderLabelViewport(xpath_, splitLines("label content"), 1, 0)
	labelOutput := buffer.LinesBufToStringForTests(result)

	assert.NotContains(t, labelOutput, "╭", "label viewport should not have border")

	result = viewports.RenderFullscreenViewport(xpath_, splitLines("label content"), 2, 5)
	fullscreenOutput := buffer.LinesBufToStringForTests(result)

	assert.Contains(t, fullscreenOutput, "╭",
		"fullscreen label MUST have border (top-left corner)")
	assert.Contains(t, fullscreenOutput, "╮",
		"fullscreen label MUST have border (top-right corner)")
	assert.Contains(t, fullscreenOutput, "╰",
		"fullscreen label MUST have border (bottom-left corner)")
	assert.Contains(t, fullscreenOutput, "╯",
		"fullscreen label MUST have border (bottom-right corner)")
	assert.Contains(t, fullscreenOutput, "│",
		"fullscreen label MUST have border (sides)")
}

func TestFullscreenLabel_UsesGetViewportContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "label")

	viewports.RenderLabelViewport(xpath_, splitLines("label content"), 0, 0)

	content := viewports.GetViewportContent(xpath_)
	require.NotNil(t, content, "GetViewportContent should return content")
	assert.Equal(t, "label content", string(content.Line(0)))

	result := viewports.RenderFullscreenViewport(xpath_, content, 0, 5)
	fullscreenOutput := buffer.LinesBufToStringForTests(result)

	assert.Contains(t, fullscreenOutput, "label content",
		"fullscreen should contain label text when using GetViewportContent with same version")
	assert.Contains(t, fullscreenOutput, "╭",
		"fullscreen label MUST have border")
}

func TestRenderFullscreenViewport_FullWidth(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 80, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "fullscreen")
	content := strings.Repeat("x", 200)
	result := viewports.RenderFullscreenViewport(xpath_, splitLines(content), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	require.NotEmpty(t, lines, "should have rendered lines")

	assert.Contains(t, lines[0], "╭",
		"first line should be top border: %q", lines[0])

	debugBuf := buffer.NewLinesBuf()
	viewports.Debug(debugBuf)

	debug := buffer.LinesBufToStringForTests(debugBuf)
	assert.Contains(t, debug, "80x",
		"viewport should use width 80, debug: %s", debug)
}

func TestVersionTracking(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")

	viewports.RenderViewportVersioned(xpath_, splitLines("content v1"), 1, 0)

	content := viewports.GetViewportContent(xpath_)
	assert.Equal(t, "content v1", string(content.Line(0)))

	viewports.RenderViewportVersioned(xpath_, splitLines("content v1 updated"), 1, 0)

	content = viewports.GetViewportContent(xpath_)
	assert.Equal(t, "content v1 updated", string(content.Line(0)),
		"GetViewportContent always returns the latest buffer")

	viewports.RenderViewportVersioned(xpath_, splitLines("content v2"), 2, 0)

	content = viewports.GetViewportContent(xpath_)
	assert.Equal(t, "content v2", string(content.Line(0)))
}

func TestActiveHighlighting(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	highlight := style.NewStyle().Background(style.Color("1"))
	highlightBorder := style.NewStyle().Background(style.Color("2"))
	viewports := New(dims, 10, style.Style{}, highlight, highlightBorder)

	xpath_ := xpath.New("test", "item")
	viewports.activeXpath = xpath_

	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	assert.Contains(t, output, "\x1b[",
		"active viewport should have ANSI styling")
}

func TestIndentation(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "indented")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 10)
	output := buffer.LinesBufToStringForTests(result)

	assert.NotEmpty(t, output, "indented viewport should render")
}

func TestUpdate_MouseClick(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)

	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	strLines := strings.Split(output, "\n")

	buf := buffer.NewLinesBufDiff()

	for i, l := range strLines {
		if l != "" || i < len(strLines)-1 {
			buf.Write([]byte(l))
		}
	}

	click := zeroterm.MouseClickMsg{
		X:      5,
		Y:      0,
		Button: zeroterm.MouseLeft,
		Lines:  buf,
	}

	_ = viewports.Update(click)
}

func TestUpdate_NilViewports(t *testing.T) {
	t.Parallel()

	var viewports *Viewports

	cmd := viewports.Update(zeroterm.KeyPressMsg{})
	assert.Nil(t, cmd, "nil Viewports should return nil cmd")
}

func TestDebug(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)

	debugBuf := buffer.NewLinesBuf()
	viewports.Debug(debugBuf)
	debug := buffer.LinesBufToStringForTests(debugBuf)

	assert.Contains(t, debug, "Viewports: 1",
		"debug should show viewport count")
	assert.Contains(t, debug, "100x50",
		"debug should show dimensions")
	assert.Contains(t, debug, xpath_.String(),
		"debug should show viewport xpath")
}

func TestEmptyContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "empty")
	result := viewports.RenderViewportVersioned(xpath_, splitLines(""), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	assert.NotEmpty(t, output, "should render even with empty content")
}

func TestZeroDimensions(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 1, Height: 1}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)
	_ = buffer.LinesBufToStringForTests(result)
}

func TestMultipleViewports(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xp1 := xpath.New("test", "item1")
	xp2 := xpath.New("test", "item2")
	xp3 := xpath.New("test", "item3")

	viewports.RenderViewportVersioned(xp1, splitLines("content1"), 1, 0)
	viewports.RenderViewportVersioned(xp2, splitLines("content2"), 1, 0)
	viewports.RenderViewportVersioned(xp3, splitLines("content3"), 1, 0)

	c1 := viewports.GetViewportContent(xp1)
	assert.Equal(t, "content1", string(c1.Line(0)), "viewport 1 should have correct content")

	c2 := viewports.GetViewportContent(xp2)
	assert.Equal(t, "content2", string(c2.Line(0)), "viewport 2 should have correct content")

	c3 := viewports.GetViewportContent(xp3)
	assert.Equal(t, "content3", string(c3.Line(0)), "viewport 3 should have correct content")

	debugBuf := buffer.NewLinesBuf()
	viewports.Debug(debugBuf)

	debug := buffer.LinesBufToStringForTests(debugBuf)
	assert.Contains(t, debug, "Viewports: 3",
		"should have 3 viewports")
}
