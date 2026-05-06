package tree

import (
	"strings"
	"unsafe"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

const maxPrefixBytes = 32 * 32

type Node struct {
	content   string
	children  []*Node
	multiline bool

	connMid  []byte
	connLast []byte
	indMid   []byte
	indLast  []byte
}

func New() *Node { return &Node{} }

func (n *Node) Root(content string) *Node {
	n.content = content
	n.multiline = strings.IndexByte(content, '\n') >= 0

	return n
}

func (n *Node) EnumeratorStyle(s style.Style) *Node {
	connMid := s.Render("├──")
	connLast := s.Render("╰──")
	//nolint:gosec // G103: styled strings are safe to convert to []byte
	n.connMid = unsafe.Slice(unsafe.StringData(connMid), len(connMid))
	//nolint:gosec // G103: styled strings are safe to convert to []byte
	n.connLast = unsafe.Slice(unsafe.StringData(connLast), len(connLast))

	return n
}

func (n *Node) IndenterStyle(s style.Style) *Node {
	indMid := s.Render("│  ")
	indLast := s.Render("   ")
	//nolint:gosec // G103: styled strings are safe to convert to []byte
	n.indMid = unsafe.Slice(unsafe.StringData(indMid), len(indMid))
	//nolint:gosec // G103: styled strings are safe to convert to []byte
	n.indLast = unsafe.Slice(unsafe.StringData(indLast), len(indLast))

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

// View renders the tree into the provided byte slice, reusing its backing array
// across calls. Use this when you need the output as a string.
func (n *Node) View(buf *[]byte) {
	if len(n.children) == 0 {
		*buf = append((*buf)[:0], n.content...)

		return
	}

	*buf = (*buf)[:0]

	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	pfxEnd[0] = 0

	treeSty := n.nodeStyle()
	*buf = n.renderBytes(*buf, pfxBuf[:], pfxEnd[:], 0, treeSty.midConn, treeSty, true)
}

type treeStyle struct {
	midConn  []byte
	lastConn []byte
	midInd   []byte
	lastInd  []byte
}

func (n *Node) nodeStyle() treeStyle {
	if len(n.connMid) > 0 {
		return treeStyle{
			midConn:  n.connMid,
			lastConn: n.connLast,
			midInd:   n.indMid,
			lastInd:  n.indLast,
		}
	}

	return treeStyle{
		midConn:  defaultMidConn,
		lastConn: defaultLastConn,
		midInd:   defaultMidInd,
		lastInd:  defaultLastInd,
	}
}

var (
	defaultMidConn  = []byte("├── ")
	defaultLastConn = []byte("╰── ")
	defaultMidInd   = []byte("│   ")
	defaultLastInd  = []byte("    ")
)

func (n *Node) renderBytes(
	buf []byte, pfxBuf []byte, pfxEnd []int, depth int, conn []byte, treeSty treeStyle, isRoot bool,
) []byte {
	if isRoot { //nolint:nestif // performance: single function avoids extra call overhead
		buf = append(buf, n.content...)
	} else {
		pfxLen := pfxEnd[depth-1]
		buf = append(buf, pfxBuf[:pfxLen]...)
		buf = append(buf, conn...)

		if !n.multiline {
			buf = append(buf, n.content...)
		} else {
			pfx := strings.IndexByte(n.content, '\n')
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
		pfxEnd[depth+1] = off + len(treeSty.midInd)
		buf = n.children[childIdx].renderBytes(buf, pfxBuf, pfxEnd, depth+1, treeSty.midConn, treeSty, false)
	}

	buf = append(buf, '\n')

	off := pfxEnd[depth]
	copy(pfxBuf[off:], treeSty.lastInd)
	pfxEnd[depth+1] = off + len(treeSty.lastInd)
	buf = n.children[lastIdx].renderBytes(buf, pfxBuf, pfxEnd, depth+1, treeSty.lastConn, treeSty, false)

	return buf
}

func (n *Node) inheritStyle(parent *Node) {
	if len(parent.connMid) > 0 && len(n.connMid) == 0 {
		n.connMid = parent.connMid
		n.connLast = parent.connLast
		n.indMid = parent.indMid
		n.indLast = parent.indLast

		for _, child := range n.children {
			child.inheritStyle(n)
		}
	}
}
