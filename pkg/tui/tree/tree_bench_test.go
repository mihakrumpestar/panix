package tree

import (
	"fmt"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	lipglosstree "charm.land/lipgloss/v2/tree"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

const (
	benchDepth   = 2
	benchBreadth = 10 // 10^2 = 100 leaf nodes
)

var (
	benchStyle = style.NewStyle().Foreground(style.Color("#6272A4"))

	benchContentLines = []string{
		"node",
		strings.Repeat("=", 40),
		"  status: running",
		"  duration: 1m23s",
	}
	benchContentStr = strings.Join(benchContentLines, "\n")
)

// simulateContent creates a realistic multi-line buffer with formatting work.
func simulateContent(_ int, _ *buffer.LinesBuf) *buffer.LinesBuf {
	buf := buffer.NewLinesBuf()
	for _, line := range benchContentLines {
		buf.WriteLine([]byte(line))
	}

	return buf
}

// Ours — full frame cost (BeginFrame + rebuild + render).
func Benchmark__TreeNoChange(b *testing.B)   { benchOurRender(b, benchDepth, benchBreadth, changeNone) }
func Benchmark__TreeAllChange(b *testing.B)  { benchOurRender(b, benchDepth, benchBreadth, changeAll) }
func Benchmark__TreeHalfChange(b *testing.B) { benchOurRender(b, benchDepth, benchBreadth, changeHalf) }

// Lipgloss has no incremental rendering — all variants re-render the full tree.
// Variants kept for side-by-side comparison with our benchmarks.
func Benchmark_Lipgloss__TreeNoChange(b *testing.B) { benchLipglossRender(b, benchDepth, benchBreadth) }
func Benchmark_Lipgloss__TreeAllChange(b *testing.B) {
	benchLipglossRender(b, benchDepth, benchBreadth)
}
func Benchmark_Lipgloss__TreeHalfChange(b *testing.B) {
	benchLipglossRender(b, benchDepth, benchBreadth)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

type changeKind int

const (
	changeNone changeKind = iota
	changeAll
	changeHalf
)

// collectLeafXps recursively collects all leaf node xpaths.
func collectLeafXps(n *Node, xps *[]string) {
	if len(n.children) == 0 && n.xpath != "" {
		*xps = append(*xps, n.xpath.String())
	}

	for _, child := range n.children {
		collectLeafXps(child, xps)
	}
}

// precomputedTree mirrors the tree structure with pre-computed xpath objects.
type precomputedTree struct {
	xp       xpath.Xpath
	children []*precomputedTree
}

func precomputeXpaths(depth, breadth int) *precomputedTree {
	var build func(depth int, parentXp string) *precomputedTree

	build = func(depth int, parentXp string) *precomputedTree {
		node := &precomputedTree{xp: xpath.New(parentXp)}

		if depth > 0 {
			for i := range breadth {
				childXp := fmt.Sprintf("%s/%d/node", parentXp, i)
				node.children = append(node.children, build(depth-1, childXp))
			}
		}

		return node
	}

	pt := &precomputedTree{xp: xpath.New("root")}
	for i := range breadth {
		pt.children = append(pt.children, build(depth-1, fmt.Sprintf("root/%d", i)))
	}

	return pt
}

func buildTreeFromPrecomputed(xps *precomputedTree) *Node {
	root := NewTree(benchStyle, 3)

	var build func(parent *Node, pt *precomputedTree)

	build = func(parent *Node, pt *precomputedTree) {
		for _, childPt := range pt.children {
			child := parent.Child(childPt.xp, 1, simulateContent)
			build(child, childPt)
		}
	}

	build(root, xps)

	return root
}

func benchOurRender(b *testing.B, depth, breadth int, change changeKind) {
	b.Helper()
	b.ReportAllocs()

	allXps := precomputeXpaths(depth, breadth)
	root := buildTreeFromPrecomputed(allXps)
	root.Render()

	var leafXps []string

	changeSet := 0

	if change != changeNone {
		collectLeafXps(root, &leafXps)

		changeSet = len(leafXps)
		if change == changeHalf {
			changeSet = len(leafXps) / 2
		}
	}

	changeMap := make(map[string]bool, changeSet)
	for _, xp := range leafXps[:changeSet] {
		changeMap[xp] = true
	}

	b.ResetTimer()

	var version uint64

	for b.Loop() {
		root.BeginFrame()

		version++

		var rebuild func(parent *Node, pt *precomputedTree)

		rebuild = func(parent *Node, pt *precomputedTree) {
			for _, childPt := range pt.children {
				ver := uint64(1)

				if change != changeNone {
					childXp := childPt.xp.String()
					if changeMap[childXp] {
						ver = version
					}
				}

				child := parent.Child(childPt.xp, ver, simulateContent)
				rebuild(child, childPt)
			}
		}

		rebuild(root, allXps)
		root.Render()
	}
}

// ── Lipgloss ────────────────────────────────────────────────────────────────

func benchLipglossRender(b *testing.B, depth, breadth int) {
	b.Helper()
	b.ReportAllocs()

	refStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))

	var build func(depth int) *lipglosstree.Tree

	build = func(depth int) *lipglosstree.Tree {
		node := lipglosstree.Root(benchContentStr).
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
