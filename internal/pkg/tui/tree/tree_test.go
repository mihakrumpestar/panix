package tree

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	lptree "charm.land/lipgloss/v2/tree"
)

var testStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

func assertTreesEqual(t *testing.T, got, want string) {
	t.Helper()

	gotLines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	wantLines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")

	if len(gotLines) != len(wantLines) {
		t.Errorf("line count mismatch: got %d, want %d", len(gotLines), len(wantLines))
	}

	maxLines := min(len(gotLines), len(wantLines))

	for i := range maxLines {
		if gotLines[i] != wantLines[i] {
			t.Errorf("line %d mismatch:\n  got:  %q\n  want: %q", i, gotLines[i], wantLines[i])
		}
	}

	if len(gotLines) != len(wantLines) {
		t.Logf("GOT:\n%s", got)
		t.Logf("WANT:\n%s", want)
	}
}

func buildEqualStructures(depth, breadth int) (*Node, *lptree.Tree) {
	var buildOur func(d int) *Node

	buildOur = func(d int) *Node {
		node := New().Root("node").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)

		if d > 0 {
			for range breadth {
				node.Child(buildOur(d - 1))
			}
		}

		return node
	}

	var buildLP func(d int) *lptree.Tree

	buildLP = func(d int) *lptree.Tree {
		node := lptree.New().Root("node").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)

		if d > 0 {
			for range breadth {
				node.Child(buildLP(d - 1))
			}
		}

		return node
	}

	return buildOur(depth), buildLP(depth)
}

func TestSingleNode(t *testing.T) {
	t.Parallel()

	got := New().Root("root").String()

	want := lptree.New().Root("root").String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSingleChild(t *testing.T) {
	t.Parallel()

	our := New().Root("root").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	our.Child(New().Root("child"))

	lptNode := lptree.New().Root("root").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	lptNode.Child(lptree.New().Root("child"))

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestMultipleChildren(t *testing.T) {
	t.Parallel()

	our := New().Root("root").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b", "c"} {
		our.Child(New().Root(s))
	}

	lptNode := lptree.New().Root("root").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b", "c"} {
		lptNode.Child(lptree.New().Root(s))
	}

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestNested(t *testing.T) {
	t.Parallel()

	our := New().Root("root").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b"} {
		child := New().Root(s)
		for _, s2 := range []string{"x", "y"} {
			child.Child(New().Root(s2))
		}

		our.Child(child)
	}

	lptNode := lptree.New().Root("root").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	for _, s := range []string{"a", "b"} {
		child := lptree.New().Root(s)
		for _, s2 := range []string{"x", "y"} {
			child.Child(lptree.New().Root(s2))
		}

		lptNode.Child(child)
	}

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestNoStyle(t *testing.T) {
	t.Parallel()

	our := New().Root("root")
	our.Child(New().Root("child"))

	lptNode := lptree.New().Root("root")
	lptNode.Child(lptree.New().Root("child"))

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestUnstyledEnumerators(t *testing.T) {
	t.Parallel()

	our := New().Root("root").Enumerator(EnumeratorRounded)
	for _, s := range []string{"a", "b"} {
		our.Child(New().Root(s))
	}

	lptNode := lptree.New().Root("root").Enumerator(lptree.RoundedEnumerator)
	for _, s := range []string{"a", "b"} {
		lptNode.Child(lptree.New().Root(s))
	}

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestDeepNesting(t *testing.T) {
	t.Parallel()

	our := New().Root("r").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
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

	lptNode := lptree.New().Root("r").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	{
		a := lptree.New().Root("a")
		a.Child(lptree.New().Root("a1"))
		a.Child(lptree.New().Root("a2"))
		lptNode.Child(a)

		b := lptree.New().Root("b")
		b1 := lptree.New().Root("b1")
		b1.Child(lptree.New().Root("b1a"))
		b.Child(b1)
		lptNode.Child(b)
	}

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestMixedStringAndNodeChildren(t *testing.T) {
	t.Parallel()

	our := New().Root("root").Enumerator(EnumeratorRounded).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	our.ChildString("string child")
	our.Child(New().Root("node child"))

	lptNode := lptree.New().Root("root").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(testStyle).IndenterStyle(testStyle)
	lptNode.Child("string child")
	lptNode.Child(lptree.New().Root("node child"))

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestMultiline(t *testing.T) {
	t.Parallel()

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	our := New().Root("r").Enumerator(EnumeratorRounded).EnumeratorStyle(style).IndenterStyle(style)
	our.ChildString("line1\nline2\nline3")

	a := New().Root("a")
	our.Child(a)
	a.ChildString("child-line1\nchild-line2")

	lptNode := lptree.New().Root("r").Enumerator(lptree.RoundedEnumerator).EnumeratorStyle(style).IndenterStyle(style)
	lptNode.Child("line1\nline2\nline3")

	b := lptree.New().Root("a")
	lptNode.Child(b)
	b.Child("child-line1\nchild-line2")

	assertTreesEqual(t, our.String(), lptNode.String())
}

func TestAutoGenerated(t *testing.T) {
	t.Parallel()

	for _, depth := range []int{0, 1, 2, 3} {
		for _, breadth := range []int{1, 2, 3, 4} {
			our, lptNode := buildEqualStructures(depth, breadth)

			t.Run("", func(t *testing.T) {
				t.Parallel()

				assertTreesEqual(t, our.String(), lptNode.String())
			})
		}
	}
}

func BenchmarkSimpleTree(b *testing.B)       { benchTree(b, true, 3, 3) }
func BenchmarkLipglossTree(b *testing.B)     { benchTree(b, false, 3, 3) }
func BenchmarkSimpleTreeFlat(b *testing.B)   { benchTree(b, true, 1, 20) }
func BenchmarkLipglossTreeFlat(b *testing.B) { benchTree(b, false, 1, 20) }
func BenchmarkSimpleTreeDeep(b *testing.B)   { benchTree(b, true, 8, 2) }
func BenchmarkLipglossTreeDeep(b *testing.B) { benchTree(b, false, 8, 2) }

func benchTree(b *testing.B, simple bool, depth, breadth int) {
	b.Helper()

	our, lptNode := buildEqualStructures(depth, breadth)

	if simple {
		b.ResetTimer()

		for b.Loop() {
			_ = our.String()
		}
	} else {
		b.ResetTimer()

		for b.Loop() {
			_ = lptNode.String()
		}
	}
}
