package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func Benchmark__Wrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"
	contentBuf := buffer.NewLinesBuf()
	contentBuf.WriteLine([]byte(input))

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		WrapBuf(buf, contentBuf, 20, "")
	}

	buf.Release()
	contentBuf.Release()
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
	contentBuf := buffer.NewLinesBuf()
	contentBuf.WriteLine([]byte(input))

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		WrapBuf(buf, contentBuf, 80, "")
	}

	buf.Release()
	contentBuf.Release()
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

	input := make([]byte, 0, len(rendered0)+len([]byte("nix build .#nixosConfigurations.machine "))+len(rendered1))
	input = append(input, rendered0...)
	input = append(input, []byte("nix build .#nixosConfigurations.machine ")...)
	input = append(input, rendered1...)

	contentBuf := buffer.NewLinesBuf()
	contentBuf.WriteLine(input)

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		WrapBuf(buf, contentBuf, 40, "")
	}

	buf.Release()
	contentBuf.Release()
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

	contentBuf := buffer.NewLinesBuf()
	for _, l := range lines {
		contentBuf.WriteLine([]byte(l))
	}

	buf := buffer.NewLinesBuf()

	b.ResetTimer()

	for b.Loop() {
		WrapBuf(buf, contentBuf, 20, "")
	}

	buf.Release()
	contentBuf.Release()
}

func Benchmark_Lipgloss__Wrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}
