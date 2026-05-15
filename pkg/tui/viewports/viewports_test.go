package viewports

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

func splitLines(s string) [][]byte {
	if s == "" {
		return nil
	}

	return bytes.Split([]byte(s), []byte("\n"))
}

func splitLinesBuf(s string) *buffer.LinesBuf {
	lb := buffer.NewLinesBuf()
	lb.WriteLines(splitLines(s))

	return lb
}

func TestNew(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	highlight := style.NewStyle().Background(style.Color("2"))
	highlightBorder := style.NewStyle().Background(style.Color("3"))

	viewports := New(dims, 10, border, highlight, highlightBorder)

	if viewports.dimensions.Width != 100 {
		t.Errorf("width = %d, want 100", viewports.dimensions.Width)
	}

	if viewports.dimensions.Height != 50 {
		t.Errorf("height = %d, want 50", viewports.dimensions.Height)
	}

	if viewports.mainXpath.String() != "main" {
		t.Errorf("mainXpath = %q, want 'main'", viewports.mainXpath.String())
	}

	if viewports.activeXpath != viewports.mainXpath {
		t.Error("activeXpath should be mainXpath initially")
	}

	if viewports.IsFullscreen() {
		t.Error("should not be fullscreen initially")
	}
}

func TestContentWidth(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	expected := 100 - 2
	if viewports.ContentWidth() != expected {
		t.Errorf("ContentWidth = %d, want %d", viewports.ContentWidth(), expected)
	}
}

func TestFullscreen(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.SetFullscreen(xpath_)

	if !viewports.IsFullscreen() {
		t.Error("should be fullscreen after SetFullscreen")
	}

	if viewports.GetFullscreenXpath() != xpath_ {
		t.Errorf("fullscreen xpath = %v, want %v", viewports.GetFullscreenXpath(), xpath_)
	}

	viewports.ExitFullscreen()

	if viewports.IsFullscreen() {
		t.Error("should not be fullscreen after ExitFullscreen")
	}
}

func TestHasActiveInner(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	if viewports.HasActiveInner() {
		t.Error("should not have active inner initially (main is active)")
	}

	xpath_ := xpath.New("test", "item")
	viewports.activeXpath = xpath_

	if !viewports.HasActiveInner() {
		t.Error("should have active inner after setting non-main xpath")
	}

	viewports.DeselectAll()

	if viewports.HasActiveInner() {
		t.Error("should not have active inner after DeselectAll")
	}
}

func TestGetActiveInnerViewportContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	_, ok := viewports.GetActiveInnerViewportContent()
	if ok {
		t.Error("should not have content when no active inner")
	}

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("test content"), 1, 0)

	viewports.activeXpath = xpath_

	content, ok := viewports.GetActiveInnerViewportContent()
	if !ok {
		t.Fatal("should have content with active inner")
	}

	if len(content) == 0 || string(content[0]) != "test content" {
		t.Errorf("content = %q, want 'test content'", string(content[0]))
	}
}

func TestGetActiveInnerViewportXpath(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	if viewports.GetActiveInnerViewportXpath().String() != "" {
		t.Error("should return empty xpath when no active inner")
	}

	xpath_ := xpath.New("test", "item")
	viewports.activeXpath = xpath_

	if viewports.GetActiveInnerViewportXpath() != xpath_ {
		t.Errorf("xpath = %v, want %v", viewports.GetActiveInnerViewportXpath(), xpath_)
	}
}

func TestGetViewportContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)

	content := viewports.GetViewportContent(xpath_)
	if len(content) == 0 || string(content[0]) != "content" {
		t.Errorf("content = %q, want 'content'", string(content[0]))
	}

	missing := viewports.GetViewportContent(xpath.New("missing"))
	if len(missing) != 0 {
		t.Errorf("missing content = %v, want empty", missing)
	}
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

	if viewports.IsFullscreen() {
		t.Error("should not be fullscreen after reset")
	}

	if viewports.activeXpath != viewports.mainXpath {
		t.Error("activeXpath should be main after reset")
	}

	content := viewports.GetViewportContent(xp1)
	if len(content) != 0 {
		t.Error("viewport should be removed after reset")
	}
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
	if len(content) != 0 {
		t.Error("viewport should be removed")
	}

	if viewports.activeXpath != viewports.mainXpath {
		t.Error("activeXpath should be main after removing active viewport")
	}
}

