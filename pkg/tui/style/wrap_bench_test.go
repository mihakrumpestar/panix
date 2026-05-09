package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func Benchmark__Wrap_ShortLine(b *testing.B) {
	input := "hello world foo bar"

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 20, "")
	}
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

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 80, "")
	}
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
	ansi := NewANSIStyle(sty)
	input := ansi.Render("📋 flake1 ") + "nix build .#nixosConfigurations.machine " + ansi.Render("(1.23s)")

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 40, "")
	}
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

	b.ResetTimer()

	for b.Loop() {
		_ = Wrap(input, 20, "")
	}
}

func Benchmark_Lipgloss__Wrap_MultiLine(b *testing.B) {
	input := "line1\nline2 word line3 more text here that needs wrapping"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Wrap(input, 20, "")
	}
}
