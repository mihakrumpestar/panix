package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	zone "github.com/lrstanley/bubblezone/v2"
)

func init() {
	zone.NewGlobal()
}

func makeANSIContent(width, lines int) string {
	var b strings.Builder

	for i := range lines {
		switch {
		case i%10 == 0:
			fmt.Fprintf(&b, "\x1b[1;34msrc/%s\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", fmt.Sprintf("pkg%-4d", i))
		case i%3 == 0:
			fmt.Fprintf(&b, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", i%6, i)
		default:
			fmt.Fprintf(&b, "line %d: plain text with some content that is reasonably long for testing purposes here  ", i)
		}

		if i < lines-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func makePlainContent(width, lines int) string {
	var b strings.Builder
	for i := range lines {
		fmt.Fprintf(&b, "line %d: plain text with some content that is reasonably long for testing purposes here  ", i)

		if i < lines-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func makeZoneContent(width, lines int) string {
	var b strings.Builder

	for i := range lines {
		line := fmt.Sprintf("line %d: plain text with some content that is reasonably long for testing purposes here  ", i)
		b.WriteString(zone.Mark(fmt.Sprintf("zone-%d", i), line))

		if i < lines-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// mouseMsg implements tea.MouseMsg for bubblezone InBounds.
type mouseMsg struct{ x, y int }

func (m mouseMsg) Mouse() tea.Mouse {
	return tea.Mouse{X: m.x, Y: m.y}
}
func (m mouseMsg) String() string { return fmt.Sprintf("mouse(%d,%d)", m.x, m.y) }

// --- Full Render Pipeline ---

func BenchmarkRenderPipe(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	prevBuf := NewCellBuf(width, height)

	var writer bytes.Buffer
	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		buf.Clear()
		buf.WriteANSIString(0, 0, content)
		diffs := Diff(buf, prevBuf)

		w.Reset()
		w.WriteDiff(diffs, buf)

		for _, d := range diffs {
			y := d.Y
			off := y * buf.width
			copy(prevBuf.cells[off:off+buf.width], buf.cells[off:off+buf.width])
			prevBuf.lineVersions[y] = buf.lineVersions[y]
		}
		prevBuf.version = buf.version
	}
}

func BenchmarkBubbleteaRenderPipe(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		ss := uv.NewStyledString(content)
		ss.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

// --- ANSI string → cells ---

func BenchmarkWriteANSIString(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf := NewCellBuf(width, height)
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkBubbleteaWriteANSIString(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		ss := uv.NewStyledString(content)
		ss.Draw(screen, screen.Bounds())
	}
}

func BenchmarkWriteANSIStringPlain(b *testing.B) {
	width, height := 200, 50
	content := makePlainContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf := NewCellBuf(width, height)
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkBubbleteaWriteANSIStringPlain(b *testing.B) {
	width, height := 200, 50
	content := makePlainContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		ss := uv.NewStyledString(content)
		ss.Draw(screen, screen.Bounds())
	}
}

// --- Diff + Write (terminal output generation) ---

func BenchmarkDiffWriteDiffNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	var writer bytes.Buffer

	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		diffs := Diff(buf, prevBuf)
		w.Reset()
		w.WriteDiff(diffs, buf)
		writer.Reset()
	}
}

func BenchmarkBubbleteaDiffWriteDiffNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	ss := uv.NewStyledString(content)
	ss.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	b.ResetTimer()

	for b.Loop() {
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

func BenchmarkDiffWriteDiffPartialChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	changeContent := "\x1b[31mCHANGED\x1b[0m line with different content here"

	var writer bytes.Buffer
	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		buf.WriteANSIString(0, 25, changeContent)
		diffs := Diff(buf, prevBuf)
		w.Reset()
		w.WriteDiff(diffs, buf)

		for _, d := range diffs {
			y := d.Y
			off := y * buf.width
			copy(prevBuf.cells[off:off+buf.width], buf.cells[off:off+buf.width])
			prevBuf.lineVersions[y] = buf.lineVersions[y]
		}
		prevBuf.version = buf.version

		writer.Reset()
	}
}

func BenchmarkBubbleteaDiffWriteDiffPartialChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	ss := uv.NewStyledString(content)
	ss.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	changeContent := "\x1b[31mCHANGED\x1b[0m line with different content here"
	changeSS := uv.NewStyledString(changeContent)

	b.ResetTimer()

	for b.Loop() {
		changeSS.Draw(screen, image.Rect(0, 25, width, 26))
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
		ss.Draw(screen, screen.Bounds())
	}
}

func BenchmarkDiffWriteDiffFullChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.WriteANSIString(0, 0, makePlainContent(width, height))

	var writer bytes.Buffer

	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		diffs := Diff(buf, prevBuf)
		w.Reset()
		w.WriteDiff(diffs, buf)
		writer.Reset()
	}
}

func BenchmarkBubbleteaDiffWriteDiffFullChange(b *testing.B) {
	width, height := 200, 50
	content1 := makeANSIContent(width, height)
	content2 := makePlainContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen1 := uv.NewScreenBuffer(width, height)
	ss1 := uv.NewStyledString(content1)
	ss1.Draw(screen1, screen1.Bounds())

	screen2 := uv.NewScreenBuffer(width, height)
	ss2 := uv.NewStyledString(content2)
	ss2.Draw(screen2, screen2.Bounds())

	renderer.Render(screen1.RenderBuffer)

	b.ResetTimer()

	for b.Loop() {
		renderer.Render(screen2.RenderBuffer)
		termBuf.Reset()
		renderer.Render(screen1.RenderBuffer)
		termBuf.Reset()
	}
}

// --- Zone marking + scanning ---

func BenchmarkZoneMark(b *testing.B) {
	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = Mark("test-zone", content)
	}
}

