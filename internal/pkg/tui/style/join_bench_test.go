package style

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func BenchmarkJoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := "📋 "
	label := "BUILD"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkLipglossJoinHorizontal_Top_SameHeight(b *testing.B) {
	icon := "📋 "
	label := "BUILD"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func BenchmarkJoinHorizontal_Top_DiffHeight(b *testing.B) {
	icon := "📋 \n   "
	label := "flake1\nflake2\nflake3"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkLipglossJoinHorizontal_Top_DiffHeight(b *testing.B) {
	icon := "📋 \n   "
	label := "flake1\nflake2\nflake3"
	dur := " (1.23s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}

func BenchmarkJoinHorizontal_Top_WithANSI(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	ansi := NewANSIStyle(lgStyle)
	icon := ansi.Render("📋 ")
	label := ansi.Render("flake1\nflake2\nflake3")
	dur := ansi.Render(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = JoinHorizontal(Top, icon, label, dur)
	}
}

func BenchmarkLipglossJoinHorizontal_Top_WithANSI(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	icon := lgStyle.Render("📋 ")
	label := lgStyle.Render("flake1\nflake2\nflake3")
	dur := lgStyle.Render(" (1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.JoinHorizontal(lipgloss.Top, icon, label, dur)
	}
}
