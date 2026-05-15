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

// Len returns the number of node children.
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

func (n *Node) renderInto(buf *buffer.LinesBuf) {
	nChildren := len(n.children)
	if nChildren == 0 {
		n.writeLines(buf)

		return
	}

	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	n.writeLines(buf)

	treeStyle := n.treeStyle
	lastIdx := nChildren - 1

	for childIdx := range lastIdx {
		off := pfxEnd[0]
		copy(pfxBuf[off:], treeStyle.indMid)
		pfxEnd[1] = off + len(treeStyle.indMid)
		n.children[childIdx].renderNode(buf, pfxBuf[:], pfxEnd[:], 1, treeStyle.connMid, treeStyle)
	}

	off := pfxEnd[0]
	copy(pfxBuf[off:], treeStyle.indLast)
	pfxEnd[1] = off + len(treeStyle.indLast)
	n.children[lastIdx].renderNode(buf, pfxBuf[:], pfxEnd[:], 1, treeStyle.connLast, treeStyle)
}

func (n *Node) writeLines(buf *buffer.LinesBuf) {
	src := n.content
	if src == nil || src.Len() == 0 {
		buf.EmptyLine()

		return
	}

	buf.AppendFrom(src)
}

func (n *Node) renderNode(buf *buffer.LinesBuf, pfxBuf []byte, pfxEnd []int, depth int, conn []byte, treeStyle *treeStyle) {
	pfx := pfxBuf[:pfxEnd[depth-1]]

	src := n.content

	nLines := 0
	if src != nil {
		nLines = src.Len()
	}

	if nLines <= 1 {
		if nLines == 0 {
			buf.WriteLine2(pfx, conn)
		} else {
			buf.WriteLine3(pfx, conn, src.Line(0))
		}
	} else {
		buf.WriteLine3(pfx, conn, src.Line(0))

		contPfx := pfxBuf[:pfxEnd[depth]]

		for i := 1; i < nLines; i++ {
			buf.WriteLine2(contPfx, src.Line(i))
		}
	}

	nChildren := len(n.children)
	if nChildren == 0 {
		return
	}

	lastIdx := nChildren - 1

	for childIdx := range lastIdx {
		off := pfxEnd[depth]
		copy(pfxBuf[off:], treeStyle.indMid)
		pfxEnd[depth+1] = off + len(treeStyle.indMid)
		n.children[childIdx].renderNode(buf, pfxBuf, pfxEnd, depth+1, treeStyle.connMid, treeStyle)
	}

	off := pfxEnd[depth]
	copy(pfxBuf[off:], treeStyle.indLast)
	pfxEnd[depth+1] = off + len(treeStyle.indLast)
	n.children[lastIdx].renderNode(buf, pfxBuf, pfxEnd, depth+1, treeStyle.connLast, treeStyle)
}

// Helpers

// renderOneLine renders a single-line style and returns the first line as []byte.
func renderOneLine(s style.Style, content string) []byte {
	return s.RenderLine([]byte(content))
}
