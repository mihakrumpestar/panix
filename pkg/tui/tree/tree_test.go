package tree

import (
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

func lb(s string) *buffer.LinesBuf {
	buf := buffer.NewLinesBuf()
	buf.WriteLine([]byte(s))

	return buf
}

const testStep = 3

func addChild(parent *Node, xp string, content string, version uint64) *Node {
	return parent.Child(xpath.New(xp), version, func(_ int, _ *buffer.LinesBuf) *buffer.LinesBuf {
		return lb(content)
	})
}

func viewString(n *Node) string {
	buf := n.Render()

	return buffer.LinesBufToStringForTests(buf)
}

func TestNewTree(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)

	assert.Equal(t, 0, root.depth)
	assert.Nil(t, root.content)
	assert.Empty(t, root.children)
}

func TestChild_Add(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	child := addChild(root, "a", "content-a", 1)

	assert.Equal(t, 1, child.depth)
	assert.Equal(t, xpath.New("a"), child.xpath)
	assert.Equal(t, uint64(1), child.contentVersion)
	assert.Len(t, root.children, 1)
}

func TestChild_Update(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	child1 := addChild(root, "a", "v1", 1)

	root.BeginFrame()
	child2 := addChild(root, "a", "v2", 2)

	assert.Same(t, child1, child2)
	assert.Equal(t, uint64(2), child2.contentVersion)
}

func TestChild_UpdateSameVersion(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "v1", 1)

	root.BeginFrame()
	addChild(root, "a", "v1", 1)

	assert.Len(t, root.children, 1)
}

func TestChild_Multiple(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)
	addChild(root, "c", "c", 1)

	assert.Len(t, root.children, 3)
}

func TestChild_Depth(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	a := addChild(root, "a", "a", 1)
	b := addChild(a, "a/b", "b", 1)
	c := addChild(b, "a/b/c", "c", 1)

	assert.Equal(t, 1, a.depth)
	assert.Equal(t, 2, b.depth)
	assert.Equal(t, 3, c.depth)
}

func TestRender_Empty(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	result := viewString(root)

	assert.Empty(t, result)
}

func TestRender_SingleChild(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "hello", 1)

	result := viewString(root)
	assert.Contains(t, result, "hello")
}

func TestRender_MultipleChildren(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)
	addChild(root, "c", "c", 1)

	result := viewString(root)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
}

func TestRender_Nested(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	a := addChild(root, "a", "a", 1)
	addChild(a, "a/x", "x", 1)
	addChild(a, "a/y", "y", 1)
	b := addChild(root, "b", "b", 1)
	addChild(b, "b/z", "z", 1)

	result := viewString(root)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "x")
	assert.Contains(t, result, "y")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "z")
}

func TestRender_CacheHit(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "hello", 1)

	result1 := viewString(root)
	result2 := viewString(root)

	assert.Equal(t, result1, result2)
}

func TestRender_CacheMiss_VersionChange(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "v1", 1)

	result1 := viewString(root)

	root.BeginFrame()
	addChild(root, "a", "v2", 2)
	result2 := viewString(root)

	assert.NotEqual(t, result1, result2)
	assert.Contains(t, result2, "v2")
}

func TestRender_CacheMiss_SiblingChange(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "a", 1)

	viewString(root)

	root.BeginFrame()
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)
	result2 := viewString(root)

	assert.Contains(t, result2, "a")
	assert.Contains(t, result2, "b")
}

func TestRender_ConnectorChange(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "flake", 1)

	viewString(root)

	root.BeginFrame()
	addChild(root, "a", "flake", 1)
	addChild(root, "b", "flake2", 1)
	result2 := viewString(root)

	// Root children (flakes) render without prefix — no connectors at this level.
	assert.Contains(t, result2, "flake")
	assert.Contains(t, result2, "flake2")
}

func TestRender_ConnectorsOnChildren(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	flake := addChild(root, "f", "flake", 1)
	addChild(flake, "f/a", "child-a", 1)
	addChild(flake, "f/b", "child-b", 1)

	result := viewString(root)

	assert.Contains(t, result, "├──")
	assert.Contains(t, result, "╰──")
	assert.Contains(t, result, "child-a")
	assert.Contains(t, result, "child-b")
}

func TestReset(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)

	root.Reset()

	assert.Empty(t, root.children)
}

func TestInvalidateCache(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "a", 1)

	viewString(root)
	root.InvalidateCache()
	result := viewString(root)

	assert.Contains(t, result, "a")
}

func TestRender_LeafCacheContent(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	addChild(root, "a", "leaf-content", 1)

	result := viewString(root)
	assert.Contains(t, result, "leaf-content")
}

func TestRender_NonLeafNotCached(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	a := addChild(root, "a", "parent", 1)
	addChild(a, "a/x", "child", 1)

	result1 := viewString(root)
	result2 := viewString(root)

	assert.Equal(t, result1, result2)
	assert.Contains(t, result2, "parent")
	assert.Contains(t, result2, "child")
}

func TestChild_PropagatesState(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep)
	a := addChild(root, "a", "a", 1)
	b := addChild(a, "a/b", "b", 1)

	assert.NotNil(t, a.state)
	assert.NotNil(t, b.state)
	assert.Same(t, root.state, a.state)
	assert.Same(t, root.state, b.state)
}

func TestReset_NilSafe(t *testing.T) {
	t.Parallel()

	var n *Node

	assert.NotPanics(t, func() { n.Reset() })
}

func TestInvalidateCache_NilSafe(t *testing.T) {
	t.Parallel()

	var n *Node

	assert.NotPanics(t, func() { n.InvalidateCache() })
}
