package tree

import (
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	lipglosstree "charm.land/lipgloss/v2/tree"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var benchStyle = style.NewStyle().Foreground(style.Color("#6272A4"))

func Benchmark__Tree(b *testing.B)              { benchTree(b, 3, 3) }
func Benchmark__TreeFlat(b *testing.B)          { benchTree(b, 1, 20) }
func Benchmark__TreeDeep(b *testing.B)          { benchTree(b, 8, 2) }
func Benchmark_Lipgloss__Tree(b *testing.B)     { benchRefTree(b, 3, 3) }
func Benchmark_Lipgloss__TreeFlat(b *testing.B) { benchRefTree(b, 1, 20) }
func Benchmark_Lipgloss__TreeDeep(b *testing.B) { benchRefTree(b, 8, 2) }

func benchTree(b *testing.B, depth, breadth int) {
	b.Helper()

	tree := buildBenchTree(depth, breadth)

	buf := buffer.NewLinesBufDiff()

	tree.Render(buf)

	b.ResetTimer()

	for b.Loop() {
		buf.Reset()
		tree.Render(buf)
	}
}

func buildBenchTree(depth, breadth int) *Node {
	var build func(depth int) *Node

	build = func(depth int) *Node {
		node := NewTree(benchStyle).NewNode(lb("node"))

		if depth > 0 {
			for range breadth {
				node.Child(build(depth - 1))
			}
		}

		return node
	}

	return build(depth)
}

func benchRefTree(b *testing.B, depth, breadth int) {
	b.Helper()

	refStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

	var build func(depth int) *lipglosstree.Tree

	build = func(depth int) *lipglosstree.Tree {
		node := lipglosstree.Root("node").
			EnumeratorStyle(refStyle).
			IndenterStyle(refStyle)

		if depth > 0 {
			for range breadth {
				node.Child(build(depth - 1))
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
