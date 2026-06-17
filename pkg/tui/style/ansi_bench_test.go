package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func Benchmark__Render_SingleLine(b *testing.B) {
	sty := NewStyle().Foreground(Color("#FF79C6"))
	ansi := newANSIStyle(sty)
	input := bytesFromLines([]string{"📋BUILD"})

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		ansi.render(buf, input)
		buf.Release()
	}
}

func Benchmark_Lipgloss__Render_SingleLine(b *testing.B) {
	lgSty := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	b.ResetTimer()

	for b.Loop() {
		_ = lgSty.Render("📋BUILD")
	}
}

func Benchmark__Render_MultiLine(b *testing.B) {
	sty := NewStyle().Foreground(Color("#FF79C6"))
	ansi := newANSIStyle(sty)
	input := bytesFromLines([]string{"line1", "line2", "line3"})

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		ansi.render(buf, input)
		buf.Release()
	}
}

func Benchmark_Lipgloss__Render_MultiLine(b *testing.B) {
	lgSty := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	content := "line1\nline2\nline3"

	b.ResetTimer()

	for b.Loop() {
		_ = lgSty.Render(content)
	}
}

func Benchmark__Render_LongLine(b *testing.B) {
	sty := NewStyle().Foreground(Color("#FF79C6"))
	ansi := newANSIStyle(sty)
	input := bytesFromLines([]string{strings.Repeat("x", 200)})

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		ansi.render(buf, input)
		buf.Release()
	}
}

func Benchmark_Lipgloss__Render_LongLine(b *testing.B) {
	lgSty := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	content := strings.Repeat("x", 200)

	b.ResetTimer()

	for b.Loop() {
		_ = lgSty.Render(content)
	}
}

func Benchmark__Render_WithEmoji(b *testing.B) {
	sty := NewStyle().Foreground(Color("#8BE9FD")).Bold(true)
	ansi := newANSIStyle(sty)
	input := bytesFromLines([]string{"📁 flake1  (0.50s)"})

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		ansi.render(buf, input)
		buf.Release()
	}
}

func Benchmark_Lipgloss__Render_WithEmoji(b *testing.B) {
	lgSty := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	content := "📁 flake1  (0.50s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lgSty.Render(content)
	}
}

// --- RenderLine benchmarks ---

func Benchmark__RenderLine_Colored(b *testing.B) {
	sty := NewStyle().Foreground(Color("#FF79C6"))
	line := []byte("📋BUILD")

	b.ResetTimer()

	for b.Loop() {
		_ = sty.RenderLine(line)
	}
}

func Benchmark__RenderLine_NoStyle(b *testing.B) {
	sty := NewStyle()
	line := []byte("simple text")

	b.ResetTimer()

	for b.Loop() {
		_ = sty.RenderLine(line)
	}
}

func Benchmark__RenderLine_ColoredBold(b *testing.B) {
	sty := NewStyle().Foreground(Color("#FF79C6")).Bold(true)
	line := []byte("📋 flake1  (0.50s)")

	b.ResetTimer()

	for b.Loop() {
		_ = sty.RenderLine(line)
	}
}
