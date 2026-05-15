package style

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func Benchmark__Width_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth([]byte(input))
	}
}

func Benchmark_Lipgloss__Width_ASCII(b *testing.B) {
	input := "hello world"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_WithANSI(b *testing.B) {
	input := []byte("\x1b[38;2;255;121;198m📋BUILD\x1b[m")

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func Benchmark_Lipgloss__Width_WithANSI(b *testing.B) {
	input := "\x1b[38;2;255;121;198m📋BUILD\x1b[m"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_Emoji(b *testing.B) {
	input := []byte("📁 📦 💻 📋 ⚙ ✗")

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func Benchmark_Lipgloss__Width_Emoji(b *testing.B) {
	input := "📁 📦 💻 📋 ⚙ ✗"

	b.ResetTimer()

	for b.Loop() {
		_ = lipgloss.Width(input)
	}
}

func Benchmark__Width_CommandNumber(b *testing.B) {
	input := []byte("1 ")

	b.ResetTimer()

	for b.Loop() {
		_ = CellWidth(input)
	}
}

func Benchmark_Lipgloss__Width_CommandNumber(b *testing.B) {
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

func Benchmark_Lipgloss__Height_SingleLine(b *testing.B) {
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

func Benchmark_Lipgloss__Height_MultiLine(b *testing.B) {
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

func Benchmark_Lipgloss__Height_LongMultiLine(b *testing.B) {
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

// --- JoinHorizontal ---

func Benchmark__JoinHorizontal(b *testing.B) {
	left := []byte("\x1b[1;34msrc/\x1b[0m \x1b[32mpackage\x1b[0m")
	right := []byte("build output line here with some text")

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinHorizontal(buf, Left, left, right)
		buf.Release()
	}
}

func Benchmark__JoinHorizontal_MultiLine(b *testing.B) {
	var leftBuf, rightBuf []byte

	var (
		leftStrSb187  strings.Builder
		rightStrSb187 strings.Builder
	)

	for row := range 20 {
		if row > 0 {
			leftStrSb187.WriteString("\n")
			rightStrSb187.WriteString("\n")
		}

		fmt.Fprintf(&leftStrSb187, "\x1b[3%dmphase-%d\x1b[0m", row%6, row)
		fmt.Fprintf(&rightStrSb187, "output for phase %d with some content here", row)
	}

	leftBuf = []byte(leftStrSb187.String())
	rightBuf = []byte(rightStrSb187.String())

	b.ResetTimer()

	for b.Loop() {
		buf := buffer.NewLinesBuf()
		JoinHorizontal(buf, Left, leftBuf, rightBuf)
		buf.Release()
	}
}
