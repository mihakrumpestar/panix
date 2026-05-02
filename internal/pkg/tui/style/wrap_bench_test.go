package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func BenchmarkWrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 20, "")
	}
}

func BenchmarkLipglossWrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}

func BenchmarkWrap_LongParagraph(b *testing.B) {
	input := strings.Repeat("hello world foo bar baz ", 20)

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 80, "")
	}
}

func BenchmarkLipglossWrap_LongParagraph(b *testing.B) {
	input := strings.Repeat("hello world foo bar baz ", 20)

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 80, "")
	}
}

func BenchmarkWrap_WithANSI(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	ansi := NewANSIStyle(lgStyle)
	input := ansi.Render("📋 flake1 ") + "nix build .#nixosConfigurations.machine " + ansi.Render("(1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 40, "")
	}
}

func BenchmarkLipglossWrap_WithANSI(b *testing.B) {
	lgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	input := lgStyle.Render("📋 flake1 ") + "nix build .#nixosConfigurations.machine " + lgStyle.Render("(1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 40, "")
	}
}

func BenchmarkWrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 20, "")
	}
}

func BenchmarkLipglossWrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}
