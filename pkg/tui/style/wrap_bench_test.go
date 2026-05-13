package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func Benchmark__Wrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"
	inputBytes := [][]byte{[]byte(input)}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		Wrap(buf, inputBytes, 20, "")
	}

	buf.Release()
}

func Benchmark_Lipgloss__Wrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}

func Benchmark__Wrap_LongParagraph(b *testing.B) {
	input := strings.Repeat("hello world foo bar baz ", 20)
	inputBytes := [][]byte{[]byte(input)}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		Wrap(buf, inputBytes, 80, "")
	}

	buf.Release()
}

func Benchmark_Lipgloss__Wrap_LongParagraph(b *testing.B) {
	input := strings.Repeat("hello world foo bar baz ", 20)

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 80, "")
	}
}

func Benchmark__Wrap_WithANSI(b *testing.B) {
	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := newANSIStyle(sty)

	renderBuf := buffer.NewLinesBuf()
	ansi.render(renderBuf, [][]byte{[]byte("📋 flake1 ")})
	rendered0 := make([]byte, len(renderBuf.Line(0)))
	copy(rendered0, renderBuf.Line(0))

	ansi.render(renderBuf, [][]byte{[]byte("(1.23s)")})
	rendered1 := make([]byte, len(renderBuf.Line(0)))
	copy(rendered1, renderBuf.Line(0))
	renderBuf.Release()

	input := append(rendered0, []byte("nix build .#nixosConfigurations.machine ")...)
	input = append(input, rendered1...)
	inputBytes := [][]byte{input}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		Wrap(buf, inputBytes, 40, "")
	}

	buf.Release()
}

func Benchmark_Lipgloss__Wrap_WithANSI(b *testing.B) {
	ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	input := ls.Render("📋 flake1 ") + "nix build .#nixosConfigurations.machine " + ls.Render("(1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 40, "")
	}
}

func Benchmark__Wrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"
	lines := strings.Split(input, "\n")

	inputBytes := make([][]byte, len(lines))
	for i, l := range lines {
		inputBytes[i] = []byte(l)
	}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		Wrap(buf, inputBytes, 20, "")
	}

	buf.Release()
}

func Benchmark_Lipgloss__Wrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}
