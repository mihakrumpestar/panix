package style

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func Benchmark__Width_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkRef_Lipgloss__Width_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_WithANSI(b *testing.B) {
	input := "\x1b[38;2;255;121;198m📋BUILD\x1b[m"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkRef_Lipgloss__Width_WithANSI(b *testing.B) {
	input := "\x1b[38;2;255;121;198m📋BUILD\x1b[m"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_Emoji(b *testing.B) {
	input := "📁 📦 💻 📋 ⚙ ✗"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkRef_Lipgloss__Width_Emoji(b *testing.B) {
	input := "📁 📦 💻 📋 ⚙ ✗"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_CommandNumber(b *testing.B) {
	input := "1 "

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkRef_Lipgloss__Width_CommandNumber(b *testing.B) {
	input := "1 "

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Height_SingleLine(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = CountLines(input)
	}
}

func BenchmarkRef_Lipgloss__Height_SingleLine(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Height(input)
	}
}

func Benchmark__Height_MultiLine(b *testing.B) {
	input := "line1\nline2\nline3\nline4\nline5"

	b.ResetTimer()

	for b.Loop() {
		_ = CountLines(input)
	}
}

func BenchmarkRef_Lipgloss__Height_MultiLine(b *testing.B) {
	input := "line1\nline2\nline3\nline4\nline5"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Height(input)
	}
}

func Benchmark__Height_LongMultiLine(b *testing.B) {
	var builder strings.Builder

	for idx := range 100 {
		if idx > 0 {
			builder.WriteByte('\n')
		}

		builder.WriteString("line " + strconv.Itoa(idx))
	}

	input := builder.String()

	b.ResetTimer()

	for b.Loop() {
		_ = CountLines(input)
	}
}

func BenchmarkRef_Lipgloss__Height_LongMultiLine(b *testing.B) {
	var builder strings.Builder

	for idx := range 100 {
		if idx > 0 {
			builder.WriteByte('\n')
		}

		builder.WriteString("line " + strconv.Itoa(idx))
	}

	input := builder.String()

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Height(input)
	}
}