func TestGetOrCreateViewportVersioned(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("line1\nline2\nline3"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	if !strings.Contains(output, "╭") && !strings.Contains(output, "│") {
		t.Error("versioned viewport should have border")
	}

	result = viewports.RenderViewportVersioned(xpath_, splitLines("line1\nline2\nline3"), 1, 0)
	output2 := buffer.LinesBufToStringForTests(result)

	if output != output2 {
		t.Error("cached output should match")
	}
}

func TestGetOrCreateLabelViewport(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "label")
	result := viewports.RenderLabelViewport(xpath_, splitLines("label text"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	if strings.Contains(output, "╭") || strings.Contains(output, "│") {
		t.Error("label viewport should not have border")
	}

	if !strings.Contains(output, "\x1b[") {
		t.Error("should contain ANSI zone marker")
	}
}

func TestGetOrCreateMainViewport(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	content := strings.Repeat("line\n", 100)
	result := viewports.RenderMainViewport(splitLinesBuf(content), 1, 5)
	output := buffer.LinesBufToStringForTests(result)

	if !strings.Contains(output, "█") && !strings.Contains(output, "░") {
		t.Error("main viewport should have scrollbar when content overflows")
	}

	if !strings.Contains(output, "\x1b[") {
		t.Error("should contain ANSI zone marker")
	}
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

	if !strings.Contains(output, "╭") || !strings.Contains(output, "╮") {
		t.Error("fullscreen viewport MUST have border (top corners)")
	}

	if !strings.Contains(output, "╰") || !strings.Contains(output, "╯") {
		t.Error("fullscreen viewport MUST have border (bottom corners)")
	}

	if !strings.Contains(output, "│") {
		t.Error("fullscreen viewport MUST have border (sides)")
	}

	if !strings.Contains(output, "█") && !strings.Contains(output, "░") {
		t.Error("fullscreen viewport should have scrollbar when content overflows")
	}

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 45 {
		t.Errorf("fullscreen height = %d lines, want 45", len(lines))
	}
}

func TestFullscreenLabel_EnablesBorder(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	border := style.NewStyle().Foreground(style.Color("1"))
	viewports := New(dims, 10, border, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "label")

	result := viewports.RenderLabelViewport(xpath_, splitLines("label content"), 1, 0)
	labelOutput := buffer.LinesBufToStringForTests(result)

	if strings.Contains(labelOutput, "╭") {
		t.Error("label viewport should not have border")
	}

	result = viewports.RenderFullscreenViewport(xpath_, splitLines("label content"), 2, 5)
	fullscreenOutput := buffer.LinesBufToStringForTests(result)

	if !strings.Contains(fullscreenOutput, "╭") {
		t.Error("fullscreen label MUST have border (top-left corner)")
	}

	if !strings.Contains(fullscreenOutput, "╮") {
		t.Error("fullscreen label MUST have border (top-right corner)")
	}

	if !strings.Contains(fullscreenOutput, "╰") {
		t.Error("fullscreen label MUST have border (bottom-left corner)")
	}

	if !strings.Contains(fullscreenOutput, "╯") {
		t.Error("fullscreen label MUST have border (bottom-right corner)")
	}

	if !strings.Contains(fullscreenOutput, "│") {
		t.Error("fullscreen label MUST have border (sides)")
	}
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

	if len(lines) == 0 {
		t.Fatal("should have rendered lines")
	}

	if !strings.Contains(lines[0], "╭") {
		t.Errorf("first line should be top border: %q", lines[0])
	}

	debugBuf := buffer.NewLinesBuf()
	viewports.Debug(debugBuf)

	debug := buffer.LinesBufToStringForTests(debugBuf)
	if !strings.Contains(debug, "80x") {
		t.Errorf("viewport should use width 80, debug: %s", debug)
	}
}

func TestVersionTracking(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")

	viewports.RenderViewportVersioned(xpath_, splitLines("content v1"), 1, 0)

	content := viewports.GetViewportContent(xpath_)
	if len(content) == 0 || string(content[0]) != "content v1" {
		t.Errorf("content = %q, want 'content v1'", string(content[0]))
	}

	viewports.RenderViewportVersioned(xpath_, splitLines("content v1 updated"), 1, 0)

	content = viewports.GetViewportContent(xpath_)
	if len(content) == 0 || string(content[0]) != "content v1" {
		t.Errorf("content should not change with same version: %q", string(content[0]))
	}

	viewports.RenderViewportVersioned(xpath_, splitLines("content v2"), 2, 0)

	content = viewports.GetViewportContent(xpath_)
	if len(content) == 0 || string(content[0]) != "content v2" {
		t.Errorf("content = %q, want 'content v2'", string(content[0]))
	}
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

	if !strings.Contains(output, "\x1b[") {
		t.Error("active viewport should have ANSI styling")
	}
}

func TestIndentation(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "indented")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 10)
	output := buffer.LinesBufToStringForTests(result)

	if output == "" {
		t.Error("indented viewport should render")
	}
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
	if cmd != nil {
		t.Error("nil Viewports should return nil cmd")
	}
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

	if !strings.Contains(debug, "Viewports: 1") {
		t.Error("debug should show viewport count")
	}

	if !strings.Contains(debug, "100x50") {
		t.Error("debug should show dimensions")
	}

	if !strings.Contains(debug, xpath_.String()) {
		t.Error("debug should show viewport xpath")
	}
}

func TestEmptyContent(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 100, Height: 50}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "empty")
	result := viewports.RenderViewportVersioned(xpath_, splitLines(""), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	if output == "" {
		t.Error("should render even with empty content")
	}
}

func TestZeroDimensions(t *testing.T) {
	t.Parallel()

	dims := &Dimensions{Width: 1, Height: 1}
	viewports := New(dims, 10, style.Style{}, style.Style{}, style.Style{})

	xpath_ := xpath.New("test", "item")
	result := viewports.RenderViewportVersioned(xpath_, splitLines("content"), 1, 0)
	output := buffer.LinesBufToStringForTests(result)

	_ = output
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
	if len(c1) == 0 || string(c1[0]) != "content1" {
		t.Error("viewport 1 should have correct content")
	}

	c2 := viewports.GetViewportContent(xp2)
	if len(c2) == 0 || string(c2[0]) != "content2" {
		t.Error("viewport 2 should have correct content")
	}

	c3 := viewports.GetViewportContent(xp3)
	if len(c3) == 0 || string(c3[0]) != "content3" {
		t.Error("viewport 3 should have correct content")
	}

	debugBuf := buffer.NewLinesBuf()
	viewports.Debug(debugBuf)

	debug := buffer.LinesBufToStringForTests(debugBuf)
	if !strings.Contains(debug, "Viewports: 3") {
		t.Error("should have 3 viewports")
	}
}
