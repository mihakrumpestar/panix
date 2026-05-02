package render

import (
	"bytes"
	"fmt"
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
		buf = NewCellBuf(width, height)
		buf.WriteANSIString(0, 0, content)
		diffs := Diff(buf, prevBuf)

		w.Reset()
		w.WriteDiff(diffs, buf)
		prevBuf.copyFrom(buf)
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

// --- Diff ---

func BenchmarkDiffNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	b.ResetTimer()

	for b.Loop() {
		Diff(buf, prevBuf)
	}
}

func BenchmarkDiffPartialChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)
	prevBuf.copyFrom(buf)

	changeContent := "\x1b[31mCHANGED\x1b[0m line with different content here"

	b.ResetTimer()

	for b.Loop() {
		buf.WriteANSIString(0, 25, changeContent)
		Diff(buf, prevBuf)
		prevBuf.copyFrom(buf)
	}
}

func BenchmarkDiffFullChange(b *testing.B) {
	width, height := 200, 50
	content1 := makeANSIContent(width, height)
	content2 := makePlainContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content1)

	prevBuf := NewCellBuf(width, height)
	prevBuf.WriteANSIString(0, 0, content2)

	b.ResetTimer()

	for b.Loop() {
		Diff(buf, prevBuf)
	}
}

// --- Zone scanning vs zone marker approach ---

func BenchmarkZoneMark(b *testing.B) {
	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = Mark("test-zone", content)
	}
}

func BenchmarkBubbleteaZoneScan(b *testing.B) {
	marked := makeZoneContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = zone.Scan(marked)
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

// --- Writer output generation ---

func BenchmarkWriterDiff(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)
	buf.WriteANSIString(0, 0, content)

	prevBuf := NewCellBuf(width, height)

	changeContent := "\x1b[31mCHANGED\x1b[0m line with different content here"
	buf.WriteANSIString(0, 25, changeContent)

	diffs := Diff(buf, prevBuf)

	var writer bytes.Buffer

	w := NewWriter(&writer)

	b.ResetTimer()

	for b.Loop() {
		w.Reset()
		w.WriteDiff(diffs, buf)
		writer.Reset()
	}
}

// --- Buffer reuse vs fresh allocation ---

func BenchmarkCellBufReuse(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	buf := NewCellBuf(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf.Clear()
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkCellBufFreshAlloc(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		buf := NewCellBuf(width, height)
		buf.WriteANSIString(0, 0, content)
	}
}

func BenchmarkBubbleteaBufferAlloc(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		ss := uv.NewStyledString(content)
		ss.Draw(screen, screen.Bounds())
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

func BenchmarkLipglossStyleRender(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Bold(true)

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render("test content")
	}
}
