package style

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func Benchmark__JoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := []byte("📋 ")
	label := []byte("BUILD")
	dur := []byte(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinHorizontal(buf, Top, icon, label, dur)
		buf.Release()
	}
}

func Benchmark_Lipgloss__JoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := "📋 "
	label := "BUILD"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func Benchmark__JoinHorizontal_Top_DiffHeight(b *testing.B) {
	icon := []byte("📋 \n   ")
	label := []byte("flake1\nflake2\nflake3")
	dur := []byte(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinHorizontal(buf, Top, icon, label, dur)
		buf.Release()
	}
}

func Benchmark_Lipgloss__JoinHorizontal_Top_DiffHeight(b *testing.B) {
	icon := "📋 \n   "
	label := "flake1\nflake2\nflake3"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func Benchmark__JoinHorizontal_Top_WithANSI(b *testing.B) {
	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := newANSIStyle(sty)
	renderBuf := buffer.NewLinesBuf()
	ansi.render(renderBuf, [][]byte{[]byte("📋 ")})
	icon := renderBuf.Line(0)
	ansi.render(renderBuf, [][]byte{[]byte("flake1\nflake2\nflake3")})
	label := renderBuf.Line(0)
	ansi.render(renderBuf, [][]byte{[]byte(" (1.23s)")})
	dur := renderBuf.Line(0)
	renderBuf.Release()

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinHorizontal(buf, Top, icon, label, dur)
		buf.Release()
	}
}

func Benchmark_Lipgloss__JoinHorizontal_Top_WithANSI(b *testing.B) {
	ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	icon := ls.Render("📋 ")
	label := ls.Render("flake1\nflake2\nflake3")
	dur := ls.Render(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func Benchmark__JoinVertical(b *testing.B) {
	top := []byte("\x1b[1;34mheader\x1b[0m")
	bottom := []byte("content line with some text")

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinVertical(buf, Left, top, bottom)
		buf.Release()
	}
}

func Benchmark_Lipgloss__JoinVertical(b *testing.B) {
	top := "\x1b[1;34mheader\x1b[0m"
	bottom := "content line with some text"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	}
}
