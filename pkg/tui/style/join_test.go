package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func strsToBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}

	return out
}

func bufJoinLines(buf *buffer.LinesBuf) string {
	if buf.Len() == 0 {
		return ""
	}

	parts := make([]string, buf.Len())
	for i := range buf.Len() {
		parts[i] = string(buf.Line(i))
	}

	return strings.Join(parts, "\n")
}

func TestJoinHorizontal_Equivalence(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := newANSIStyle(sty)

	ansiStr := func(s string) string {
		b := buffer.NewLinesBuf()
		ansi.render(b, [][]byte{[]byte(s)})
		line := string(b.Line(0))
		b.Release()

		return line
	}

	cases := []struct {
		name string
		pos  Position
		strs []string
	}{
		{"Top_SingleLines", Top, []string{"📋 ", "BUILD", " (1.23s)"}},
		{"Top_DiffHeight", Top, []string{"📋 \n   ", "flake1\nflake2\nflake3", " (1.23s)"}},
		{"Top_WithANSI", Top, []string{ansiStr("📋 "), ansiStr("flake1\nflake2\nflake3"), ansiStr(" (1.23s)")}},
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

			buf := buffer.NewLinesBuf()
			JoinHorizontal(buf, testCase.pos, strsToBytes(testCase.strs)...)
			got := bufJoinLines(buf)
			buf.Release()

			if expected != got {
				t.Errorf("Mismatch for %s:\n  expected: %q\n  got:      %q", testCase.name, expected, got)
			}
		})
	}
}

func TestJoinHorizontal_EmptyBlocks(t *testing.T) {
	t.Parallel()

	buf := buffer.NewLinesBuf()
	JoinHorizontal(buf, Top)

	if buf.Len() != 0 {
		t.Errorf("JoinHorizontal() = %d lines, want 0", buf.Len())
	}

	buf.Release()

	buf = buffer.NewLinesBuf()
	JoinHorizontal(buf, Top, []byte("hello"))

	if buf.Len() != 1 || string(buf.Line(0)) != "hello" {
		t.Errorf("JoinHorizontal(single) = %v, want [hello]", buf.Line(0))
	}

	buf.Release()
}

func strsToLineBufs(ss []string) []*buffer.LinesBuf {
	out := make([]*buffer.LinesBuf, len(ss))
	for i, s := range ss {
		lb := buffer.NewLinesBuf()
		for line := range strings.SplitSeq(s, "\n") {
			lb.WriteLine([]byte(line))
		}

		out[i] = lb
	}

	return out
}

func releaseLineBufs(bufs []*buffer.LinesBuf) {
	for _, lb := range bufs {
		lb.Release()
	}
}

func TestJoinHorizontalBufs_Equivalence(t *testing.T) {
	t.Parallel()

	sty := NewStyle().Foreground(Color("#8BE9FD"))
	ansi := newANSIStyle(sty)

	ansiStr := func(s string) string {
		b := buffer.NewLinesBuf()
		ansi.render(b, [][]byte{[]byte(s)})
		line := string(b.Line(0))
		b.Release()

		return line
	}

	cases := []struct {
		name string
		pos  Position
		strs []string
	}{
		{"Top_SingleLines", Top, []string{"📋 ", "BUILD", " (1.23s)"}},
		{"Top_DiffHeight", Top, []string{"📋 \n   ", "flake1\nflake2\nflake3", " (1.23s)"}},
		{"Top_WithANSI", Top, []string{ansiStr("📋 "), ansiStr("flake1\nflake2\nflake3"), ansiStr(" (1.23s)")}},
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

			blocks := strsToLineBufs(testCase.strs)
			defer releaseLineBufs(blocks)

			buf := buffer.NewLinesBuf()
			JoinHorizontalBufs(buf, testCase.pos, blocks...)
			got := bufJoinLines(buf)
			buf.Release()

			if expected != got {
				t.Errorf("Mismatch for %s:\n  expected: %q\n  got:      %q", testCase.name, expected, got)
			}
		})
	}
}

func TestJoinHorizontalBufs_EmptyBlocks(t *testing.T) {
	t.Parallel()

	buf := buffer.NewLinesBuf()
	JoinHorizontalBufs(buf, Top)

	if buf.Len() != 0 {
		t.Errorf("JoinHorizontalBufs() = %d lines, want 0", buf.Len())
	}

	buf.Release()

	single := buffer.NewLinesBuf()
	single.WriteLine([]byte("hello"))

	buf = buffer.NewLinesBuf()
	JoinHorizontalBufs(buf, Top, single)

	if buf.Len() != 1 || string(buf.Line(0)) != "hello" {
		t.Errorf("JoinHorizontalBufs(single) = %v, want [hello]", buf.Line(0))
	}

	buf.Release()
	single.Release()
}
