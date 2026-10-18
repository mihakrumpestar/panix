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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())

	assert.Equal(t, 0, root.depth)
	assert.Nil(t, root.content)
	assert.Empty(t, root.children)
}

func TestChild_Add(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	child := addChild(root, "a", "content-a", 1)

	assert.Equal(t, 1, child.depth)
	assert.Equal(t, xpath.New("a"), child.xpath)
	assert.Equal(t, uint64(1), child.contentVersion)
	assert.Len(t, root.children, 1)
}

func TestChild_Update(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	child1 := addChild(root, "a", "v1", 1)

	root.BeginFrame()
	child2 := addChild(root, "a", "v2", 2)

	assert.Same(t, child1, child2)
	assert.Equal(t, uint64(2), child2.contentVersion)
}

func TestChild_UpdateSameVersion(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "v1", 1)

	root.BeginFrame()
	addChild(root, "a", "v1", 1)

	assert.Len(t, root.children, 1)
}

func TestChild_Multiple(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)
	addChild(root, "c", "c", 1)

	assert.Len(t, root.children, 3)
}

func TestChild_Depth(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	a := addChild(root, "a", "a", 1)
	b := addChild(a, "a/b", "b", 1)
	c := addChild(b, "a/b/c", "c", 1)

	assert.Equal(t, 1, a.depth)
	assert.Equal(t, 2, b.depth)
	assert.Equal(t, 3, c.depth)
}

func TestRender_Empty(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	result := viewString(root)

	assert.Empty(t, result)
}

func TestRender_SingleChild(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "hello", 1)

	result := viewString(root)
	assert.Contains(t, result, "hello")
}

func TestRender_MultipleChildren(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "hello", 1)

	result1 := viewString(root)
	result2 := viewString(root)

	assert.Equal(t, result1, result2)
}

func TestRender_CacheMiss_VersionChange(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "flake", 1)

	viewString(root)

	root.BeginFrame()
	addChild(root, "a", "flake", 1)
	addChild(root, "b", "flake2", 1)
	result2 := viewString(root)

	// Root children (flakes) render without prefix: no connectors at this level.
	assert.Contains(t, result2, "flake")
	assert.Contains(t, result2, "flake2")
}

func TestRender_ConnectorsOnChildren(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)

	root.Reset()

	assert.Empty(t, root.children)
}

func TestInvalidateCache(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "a", 1)

	viewString(root)
	root.InvalidateCache()
	result := viewString(root)

	assert.Contains(t, result, "a")
}

func TestRender_LeafCacheContent(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "leaf-content", 1)

	result := viewString(root)
	assert.Contains(t, result, "leaf-content")
}

func TestRender_NonLeafNotCached(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
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

// TestApplyTheme_NonPositiveStepTreatedAsOne pins the step guard that keeps
// connector composition from panicking.
func TestApplyTheme_NonPositiveStepTreatedAsOne(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), 0, DefaultGlyphs())

	assert.Equal(t, "├", string(root.treeStyle.connMid))
	assert.Equal(t, "│", string(root.treeStyle.indMid))
	assert.Equal(t, " ", string(root.treeStyle.indLast))
}

// TestDrainFreeList_ReleasesUnreusedNodes pins release of leftover prevChildren.
func TestDrainFreeList_ReleasesUnreusedNodes(t *testing.T) {
	t.Parallel()

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())
	addChild(root, "a", "a", 1)
	addChild(root, "b", "b", 1)

	// Rebuild only one child: the other becomes an unreused prevChild.
	root.BeginFrame()
	addChild(root, "a", "a", 1)

	root.DrainFreeList()

	assert.Empty(t, root.prevChildren, "DrainFreeList must release unreused nodes")
}

// TestNewTreeResetsPooledNodeState pins that a root node taken from nodePool
// never inherits state from its previous life. Without the reset, a node that
// was a child before (non-zero depth) makes a new root start at depth 1, which
// shifted every child depth by one depending on shuffled test order.
//
//nolint:paralleltest // seeds the shared node pool; t.Parallel() would let another test consume the seed
func TestNewTreeResetsPooledNodeState(t *testing.T) {
	stale := nodePool.Get().(*Node) //nolint:forcetypeassert // pool always returns *Node
	stale.depth = 7
	stale.depthWidth = 21
	stale.step = 3
	stale.contentVersion = 9
	stale.reuseIdx = 4
	stale.xpath = xpath.New("stale", "child")
	stale.children = append(stale.children, &Node{depth: 3})
	stale.prevChildren = append(stale.prevChildren, &Node{depth: 4})
	nodePool.Put(stale)

	root := NewTree(style.NewStyle(), testStep, DefaultGlyphs())

	assert.Equal(t, 0, root.depth)
	assert.Equal(t, 0, root.depthWidth)
	assert.Equal(t, 0, root.reuseIdx)
	assert.Equal(t, uint64(0), root.contentVersion)
	assert.Nil(t, root.content)
	assert.Nil(t, root.cacheEntry)
	assert.Empty(t, root.children)
	assert.Empty(t, root.prevChildren)
	assert.Equal(t, xpath.New(""), root.xpath)

	child := addChild(root, "a", "a", 1)

	assert.Equal(t, 1, child.depth)
}
