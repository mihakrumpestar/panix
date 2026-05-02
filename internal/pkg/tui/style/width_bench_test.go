package style

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func BenchmarkWidth_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkLipglossWidth_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func BenchmarkWidth_WithANSI(b *testing.B) {
	input := "\x1b[38;2;255;121;198m📋BUILD\x1b[m"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkLipglossWidth_WithANSI(b *testing.B) {
	input := "\x1b[38;2;255;121;198m📋BUILD\x1b[m"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func BenchmarkWidth_Emoji(b *testing.B) {
	input := "📁 📦 💻 📋 ⚙ ✗"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkLipglossWidth_Emoji(b *testing.B) {
	input := "📁 📦 💻 📋 ⚙ ✗"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func BenchmarkWidth_CommandNumber(b *testing.B) {
	input := "1 "

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func BenchmarkLipglossWidth_CommandNumber(b *testing.B) {
	input := "1 "

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func BenchmarkHeight_SingleLine(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = CountLines(input)
	}
}

func BenchmarkLipglossHeight_SingleLine(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Height(input)
	}
}

func BenchmarkHeight_MultiLine(b *testing.B) {
	input := "line1\nline2\nline3\nline4\nline5"

	b.ResetTimer()

	for b.Loop() {
		_ = CountLines(input)
	}
}

func BenchmarkLipglossHeight_MultiLine(b *testing.B) {
	input := "line1\nline2\nline3\nline4\nline5"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Height(input)
	}
}

func BenchmarkHeight_LongMultiLine(b *testing.B) {
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

func BenchmarkLipglossHeight_LongMultiLine(b *testing.B) {
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
