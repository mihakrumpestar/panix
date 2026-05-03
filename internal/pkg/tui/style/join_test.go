package style

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestJoinHorizontal_Equivalence(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := NewANSIStyle(sty)

	cases := []struct {
		name string
		pos  Position
		strs []string
	}{
		{"Top_SingleLines", Top, []string{"📋 ", "BUILD", " (1.23s)"}},
		{"Top_DiffHeight", Top, []string{"📋 \n   ", "flake1\nflake2\nflake3", " (1.23s)"}},
		{"Top_WithANSI", Top, []string{ansi.Render("📋 "), ansi.Render("flake1\nflake2\nflake3"), ansi.Render(" (1.23s)")}},
		{"Center_DiffHeight", Center, []string{"📋 ", "line1\nline2\nline3", " (1.23s)"}},
		{"Bottom_DiffHeight", Bottom, []string{"📋 ", "line1\nline2\nline3", " (1.23s)"}},
		{"SingleString", Top, []string{"hello"}},
		{"Empty", Top, []string{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var lgPos lipgloss.Position

			switch testCase.pos {
			case Top:
				lgPos = lipgloss.Top
			case Center:
				lgPos = lipgloss.Center
			case Bottom:
				lgPos = lipgloss.Bottom
			}

			expected := lipgloss.JoinHorizontal(lgPos, testCase.strs...)
			got := JoinHorizontal(testCase.pos, testCase.strs...)

			if expected != got {
				t.Errorf("Mismatch for %s:\n  expected: %q\n  got:      %q", testCase.name, expected, got)
			}
		})
	}
}
