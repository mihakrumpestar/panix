package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func BenchmarkRender_SingleLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	ansi := NewANSIStyle(lgStyle)

	b.ResetTimer()

	for b.Loop() {
		_ = ansi.Render("📋BUILD")
	}
}

func BenchmarkLipglossRender_SingleLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render("📋BUILD")
	}
}

func BenchmarkRender_MultiLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	ansi := NewANSIStyle(lgStyle)
	content := "line1\nline2\nline3"

	b.ResetTimer()

	for b.Loop() {
		_ = ansi.Render(content)
	}
}

func BenchmarkLipglossRender_MultiLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	content := "line1\nline2\nline3"

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render(content)
	}
}

func BenchmarkRender_LongLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	ansi := NewANSIStyle(lgStyle)
	content := strings.Repeat("x", 200)

	b.ResetTimer()

	for b.Loop() {
		_ = ansi.Render(content)
	}
}

func BenchmarkLipglossRender_LongLine(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))
	content := strings.Repeat("x", 200)

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render(content)
	}
}

func BenchmarkRender_WithEmoji(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	ansi := NewANSIStyle(lgStyle)
	content := "📁 flake1  (0.50s)"

	b.ResetTimer()

	for b.Loop() {
		_ = ansi.Render(content)
	}
}

func BenchmarkLipglossRender_WithEmoji(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	content := "📁 flake1  (0.50s)"

	b.ResetTimer()

	for b.Loop() {
		_ = lgStyle.Render(content)
	}
}
