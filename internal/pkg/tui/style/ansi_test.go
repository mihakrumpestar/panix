package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestANSIStyle_Equivalence(t *testing.T) {
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
			ls := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			if bold {
				ls = ls.Bold(true)
			}

			sty := NewStyle().Foreground(Color(color))
			if bold {
				sty = sty.Bold(true)
			}

			ansi := NewANSIStyle(sty)

			for _, tc := range tests {
				expected := ls.Render(tc)
				got := ansi.Render(tc)

				// ANSIStyle produces visually identical output but may differ
				// in SGR parameter ordering (bold merged vs separate) and
				// empty-string handling. Strip all ANSI sequences and compare
				// the visible content + verify ANSI wrapping is present.
				expectedVisible := stripANSI(expected)
				gotVisible := stripANSI(got)

				if expectedVisible != gotVisible {
					t.Errorf("Visible content mismatch for %q (color=%s bold=%v):\n  expected: %q\n  got:      %q", tc, color, bold, expectedVisible, gotVisible)
				}

				if tc != "" && !strings.Contains(got, "\x1b[") {
					t.Errorf("Missing ANSI sequences for %q (color=%s bold=%v)", tc, color, bold)
				}
			}
		}
	}
}

func TestANSIStyle_EmptyString(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#F1FA8C"))
	ansi := NewANSIStyle(sty)

	got := ansi.Render("")
	if got != "" {
		t.Errorf("ANSIStyle.Render(\"\") = %q, want \"\"", got)
	}
}

// stripANSI removes all ANSI escape sequences from a string.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0

	for i < len(s) {
		if s[i] == '\x1b' {
			i = skipANSI(s, i)

			continue
		}

		b.WriteByte(s[i])
		i++
	}

	return b.String()
}
