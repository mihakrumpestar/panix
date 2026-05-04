package zeroterm

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	zone "github.com/lrstanley/bubblezone/v2"
)

func init() {
	zone.NewGlobal()
}

func makeANSILines(width, lines int) []string {
	var b strings.Builder

	for i := range lines {
		switch {
		case i%10 == 0:
			fmt.Fprintf(&b, "\x1b[1;34msrc/pkg%-4d\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", i)
		case i%3 == 0:
			fmt.Fprintf(&b, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", i%6, i)
		default:
			fmt.Fprintf(&b, "line %d: plain text with some content that is reasonably long for testing purposes here  ", i)
		}

		if i < lines-1 {
			b.WriteByte('\n')
		}
	}

	return strings.Split(b.String(), "\n")
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

// --- Full Render Pipeline ---

func BenchmarkRenderPipe(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	prevLines := make([]string, len(lines))
	copy(prevLines, lines)

	var outBuf []byte

	outBuf = make([]byte, 0, 8192)

	b.ResetTimer()

	for b.Loop() {
		diffs := DiffLines(lines, prevLines)
		outBuf = RenderLines(outBuf[:0], diffs, lines, len(prevLines), height)

		if cap(prevLines) >= len(lines) {
			prevLines = prevLines[:len(lines)]
		} else {
			prevLines = make([]string, len(lines))
		}

		copy(prevLines, lines)
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

// --- No change (cache hit at model level) ---

func BenchmarkRenderPipeNoChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	prevLines := make([]string, len(lines))
	copy(prevLines, lines)

	var outBuf []byte

	outBuf = make([]byte, 0, 8192)

	b.ResetTimer()

	for b.Loop() {
		diffs := DiffLines(lines, prevLines)
		outBuf = RenderLines(outBuf[:0], diffs, lines, len(prevLines), height)
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

// --- DiffLines only ---

func BenchmarkDiffLinesNoChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	prevLines := make([]string, len(lines))
	copy(prevLines, lines)

	b.ResetTimer()

	for b.Loop() {
		DiffLines(lines, prevLines)
	}
}

func BenchmarkDiffLinesPartialChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	prevLines := make([]string, len(lines))
	copy(prevLines, lines)

	changedLine := "\x1b[31mCHANGED\x1b[0m line with different content here"

	b.ResetTimer()

	for b.Loop() {
		old := prevLines[25]
		prevLines[25] = changedLine
		DiffLines(lines, prevLines)
		prevLines[25] = old
	}
}

// --- RenderLines only (full change) ---

func BenchmarkRenderLinesFullChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	diffs := make([]ChangedLine, height)
	for i := range diffs {
		diffs[i] = ChangedLine{Y: i}
	}

	var outBuf []byte

	outBuf = make([]byte, 0, 8192)

	b.ResetTimer()

	for b.Loop() {
		outBuf = RenderLines(outBuf[:0], diffs, lines, height, height)
	}
}

func BenchmarkBubbleteaRenderLinesFullChange(b *testing.B) {
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

// --- RenderLines only (partial change, ~12 lines) ---

func BenchmarkRenderLinesQuarterChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	var diffs []ChangedLine
	for y := 0; y < height; y += 4 {
		diffs = append(diffs, ChangedLine{Y: y})
	}

	var outBuf []byte

	outBuf = make([]byte, 0, 8192)

	b.ResetTimer()

	for b.Loop() {
		outBuf = RenderLines(outBuf[:0], diffs, lines, height, height)
	}
}

func BenchmarkBubbleteaRenderLinesQuarterChange(b *testing.B) {
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

// --- Zone ---

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

func BenchmarkZoneAtLine(b *testing.B) {
	line := "\x1b[5z\x1b[1;31mBold Red\x1b[0m some text \x1b[/5z\x1b[32mGreen\x1b[0m"

	b.ResetTimer()

	for b.Loop() {
		ZoneAtLine(line, 5)
	}
}

type mouseMsg struct{ x, y int }

func (m mouseMsg) Mouse() tea.Mouse { return tea.Mouse{X: m.x, Y: m.y} }
func (m mouseMsg) String() string   { return fmt.Sprintf("mouse(%d,%d)", m.x, m.y) }

func BenchmarkBubbleteaZoneAtLine(b *testing.B) {
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
