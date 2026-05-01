package tree

import (
	"strings"
	"unsafe"

	"charm.land/lipgloss/v2"
)

const (
	maxPrefixBytes = 32 * 32
	sizePadding    = 64
)

type Node struct {
	content  string
	children []*Node

	connMid  string
	connLast string
	indMid   string
	indLast  string
}

func New() *Node { return &Node{} }

func (n *Node) Root(content string) *Node {
	n.content = content

	return n
}

func (n *Node) EnumeratorStyle(s lipgloss.Style) *Node {
	n.connMid = s.Render("├──")
	n.connLast = s.Render("╰──")

	return n
}

func (n *Node) IndenterStyle(s lipgloss.Style) *Node {
	n.indMid = s.Render("│  ")
	n.indLast = s.Render("   ")

	return n
}

func (n *Node) Child(child *Node) {
	child.inheritStyle(n)
	n.children = append(n.children, child)
}

func (n *Node) ChildString(s string) *Node {
	child := New().Root(s)
	child.inheritStyle(n)
	n.children = append(n.children, child)

	return child
}

func (n *Node) Children() []*Node {
	return n.children
}

func (n *Node) Length() int {
	return len(n.children)
}

func (n *Node) String() string {
	if len(n.children) == 0 {
		return n.content
	}

	treeSty := n.nodeStyle()
	size := n.measureSize(treeSty)

	buf := make([]byte, 0, size)

	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	pfxEnd[0] = 0

	buf = n.renderBytes(buf, pfxBuf[:], pfxEnd[:], 0, "", treeSty, true)

	//nolint:gosec // G103: buf contains well-formed UTF-8 tree output; no dangling pointer risk
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

func (n *Node) RenderTo(buf *strings.Builder) {
	if len(n.children) == 0 {
		buf.WriteString(n.content)

		return
	}

	buf.WriteByte('\n')

	var segs [32]string

	treeSty := n.nodeStyle()
	n.renderRoot(buf, segs[:], 0, treeSty)
}

type treeStyle struct {
	midConn     string
	lastConn    string
	midInd      string
	lastInd     string
	midConnLen  int
	lastConnLen int
	midIndLen   int
	lastIndLen  int
}

func (n *Node) nodeStyle() treeStyle {
	if n.connMid != "" {
		return treeStyle{
			midConn:     n.connMid,
			lastConn:    n.connLast,
			midInd:      n.indMid,
			lastInd:     n.indLast,
			midConnLen:  len(n.connMid),
			lastConnLen: len(n.connLast),
			midIndLen:   len(n.indMid),
			lastIndLen:  len(n.indLast),
		}
	}

	return treeStyle{
		midConn:     "├── ",
		lastConn:    "╰── ",
		midInd:      "│   ",
		lastInd:     "    ",
		midConnLen:  len("├── "),
		lastConnLen: len("╰── "),
		midIndLen:   len("│   "),
		lastIndLen:  len("    "),
	}
}

func (n *Node) renderRoot(buf *strings.Builder, segs []string, depth int, treeSty treeStyle) {
	buf.WriteString(n.content)

	nChildren := len(n.children)
	if nChildren == 0 {
		return
	}

	lastIdx := nChildren - 1
	for childIdx := range lastIdx {
		buf.WriteByte('\n')

		segs[depth] = treeSty.midInd
		n.children[childIdx].render(buf, segs, depth+1, treeSty.midConn, treeSty)
	}

	buf.WriteByte('\n')

	segs[depth] = treeSty.lastInd
	n.children[lastIdx].render(buf, segs, depth+1, treeSty.lastConn, treeSty)
}

func (n *Node) render(buf *strings.Builder, segs []string, depth int, conn string, treeSty treeStyle) {
	for childIdx := range depth - 1 {
		buf.WriteString(segs[childIdx])
	}

	buf.WriteString(conn)

	pfx := strings.IndexByte(n.content, '\n')
	if pfx < 0 {
		buf.WriteString(n.content)
	} else {
		n.renderMultilineContent(buf, segs, depth, pfx)
	}

	nChildren := len(n.children)
	if nChildren == 0 {
		return
	}

	lastIdx := nChildren - 1
	for childIdx := range lastIdx {
		buf.WriteByte('\n')

		segs[depth] = treeSty.midInd
		n.children[childIdx].render(buf, segs, depth+1, treeSty.midConn, treeSty)
	}

	buf.WriteByte('\n')

	segs[depth] = treeSty.lastInd
	n.children[lastIdx].render(buf, segs, depth+1, treeSty.lastConn, treeSty)
}

func (n *Node) renderMultilineContent(buf *strings.Builder, segs []string, depth int, pfx int) {
	buf.WriteString(n.content[:pfx])

	remaining := n.content[pfx+1:]
	for len(remaining) > 0 {
		buf.WriteByte('\n')

		for childIdx := range depth {
			buf.WriteString(segs[childIdx])
		}

		nxt := strings.IndexByte(remaining, '\n')
		if nxt >= 0 {
			buf.WriteString(remaining[:nxt])
			remaining = remaining[nxt+1:]
		} else {
			buf.WriteString(remaining)

			break
		}
	}
}

func (n *Node) renderBytes(
	buf []byte, pfxBuf []byte, pfxEnd []int, depth int, conn string, treeSty treeStyle, isRoot bool,
) []byte {
	if isRoot { //nolint:nestif // performance: single function avoids extra call overhead
		buf = append(buf, n.content...)
	} else {
		pfxLen := pfxEnd[depth-1]
		buf = append(buf, pfxBuf[:pfxLen]...)
		buf = append(buf, conn...)

		pfx := strings.IndexByte(n.content, '\n')
		if pfx < 0 {
			buf = append(buf, n.content...)
		} else {
			buf = append(buf, n.content[:pfx]...)

			remaining := n.content[pfx+1:]
			contPfxLen := pfxEnd[depth]

			for len(remaining) > 0 {
				buf = append(buf, '\n')
				buf = append(buf, pfxBuf[:contPfxLen]...)

				nxt := strings.IndexByte(remaining, '\n')
				if nxt >= 0 {
					buf = append(buf, remaining[:nxt]...)
					remaining = remaining[nxt+1:]
				} else {
					buf = append(buf, remaining...)

					break
				}
			}
		}
	}

	nChildren := len(n.children)
	if nChildren == 0 {
		return buf
	}

	lastIdx := nChildren - 1
	for childIdx := range lastIdx {
		buf = append(buf, '\n')

		off := pfxEnd[depth]
		copy(pfxBuf[off:], treeSty.midInd)
		pfxEnd[depth+1] = off + treeSty.midIndLen
		buf = n.children[childIdx].renderBytes(buf, pfxBuf, pfxEnd, depth+1, treeSty.midConn, treeSty, false)
	}

	buf = append(buf, '\n')

	off := pfxEnd[depth]
	copy(pfxBuf[off:], treeSty.lastInd)
	pfxEnd[depth+1] = off + treeSty.lastIndLen
	buf = n.children[lastIdx].renderBytes(buf, pfxBuf, pfxEnd, depth+1, treeSty.lastConn, treeSty, false)

	return buf
}

func (n *Node) measureSize(treeSty treeStyle) int {
	size := len(n.content)
	n.addSize(&size, 0, 0, treeSty)

	return size + sizePadding
}

func (n *Node) addSize(total *int, depth int, prefixLen int, treeSty treeStyle) {
	nChildren := len(n.children)
	childPrefix := prefixLen + treeSty.midIndLen

	for childIdx := range nChildren {
		child := n.children[childIdx]

		if childIdx == nChildren-1 {
			*total += prefixLen + treeSty.lastConnLen + len(child.content) + 1
		} else {
			*total += prefixLen + treeSty.midConnLen + len(child.content) + 1
		}

		if len(child.children) > 0 {
			child.addSize(total, depth+1, childPrefix, treeSty)
		}
	}
}

func (n *Node) inheritStyle(parent *Node) {
	if parent.connMid != "" && n.connMid == "" {
		n.connMid = parent.connMid
		n.connLast = parent.connLast
		n.indMid = parent.indMid
		n.indLast = parent.indLast

		for _, child := range n.children {
			child.inheritStyle(n)
		}
	}
}