func BenchmarkBubbleteaZoneMark(b *testing.B) {
	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = zone.Scan(zone.Mark("test-zone", content))
	}
}

func BenchmarkZoneLookup(b *testing.B) {
	width, height := 200, 50
	buf := NewCellBuf(width, height)
	makePlainContent(width, height)

	zoneID := globalZones.GetOrCreate("test-zone")

	y := 10
	for x := range width {
		cell := buf.CellAt(x, y)
		cell.ZoneID = zoneID
		buf.SetCell(x, y, cell)
	}

	b.ResetTimer()

	for b.Loop() {
		_ = IsZoneAt(buf, 50, 10, "test-zone")
	}
}

func BenchmarkBubbleteaZoneLookup(b *testing.B) {
	marked := makeZoneContent(200, 50)
	_ = zone.Scan(marked)

	b.ResetTimer()

	for b.Loop() {
		z := zone.Get("zone-25")
		if z != nil {
			_ = z.InBounds(mouseMsg{x: 50, y: 25})
		}
	}
}

// --- Cell comparison ---

func BenchmarkCellEqual(b *testing.B) {
	c1 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 121, 198), Bg: NewColor(40, 42, 54), Attrs: AttrBold}
	c2 := Cell{Content: "A", Width: 1, Fg: NewColor(255, 121, 198), Bg: NewColor(40, 42, 54), Attrs: AttrBold}

	b.ResetTimer()

	for b.Loop() {
		_ = c1.VisualEqual(c2)
	}
}

func BenchmarkBubbleteaCellEqual(b *testing.B) {
	c1 := &uv.Cell{Content: "A", Width: 1}
	c1.Style.Fg = color.NRGBA{R: 255, G: 121, B: 198, A: 255}
	c1.Style.Bg = color.NRGBA{R: 40, G: 42, B: 54, A: 255}
	c1.Style.Attrs = uv.AttrBold

	c2 := &uv.Cell{Content: "A", Width: 1}
	c2.Style.Fg = color.NRGBA{R: 255, G: 121, B: 198, A: 255}
	c2.Style.Bg = color.NRGBA{R: 40, G: 42, B: 54, A: 255}
	c2.Style.Attrs = uv.AttrBold

	b.ResetTimer()

	for b.Loop() {
		_ = c1.Equal(c2)
	}
}

// --- Buffer allocation + content write ---

func BenchmarkBufferAllocFresh(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf := NewCellBuf(width, height)
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkBubbleteaBufferAllocFresh(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		ss := uv.NewStyledString(content)
		ss.Draw(screen, screen.Bounds())
	}
}

