package tree

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var testStyle = style.NewStyle().Foreground(style.Color("#6272A4"))

func viewString(n *Node) string {
	var buf []byte
	n.View(&buf)

	return string(buf)
}

func TestSingleNode(t *testing.T) {
	t.Parallel()

	got := viewString(New().Root("root"))
	want := "root"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSingleChild(t *testing.T) {
	t.Parallel()

	our := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	our.Child(New().Root("child"))

	got := viewString(our)

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

	got := viewString(our)
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

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines (root + 2 children + 4 grandchildren), got %d: %q", len(lines), got)
	}
}

func TestNoStyle(t *testing.T) {
	t.Parallel()

	our := New().Root("root")
	our.Child(New().Root("child"))

	got := viewString(our)
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

	got := viewString(our)
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

	got := viewString(our)
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

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines, got %d: %q", len(lines), got)
	}
}

func TestViewReuse(t *testing.T) {
	t.Parallel()

	tree := New().Root("root").EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	tree.Child(New().Root("a"))
	tree.Child(New().Root("b"))

	var buf []byte

	for iteration := range 3 {
		tree.View(&buf)

		got := string(buf)
		if !strings.Contains(got, "root") || !strings.Contains(got, "a") {
			t.Errorf("iteration %d: unexpected output: %q", iteration, got)
		}

		lines := strings.Split(got, "\n")
		if len(lines) != 3 {
			t.Errorf("iteration %d: expected 3 lines, got %d: %q", iteration, len(lines), got)
		}
	}
}
