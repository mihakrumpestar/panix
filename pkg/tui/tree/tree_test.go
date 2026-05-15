package tree

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func lb(s string) *buffer.LinesBuf {
	buf := buffer.NewLinesBuf()
	for line := range strings.SplitSeq(s, "\n") {
		buf.WriteLine([]byte(line))
	}

	return buf
}

func rootStr(s string) *Node {
	return NewTree(style.NewStyle()).NewNode(lb(s))
}

func viewString(n *Node) string {
	buf := buffer.NewLinesBufDiff()

	n.Render(buf)

	return buf.String()
}

func TestSingleNode(t *testing.T) {
	t.Parallel()

	got := viewString(rootStr("root"))
	want := "root"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSingleChild(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	our.Child(rootStr("child"))

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

	our := rootStr("root")
	for _, s := range []string{"a", "b", "c"} {
		our.Child(rootStr(s))
	}

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d: %q", len(lines), got)
	}
}

func TestNested(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	for _, s := range []string{"a", "b"} {
		child := rootStr(s)
		for _, s2 := range []string{"x", "y"} {
			child.Child(rootStr(s2))
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

	our := rootStr("root")
	our.Child(rootStr("child"))

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestDeepNesting(t *testing.T) {
	t.Parallel()

	our := rootStr("r")
	{
		a := rootStr("a")
		a.Child(rootStr("a1"))
		a.Child(rootStr("a2"))
		our.Child(a)

		b := rootStr("b")
		b1 := rootStr("b1")
		b1.Child(rootStr("b1a"))
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

	our := rootStr("root")
	our.Child(rootStr("string child"))
	our.Child(rootStr("node child"))

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), got)
	}
}

func TestMultiline(t *testing.T) {
	t.Parallel()

	our := rootStr("r")
	our.Child(rootStr("line1\nline2\nline3"))

	a := rootStr("a")
	our.Child(a)
	a.Child(rootStr("child-line1\nchild-line2"))

	got := viewString(our)
	lines := strings.Split(got, "\n")

	if len(lines) != 7 {
		t.Errorf("expected 7 lines, got %d: %q", len(lines), got)
	}
}

func TestViewReuse(t *testing.T) {
	t.Parallel()

	tree := rootStr("root")
	tree.Child(rootStr("a"))
	tree.Child(rootStr("b"))

	var prev string

	for iteration := range 3 {
		renderBuf := buffer.NewLinesBufDiff()
		tree.Render(renderBuf)

		got := renderBuf.String()

		if iteration > 0 && got != prev {
			t.Errorf("iteration %d: output changed from previous: %q vs %q", iteration, got, prev)
		}

		prev = got

		if !strings.Contains(got, "root") || !strings.Contains(got, "a") {
			t.Errorf("iteration %d: unexpected output: %q", iteration, got)
		}

		lines := strings.Split(got, "\n")
		if len(lines) != 3 {
			t.Errorf("iteration %d: expected 3 lines, got %d: %q", iteration, len(lines), got)
		}
	}
}

func TestViewAppends(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	our.Child(rootStr("child"))

	renderBuf := buffer.NewLinesBufDiff()
	renderBuf.Write([]byte("prefix|"))
	our.Render(renderBuf)

	got := renderBuf.String()

	if !strings.HasPrefix(got, "prefix|") {
		t.Errorf("View should append to existing buffer content, got %q", got)
	}

	if !strings.Contains(got, "root") {
		t.Errorf("View should append tree content, got %q", got)
	}
}
