package style

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func Benchmark__JoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := "📋 "
	label := "BUILD"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkRef_Lipgloss__JoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := "📋 "
	label := "BUILD"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func Benchmark__JoinHorizontal_Top_DiffHeight(b *testing.B) {
	icon := "📋 \n   "
	label := "flake1\nflake2\nflake3"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkRef_Lipgloss__JoinHorizontal_Top_DiffHeight(b *testing.B) {
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
	ansi := NewANSIStyle(sty)
	icon := ansi.Render("📋 ")
	label := ansi.Render("flake1\nflake2\nflake3")
	dur := ansi.Render(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkRef_Lipgloss__JoinHorizontal_Top_WithANSI(b *testing.B) {
	ls := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	icon := ls.Render("📋 ")
	label := ls.Render("flake1\nflake2\nflake3")
	dur := ls.Render(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}
