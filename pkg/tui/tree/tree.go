package tree

import (
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

const maxPrefixBytes = 32 * 32

type Node struct {
	content   *buffer.LinesBuf
	children  []*Node
	treeStyle *treeStyle
}

type treeStyle struct {
	connMid  []byte
	connLast []byte
	indMid   []byte
	indLast  []byte
}

func NewTree(sty style.Style) *Node {
	return &Node{
		treeStyle: &treeStyle{
			connMid:  renderOneLine(sty, "├──"),
			connLast: renderOneLine(sty, "╰──"),
			indMid:   renderOneLine(sty, "│  "),
			indLast:  renderOneLine(sty, "   "),
		},
	}
}

// NewNode creates a child node with the given content and the tree's style.
func (n *Node) NewNode(content *buffer.LinesBuf) *Node {
	return &Node{content: content, treeStyle: n.treeStyle}
}

func (n *Node) Child(child *Node) {
	child.treeStyle = n.treeStyle
	n.children = append(n.children, child)
}

func (n *Node) ChildContent(content *buffer.LinesBuf) *Node {
	child := &Node{content: content, treeStyle: n.treeStyle}
	n.children = append(n.children, child)

	return child
}

func (n *Node) Len() int {
	return len(n.children)
}

// Render appends the tree output into buf.
func (n *Node) Render(buf *buffer.LinesBufDiff) {
	n.renderInto(buf.LinesBuf)
}

// RenderInto appends the tree output into dst.
func (n *Node) RenderInto(dst *buffer.LinesBuf) {
	n.renderInto(dst)
}

func (n *Node) renderInto(lb *buffer.LinesBuf) {
	nChildren := len(n.children)
	if nChildren == 0 {
		n.writeLines(lb)

		return
	}

	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	n.writeLines(lb)

	ts := n.treeStyle
	lastIdx := nChildren - 1

	for childIdx := range lastIdx {
		off := pfxEnd[0]
		copy(pfxBuf[off:], ts.indMid)
		pfxEnd[1] = off + len(ts.indMid)
		n.children[childIdx].renderNode(lb, pfxBuf[:], pfxEnd[:], 1, ts.connMid, ts)
	}

	off := pfxEnd[0]
	copy(pfxBuf[off:], ts.indLast)
	pfxEnd[1] = off + len(ts.indLast)
	n.children[lastIdx].renderNode(lb, pfxBuf[:], pfxEnd[:], 1, ts.connLast, ts)
}

func (n *Node) writeLines(lb *buffer.LinesBuf) {
	src := n.content
	if src == nil || src.Len() == 0 {
		lb.EmptyLine()

		return
	}

	lb.AppendFrom(src)
}

func (n *Node) renderNode(lb *buffer.LinesBuf, pfxBuf []byte, pfxEnd []int, depth int, conn []byte, ts *treeStyle) {
	pfx := pfxBuf[:pfxEnd[depth-1]]

	src := n.content

	nLines := 0
	if src != nil {
		nLines = src.Len()
	}

	if nLines <= 1 {
		if nLines == 0 {
			lb.WriteLine2(pfx, conn)
		} else {
			lb.WriteLine3(pfx, conn, src.Line(0))
		}
	} else {
		lb.WriteLine3(pfx, conn, src.Line(0))

		contPfx := pfxBuf[:pfxEnd[depth]]

		for i := 1; i < nLines; i++ {
			lb.WriteLine2(contPfx, src.Line(i))
		}
	}

	nChildren := len(n.children)
	if nChildren == 0 {
		return
	}

	lastIdx := nChildren - 1

	for childIdx := range lastIdx {
		off := pfxEnd[depth]
		copy(pfxBuf[off:], ts.indMid)
		pfxEnd[depth+1] = off + len(ts.indMid)
		n.children[childIdx].renderNode(lb, pfxBuf, pfxEnd, depth+1, ts.connMid, ts)
	}

	off := pfxEnd[depth]
	copy(pfxBuf[off:], ts.indLast)
	pfxEnd[depth+1] = off + len(ts.indLast)
	n.children[lastIdx].renderNode(lb, pfxBuf, pfxEnd, depth+1, ts.connLast, ts)
}

// Helpers

// renderOneLine renders a single-line style and returns the first line as []byte.
func renderOneLine(s style.Style, content string) []byte {
	return s.RenderLine([]byte(content))
}