func BenchmarkBufferAllocReuse(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf.Clear()
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkBubbleteaBufferAllocReuse(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer
	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	ss := uv.NewStyledString(content)
	ss.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	b.ResetTimer()

	for b.Loop() {
		ss.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

// --- Style conversion (lipgloss → render.Style vs lipgloss.Style.Render) ---

func BenchmarkStyleConversion(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Bold(true)

	b.ResetTimer()

	for b.Loop() {
		_ = NewStyleFromLipgloss(lgStyle)
	}
}

func BenchmarkBubbleteaStyleConversion(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Bold(true)

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render("test content")
	}
}

// --- Render pipeline with stable content (cache-hit path) ---

// RenderPipeNoChange measures the cost of the version-skip path:
// WriteANSIString re-parses but detects all cells match (no version
// bump), so Diff returns empty and WriteDiff is skipped.
// In real usage, the model-level string cache avoids WriteANSIString
// entirely for idle frames.
func BenchmarkRenderPipeNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	var writer bytes.Buffer
	w := NewWriter(&writer)

	prevVersion := buf.Version()

	b.ResetTimer()

	for b.Loop() {
		// Same content re-rendered: cells match → version doesn't change
		// → renderFrame returns immediately after the version check.
		buf.WriteANSIString(0, 0, content)
		if buf.Version() != prevVersion {
			diffs := Diff(buf, prevBuf)
			w.Reset()
			w.WriteDiff(diffs, buf)
			for _, d := range diffs {
				y := d.Y
				off := y * buf.width
				copy(prevBuf.cells[off:off+buf.width], buf.cells[off:off+buf.width])
				prevBuf.lineVersions[y] = buf.lineVersions[y]
			}
			prevBuf.version = buf.version
		}
		writer.Reset()
	}
}

func BenchmarkBubbleteaRenderPipeNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer
	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	ss := uv.NewStyledString(content)
	ss.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	b.ResetTimer()

	for b.Loop() {
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

// --- Version-check skip (idle frame, nothing changed) ---

// RenderFrameVersionSkip measures WriteANSIString when cells match
// (version doesn't increment). This is the cost of the hot parse
// path even when nothing changed — every cell comparison succeeds.
func BenchmarkRenderFrameVersionSkip(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)
	prevVersion := buf.Version()

	b.ResetTimer()

	for b.Loop() {
		buf.WriteANSIString(0, 0, content)
		_ = buf.Version() == prevVersion
	}
}

// RenderPipeModelCacheSkip measures the model-level string cache skip:
// when the rendered string is identical to the previous frame, the
// model skips WriteANSIString entirely. This is the true idle-frame
// cost — just a string comparison (~50ns for a 10KB string).
func BenchmarkRenderPipeModelCacheSkip(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	lastStr := content

	b.ResetTimer()

	for b.Loop() {
		renderStr := content
		if renderStr == lastStr {
			continue
		}
		lastStr = renderStr
		buf.WriteANSIString(0, 0, renderStr)
		diffs := Diff(buf, prevBuf)
		for _, d := range diffs {
			y := d.Y
			off := y * buf.width
			copy(prevBuf.cells[off:off+buf.width], buf.cells[off:off+buf.width])
			prevBuf.lineVersions[y] = buf.lineVersions[y]
		}
		prevBuf.version = buf.version
	}
}

// --- Quarter-screen change (realistic TUI update pattern) ---

func BenchmarkDiffWriteDiffQuarterChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	// Change ~1/4 of lines (12-13 lines out of 50)
	changedContent := "\x1b[31mCHANGED\x1b[0m line with different content for quarter screen update testing"

	var writer bytes.Buffer
	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		for y := 0; y < height; y += 4 {
			buf.WriteANSIString(0, y, changedContent)
		}
		diffs := Diff(buf, prevBuf)
		w.Reset()
		w.WriteDiff(diffs, buf)

		for _, d := range diffs {
			y := d.Y
			off := y * buf.width
			copy(prevBuf.cells[off:off+buf.width], buf.cells[off:off+buf.width])
			prevBuf.lineVersions[y] = buf.lineVersions[y]
		}
		prevBuf.version = buf.version

		writer.Reset()
	}
}

func BenchmarkBubbleteaDiffWriteDiffQuarterChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer
	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	ss := uv.NewStyledString(content)
	ss.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	changedContent := "\x1b[31mCHANGED\x1b[0m line with different content for quarter screen update testing"
	changeSS := uv.NewStyledString(changedContent)

	b.ResetTimer()

	for b.Loop() {
		for y := 0; y < height; y += 4 {
			changeSS.Draw(screen, image.Rect(0, y, width, y+1))
		}
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
		ss.Draw(screen, screen.Bounds())
	}
}
