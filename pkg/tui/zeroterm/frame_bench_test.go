package zeroterm

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	zone "github.com/lrstanley/bubblezone/v2"
)

var zoneOnce sync.Once

func ensureZoneGlobal() {
	zoneOnce.Do(func() { zone.NewGlobal() })
}

//nolint:unparam
func makeANSILines(width, lines int) [][]byte {
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

	result := strings.Split(builder.String(), "\n")

	byteLines := make([][]byte, len(result))
	for i, line := range result {
		byteLines[i] = []byte(line)
	}

	return byteLines
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

	prevLines := make([][]byte, len(lines))
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
			prevLines = make([][]byte, len(lines))
		}

		copy(prevLines, lines)
	}
}

func Benchmark_Bubbletea__RenderPipe(b *testing.B) {
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

	prevLines := make([][]byte, len(lines))
	copy(prevLines, lines)

	var outBuf []byte

	outBuf = make([]byte, 0, 8192)

	b.ResetTimer()

	for b.Loop() {
		diffs := DiffLines(lines, prevLines)
		outBuf = RenderLines(outBuf[:0], diffs, lines, len(prevLines), height)
	}
}

func Benchmark_Bubbletea__RenderPipeNoChange(b *testing.B) {
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

func Benchmark_Bubbletea__RenderLinesFullChange(b *testing.B) {
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

func Benchmark_Bubbletea__RenderLinesQuarterChange(b *testing.B) {
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

func Benchmark_Bubbletea__ZoneMark(b *testing.B) {
	content := makePlainContent(200, 50)

	b.ResetTimer()

	for b.Loop() {
		_ = zone.Scan(zone.Mark("test-zone", content))
	}
}
