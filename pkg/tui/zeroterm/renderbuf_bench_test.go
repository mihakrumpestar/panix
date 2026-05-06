package zeroterm

import (
	"fmt"
	"strings"
	"testing"
)

func makeTestContent(lines, width int) string {
	var builder strings.Builder

	for lineIdx := range lines {
		if lineIdx > 0 {
			builder.WriteByte('\n')
		}

		fmt.Fprintf(&builder, "\x1b[1;34m%-*d\x1b[0m \x1b[32mOK\x1b[0m content here", width-20, lineIdx)
	}

	return builder.String()
}

func Benchmark__RenderBufferReset(b *testing.B) {
	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 100)

	content := makeTestContent(100, 120)
	renderBuf.WriteString(content)

	b.ResetTimer()

	for b.Loop() {
		renderBuf.Reset()
	}
}

func Benchmark__RenderBufferWriteLine(b *testing.B) {
	line := []byte("\x1b[1;34mtest line content here\x1b[0m")

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 100)

	b.ResetTimer()

	for b.Loop() {
		renderBuf.Reset()

		for range 100 {
			renderBuf.WriteLine(line)
		}

		_ = renderBuf.Lines()
	}
}

func Benchmark__RenderBufferWriteString(b *testing.B) {
	content := makeTestContent(50, 120)

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 64)

	b.ResetTimer()

	for b.Loop() {
		renderBuf.Reset()
		renderBuf.WriteString(content)
		_ = renderBuf.Lines()
	}
}

func Benchmark__RenderBufferWriteStringSmall(b *testing.B) {
	content := makeTestContent(24, 80)

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 32)

	b.ResetTimer()

	for b.Loop() {
		renderBuf.Reset()
		renderBuf.WriteString(content)
		_ = renderBuf.Lines()
	}
}

func Benchmark__RenderBufferWriteStringLarge(b *testing.B) {
	content := makeTestContent(200, 200)

	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 256)

	b.ResetTimer()

	for b.Loop() {
		renderBuf.Reset()
		renderBuf.WriteString(content)
		_ = renderBuf.Lines()
	}
}

func Benchmark__RenderBufferLines(b *testing.B) {
	var renderBuf RenderBuffer

	renderBuf.lines = make([][]byte, 0, 100)

	content := makeTestContent(100, 120)
	renderBuf.WriteString(content)

	b.ResetTimer()

	for b.Loop() {
		_ = renderBuf.Lines()
	}
}
