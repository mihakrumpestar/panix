package tree

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, "root", viewString(rootStr("root")))
}

func TestSingleChild(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	our.Child(rootStr("child"))

	got := viewString(our)

	assert.Contains(t, got, "root")
	assert.Contains(t, got, "child")

	lines := strings.Split(got, "\n")
	assert.Len(t, lines, 2)
}

func TestMultipleChildren(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	for _, s := range []string{"a", "b", "c"} {
		our.Child(rootStr(s))
	}

	got := viewString(our)
	lines := strings.Split(got, "\n")

	assert.Len(t, lines, 4)
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

	assert.Len(t, lines, 7)
}

func TestNoStyle(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	our.Child(rootStr("child"))

	got := viewString(our)
	lines := strings.Split(got, "\n")

	assert.Len(t, lines, 2)
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

	assert.Len(t, lines, 7)
}

func TestMixedStringAndNodeChildren(t *testing.T) {
	t.Parallel()

	our := rootStr("root")
	our.Child(rootStr("string child"))
	our.Child(rootStr("node child"))

	got := viewString(our)
	lines := strings.Split(got, "\n")

	assert.Len(t, lines, 3)
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

	assert.Len(t, lines, 7)
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

		if iteration > 0 {
			assert.Equal(t, prev, got, "iteration %d", iteration)
		}

		prev = got

		assert.Contains(t, got, "root", "iteration %d", iteration)
		assert.Contains(t, got, "a", "iteration %d", iteration)

		lines := strings.Split(got, "\n")
		assert.Len(t, lines, 3, "iteration %d", iteration)
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

	assert.True(t, strings.HasPrefix(got, "prefix|"))
	assert.Contains(t, got, "root")
}
