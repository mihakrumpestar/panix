package tree

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type EnumeratorType int

const (
	EnumeratorDefault EnumeratorType = iota
	EnumeratorRounded
)

type Node struct {
	content  string
	children []*Node

	connMid  string
	connLast string
	indMid   string
	indLast  string

	enumType EnumeratorType
}

func New() *Node { return &Node{} }

func (n *Node) Root(content string) *Node {
	n.content = content

	return n
}

func (n *Node) Enumerator(e EnumeratorType) *Node {
	n.enumType = e

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

const growBase = 512

func (n *Node) String() string {
	if len(n.children) == 0 {
		return n.content
	}

	var b strings.Builder
	b.Grow(len(n.content) + growBase)
	n.renderRoot(&b)

	return b.String()
}

func (n *Node) RenderTo(buf *strings.Builder) {
	if len(n.children) == 0 {
		buf.WriteString(n.content)

		return
	}

	buf.WriteByte('\n')
	n.renderRoot(buf)
}

func (n *Node) renderRoot(buf *strings.Builder) {
	buf.WriteString(n.content)

	for i, child := range n.children {
		isLast := i == len(n.children)-1

		buf.WriteByte('\n')
		child.render(buf, n.conn(isLast), n.ind(isLast))
	}
}

func (n *Node) render(buf *strings.Builder, prefix, indent string) {
	buf.WriteString(prefix)
	buf.WriteString(line1(n.content))

	for _, line := range tailLines(n.content) {
		buf.WriteByte('\n')
		buf.WriteString(indent)
		buf.WriteString(line)
	}

	for i, child := range n.children {
		isLast := i == len(n.children)-1

		buf.WriteByte('\n')
		child.render(buf, indent+n.conn(isLast), indent+n.ind(isLast))
	}
}

func line1(str string) string {
	before, _, ok := strings.Cut(str, "\n")
	if ok {
		return before
	}

	return str
}

func tailLines(s string) []string {
	_, after, ok := strings.Cut(s, "\n")
	if !ok {
		return nil
	}

	return strings.Split(after, "\n")
}

func (n *Node) conn(isLast bool) string {
	if n.connMid != "" {
		if isLast {
			return n.connLast
		}

		return n.connMid
	}

	if isLast {
		if n.enumType == EnumeratorRounded {
			return "╰── "
		}

		return "└── "
	}

	return "├── "
}

func (n *Node) ind(isLast bool) string {
	if n.indMid != "" {
		if isLast {
			return n.indLast
		}

		return n.indMid
	}

	if isLast {
		return "    "
	}

	return "│   "
}

func (n *Node) inheritStyle(parent *Node) {
	if parent.connMid != "" && n.connMid == "" {
		n.connMid = parent.connMid
		n.connLast = parent.connLast
		n.indMid = parent.indMid
		n.indLast = parent.indLast

		n.enumType = parent.enumType
		for _, child := range n.children {
			child.inheritStyle(n)
		}
	}
}