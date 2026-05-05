package zeroterm

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	zone "github.com/lrstanley/bubblezone/v2"
)

var zoneOnce sync.Once

func ensureZoneGlobal() {
	zoneOnce.Do(func() { zone.NewGlobal() })
}

//nolint:unparam
func makeANSILines(width, lines int) []string {
	var builder strings.Builder

	for lineIdx := range lines {
		switch {
		case lineIdx%10 == 0:
			fmt.Fprintf(&builder, "\x1b[1;34msrc/pkg%-4d\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line", lineIdx)
		case lineIdx%3 == 0:
			fmt.Fprintf(&builder, "\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width", lineIdx%6, lineIdx)
		default:
			fmt.Fprintf(&builder, "line %d: plain text with some content that is reasonably long for testing purposes here  ", lineIdx)
		}

		if lineIdx < lines-1 {
			builder.WriteByte('\n')
		}
	}

	return strings.Split(builder.String(), "\n")
}

//nolint:unparam
func makeANSIContent(width, lines int) string {
	var builder strings.Builder

	for lineIdx := range lines {
		switch {
		case lineIdx%10 == 0:
			fmt.Fprintf(&builder,
				"\x1b[1;34msrc/%s\x1b[0m \x1b[32mOK\x1b[0m package with a longer description that fills the line",
				fmt.Sprintf("pkg%-4d", lineIdx),
			)
		case lineIdx%3 == 0:
			fmt.Fprintf(&builder,
				"\x1b[3%dmline %d: colored text with escape sequences\x1b[0m and plain suffix to fill width",
				lineIdx%6, lineIdx,
			)
		default:
			fmt.Fprintf(&builder,
				"line %d: plain text with some content that is reasonably long for testing purposes here  ",
				lineIdx,
			)
		}

		if lineIdx < lines-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

//nolint:unparam
func makePlainContent(width, lines int) string {
	var builder strings.Builder
	for lineIdx := range lines {
		fmt.Fprintf(&builder, "line %d: plain text with some content that is reasonably long for testing purposes here  ", lineIdx)

		if lineIdx < lines-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

// --- Full Render Pipeline ---

func Benchmark__RenderPipe(b *testing.B) {
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

func BenchmarkRef_Bubbletea__RenderPipe(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	b.ResetTimer()

	for b.Loop() {
		screen := uv.NewScreenBuffer(width, height)
		styledStr := uv.NewStyledString(content)
		styledStr.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

// --- No change (cache hit at model level) ---

func Benchmark__RenderPipeNoChange(b *testing.B) {
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

func BenchmarkRef_Bubbletea__RenderPipeNoChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	styledContent := uv.NewStyledString(content)
	styledContent.Draw(screen, screen.Bounds())
	renderer.Render(screen.RenderBuffer)

	b.ResetTimer()

	for b.Loop() {
		renderer.Render(screen.RenderBuffer)
		termBuf.Reset()
	}
}

// --- DiffLines only ---

func Benchmark__DiffLinesNoChange(b *testing.B) {
	width, height := 200, 50
	lines := makeANSILines(width, height)

	prevLines := make([]string, len(lines))
	copy(prevLines, lines)

	b.ResetTimer()

	for b.Loop() {
		DiffLines(lines, prevLines)
	}
}

func Benchmark__DiffLinesPartialChange(b *testing.B) {
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

func Benchmark__RenderLinesFullChange(b *testing.B) {
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

func BenchmarkRef_Bubbletea__RenderLinesFullChange(b *testing.B) {
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

func Benchmark__RenderLinesQuarterChange(b *testing.B) {
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

func BenchmarkRef_Bubbletea__RenderLinesQuarterChange(b *testing.B) {
	width, height := 200, 50
	content := makeANSIContent(width, height)

	var termBuf bytes.Buffer

	renderer := uv.NewTerminalRenderer(&termBuf, []string{"TERM=xterm-256color"})
	renderer.SetScrollOptim(true)

	screen := uv.NewScreenBuffer(width, height)
	styledContent := uv.NewStyledString(content)
	styledContent.Draw(screen, screen.Bounds())
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
		styledContent.Draw(screen, screen.Bounds())
	}
}

// --- Zone ---

func Benchmark__ZoneMark(b *testing.B) {
	ensureZoneGlobal()

	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = Mark("test-zone", content)
	}
}

func BenchmarkRef_Bubbletea__ZoneMark(b *testing.B) {
	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = zone.Scan(zone.Mark("test-zone", content))
	}
}

func Benchmark__ZoneAtLine(b *testing.B) {
	line := "\x1b[5z\x1b[1;31mBold Red\x1b[0m some text \x1b[/5z\x1b[32mGreen\x1b[0m"

	b.ResetTimer()

	for b.Loop() {
		ZoneAtLine(line, 5)
	}
}

type mouseMsg struct{ x, y int }

func (m mouseMsg) Mouse() tea.Mouse { return tea.Mouse{X: m.x, Y: m.y} }
func (m mouseMsg) String() string   { return fmt.Sprintf("mouse(%d,%d)", m.x, m.y) }

func BenchmarkRef_Bubbletea__ZoneAtLine(b *testing.B) {
	ensureZoneGlobal()

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

//nolint:unparam
func makeZoneContent(width, lines int) string {
	var builder strings.Builder

	for lineIdx := range lines {
		line := fmt.Sprintf("line %d: plain text with some content that is reasonably long for testing purposes here  ", lineIdx)
		builder.WriteString(zone.Mark(fmt.Sprintf("zone-%d", lineIdx), line))

		if lineIdx < lines-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}
