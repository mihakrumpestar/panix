package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func TestAnsiStyle_Equivalence(t *testing.T) {
	t.Parallel()

	colors := []string{"#F1FA8C", "#50FA7B", "#FF5555", "#8BE9FD"}
	bolds := []bool{false, true}

	tests := []string{
		"hello",
		"line1\nline2\nline3",
		"emoji 📦 here",
		"📁 flake1",
		"long line: " + strings.Repeat("x", 200),
	}

	for _, color := range colors {
		for _, bold := range bolds {
			lgSty := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			if bold {
				lgSty = lgSty.Bold(true)
			}

			sty := NewStyle().Foreground(Color(color))
			if bold {
				sty = sty.Bold(true)
			}

			ansi := newANSIStyle(sty)

			for _, input := range tests {
				expected := lgSty.Render(input)
				inputBytes := bytesFromLines(strings.Split(input, "\n"))

				buf := buffer.NewLinesBuf()
				ansi.render(buf, inputBytes)
				got := buf.Lines()

				expectedVisible := string(StripANSI([]byte(expected)))
				gotVisible := string(StripANSI(bytesToSingleByte(got)))

				if expectedVisible != gotVisible {
					t.Errorf("Visible content mismatch for %q (color=%s bold=%v):\n  expected: %q\n  got:      %q", input, color, bold, expectedVisible, gotVisible)
				}

				if input != "" && !strings.Contains(string(bytesToSingleByte(got)), "\x1b[") {
					t.Errorf("Missing ANSI sequences for %q (color=%s bold=%v)", input, color, bold)
				}

				buf.Release()
			}
		}
	}
}

func TestAnsiStyle_EmptyString(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#F1FA8C"))
	ansi := newANSIStyle(sty)

	buf := buffer.NewLinesBuf()
	ansi.render(buf, nil)

	if buf.Len() != 0 {
		t.Errorf("ansiStyle.render(nil) = %d lines, want 0", buf.Len())
	}

	buf.Release()
}

func bytesFromLines(lines []string) [][]byte {
	out := make([][]byte, len(lines))
	for i, l := range lines {
		out[i] = []byte(l)
	}

	return out
}

func bytesToSingleByte(lines [][]byte) []byte {
	if len(lines) == 0 {
		return nil
	}

	size := 0
	for _, l := range lines {
		size += len(l) + 1
	}

	buf := make([]byte, 0, size)

	for i, l := range lines {
		if i > 0 {
			buf = append(buf, '\n')
		}

		buf = append(buf, l...)
	}

	return buf
}
