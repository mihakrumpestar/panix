package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestANSIStyle_Equivalence(t *testing.T) {
	t.Parallel()

	styles := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F1FA8C")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F1FA8C")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")),
	}

	tests := []string{
		"hello",
		"line1\nline2\nline3",
		"",
		"emoji 📦 here",
		"📁 flake1",
		"long line: " + strings.Repeat("x", 200),
	}

	for _, lgStyle := range styles {
		ansi := NewANSIStyle(lgStyle)

		for _, tc := range tests {
			expected := lgStyle.Render(tc)
			got := ansi.Render(tc)

			if expected != got {
				t.Errorf("Mismatch for %q:\n  expected: %q\n  got:      %q", tc, expected, got)
			}
		}
	}
}
