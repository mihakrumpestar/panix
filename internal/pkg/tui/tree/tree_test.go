package tree

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

var testStyle = style.NewStyle().Foreground(style.Color("#6272A4"))

func TestSingleNode(t *testing.T) {
	t.Parallel()

	got := New().Root("root").String()
	want := "root"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSingleChild(t *testing.T) {
	t.Parallel()

	our := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	our.Child(New().Root("child"))

	got := our.String()

	if !strings.Contains(got, "root") || !strings.Contains(got, "child") {
		t.Errorf("tree doesn't contain expected nodes: %q", got)
	}

	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestMultipleChildren(t *testing.T) {
	t.Parallel()

	our := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b", "c"} {
		our.Child(New().Root(s))
	}

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d: %q", len(lines), got)
	}
}

func TestNested(t *testing.T) {
	t.Parallel()

	our := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b"} {
		child := New().Root(s)
		for _, s2 := range []string{"x", "y"} {
			child.Child(New().Root(s2))
		}

		our.Child(child)
	}

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines (root + 2 children + 4 grandchildren), got %d: %q", len(lines), got)
	}
}

func TestNoStyle(t *testing.T) {
	t.Parallel()

	our := New().Root("root")
	our.Child(New().Root("child"))

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestDeepNesting(t *testing.T) {
	t.Parallel()

	our := New().Root("r").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	{
		a := New().Root("a")
		a.Child(New().Root("a1"))
		a.Child(New().Root("a2"))
		our.Child(a)

		b := New().Root("b")
		b1 := New().Root("b1")
		b1.Child(New().Root("b1a"))
		b.Child(b1)
		our.Child(b)
	}

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines, got %d: %q", len(lines), got)
	}
}

func TestMixedStringAndNodeChildren(t *testing.T) {
	t.Parallel()

	our := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	our.ChildString("string child")
	our.Child(New().Root("node child"))

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), got)
	}
}

func TestMultiline(t *testing.T) {
	t.Parallel()

	sty := style.NewStyle().Foreground(style.Color("#6272A4"))
	our := New().Root("r").EnumeratorStyle(sty).IndenterStyle(sty)
	our.ChildString("line1\nline2\nline3")

	a := New().Root("a")
	our.Child(a)
	a.ChildString("child-line1\nchild-line2")

	got := our.String()
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines, got %d: %q", len(lines), got)
	}
}

func BenchmarkSimpleTree(b *testing.B) { benchTree(b, 3, 3) }
func BenchmarkSimpleTreeFlat(b *testing.B)   { benchTree(b, 1, 20) }
func BenchmarkSimpleTreeDeep(b *testing.B)   { benchTree(b, 8, 2) }

func benchTree(b *testing.B, depth, breadth int) {
	b.Helper()

	var build func(d int) *Node

	build = func(d int) *Node {
		node := New().Root("node").EnumeratorStyle(testStyle).IndenterStyle(testStyle)

		if d > 0 {
			for range breadth {
				node.Child(build(d - 1))
			}
		}

		return node
	}

	tree := build(depth)

	b.ResetTimer()

	for b.Loop() {
		_ = tree.String()
	}
}
