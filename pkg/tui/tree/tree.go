// Package tree implements a retained-mode tree renderer with frame-based
// node reuse and per-leaf content caching. It is designed for building
// hierarchical UI (e.g. build logs) where the tree structure is rebuilt
// every frame from the same data source.
//
// Usage pattern:
//
//	root := tree.NewTree(style, step)
//	for each frame:
//	    root.BeginFrame()                          // move children to freeMap
//	    parent.Child(xp, version, calculate)       // rebuild tree depth-first
//	    target := root.Render()                    // render with tree connectors
//
// The tree reuses nodes across frames via a freeMap (keyed by xpath).
// Leaf nodes cache their rendered content (with prefix/connectors) and
// only re-render when version, cacheGen, depthWidth, or prefixKey changes.
package tree

import (
	"sync"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// maxPrefixBytes is the maximum depth of the tree prefix buffer.
// Supports trees up to 32 levels deep with 3-byte connectors.
const maxPrefixBytes = 32 * 32

// nodePool recycles Node allocations across frames to avoid GC pressure.
var nodePool = sync.Pool{
	New: func() any {
		return &Node{}
	},
}

// treeState holds shared state for the entire tree. All nodes in a tree
// point to the same treeState. Only the root node owns this struct.
type treeState struct {
	renderBuf *buffer.LinesBuf // reusable buffer for Render() output
	root      *Node            // back-pointer to root node (for root-only checks)

	// invalidateGen is incremented on InvalidateCache(). Nodes compare
	// their cacheGen against this to detect stale content lazily.
	invalidateGen uint64

	// freeMap holds nodes from the previous frame that haven't been reused yet.
	// Keyed by xpath for O(1) lookup during Child() calls.
	freeMap map[xpath.Xpath]*Node
}

// Node is a tree node that holds content, children, and rendering state.
// Non-root nodes share treeStyle and state via pointers (zero per-node overhead).
type Node struct {
	content        *buffer.LinesBuf // this node's content (set by calculate callback)
	children       []*Node          // child nodes (built by Child() calls)
	cacheEntry     *CacheEntry      // leaf-only: cached rendered output with prefix
	treeStyle      *treeStyle       // shared connector/indent byte slices
	state          *treeState       // shared tree state (root-only fields)
	xpath          xpath.Xpath      // unique path identifier for this node
	contentVersion uint64           // caller-provided version (triggers recalculate on change)
	cacheGen       uint64           // matches state.invalidateGen when content was last updated
	depth          int              // depth in tree (root=0, flake=1, cfg=2, ...)
	depthWidth     int              // indent width in columns (depth * step), used for cache invalidation
	step           int              // indent width per depth level (propagated from root)
}

// CacheEntry stores the rendered output of a leaf node (content + prefix +
// connector). The entry is invalidated when any of its key fields change.
type CacheEntry struct {
	version     uint64           // contentVersion when this entry was rendered
	prefixKey   uint64           // encodes the path from root (isLastChild bitmask)
	isLastChild bool             // whether this node is the last child at its level
	content     *buffer.LinesBuf // rendered output: prefix + connector + node content
}

// treeStyle holds the pre-rendered byte slices for tree connectors and indents.
// Shared by all nodes in the tree (allocated once in NewTree).
type treeStyle struct {
	connMid  []byte // "├──" styled
	connLast []byte // "╰──" styled
	indMid   []byte // "│  " styled (continuation line indent)
	indLast  []byte // "   " styled (continuation line indent after last child)
}

// NewTree creates a new tree root with the given style and indent step.
// The root node is the only node that owns treeState.renderBuf.
func NewTree(sty style.Style, step int) *Node {
	state := &treeState{
		freeMap:   make(map[xpath.Xpath]*Node),
		renderBuf: buffer.NewLinesBuf(),
	}

	node := nodePool.Get().(*Node) //nolint:forcetypeassert // pool always returns *Node
	node.treeStyle = &treeStyle{
		connMid:  renderOneLine(sty, "├──"),
		connLast: renderOneLine(sty, "╰──"),
		indMid:   renderOneLine(sty, "│  "),
		indLast:  renderOneLine(sty, "   "),
	}
	node.step = step
	node.state = state
	state.root = node

	return node
}

// BeginFrame moves all children to the freeMap for reuse and clears the
// children list. Call Child() to repopulate. The freeMap ensures O(1)
// lookup by xpath while preserving insertion order in the children slice.
func (n *Node) BeginFrame() {
	moveToFree(n.state.freeMap, n.children)
	n.children = n.children[:0]
}

// moveToFree recursively moves all nodes in the subtree to the freeMap.
func moveToFree(freeMap map[xpath.Xpath]*Node, children []*Node) {
	for _, child := range children {
		moveToFree(freeMap, child.children)
		child.children = child.children[:0]
		freeMap[child.xpath] = child
	}
}

// Child finds or creates a child by xpath. If a node with the same xpath
// exists in the freeMap (from a previous frame), it is reused. Otherwise a
// new node is allocated from the pool. The child is always appended,
// preserving call order.
//
// calculate is called only on cache miss (new node, or version/cacheGen/
// depthWidth changed) with the parent's depth width (parent.depth * step).
// On cache hit the function is NOT called.
func (n *Node) Child(childXp xpath.Xpath, version uint64, calculate func(depthWidth int) *buffer.LinesBuf) *Node {
	state := n.state
	childDepth := n.depth + 1
	depthWidth := n.depth * n.step

	// Try to reuse a node from the previous frame.
	if free, ok := state.freeMap[childXp]; ok {
		delete(state.freeMap, childXp)

		free.depth = childDepth

		// Recalculate if version changed, cache was invalidated, or node
		// moved to a different depth (depthWidth changed).
		if free.contentVersion != version || free.cacheGen != state.invalidateGen || free.depthWidth != depthWidth {
			free.content.Release()
			free.content = calculate(depthWidth)
			free.contentVersion = version
			free.cacheGen = state.invalidateGen
			free.depthWidth = depthWidth
		}

		n.children = append(n.children, free)

		return free
	}

	// No reuse — allocate a new node from the pool.
	node := nodePool.Get().(*Node) //nolint:forcetypeassert // pool always returns *Node
	node.xpath = childXp
	node.content = calculate(depthWidth)
	node.contentVersion = version
	node.depth = childDepth
	node.depthWidth = depthWidth
	node.step = n.step
	node.cacheGen = state.invalidateGen
	node.treeStyle = n.treeStyle
	node.state = state
	node.cacheEntry = nil
	node.children = node.children[:0]

	n.children = append(n.children, node)

	return node
}

// Render renders the tree into the internal renderBuf and returns it.
// Only works on the root node; non-root nodes return their renderBuf as-is.
// After rendering, the freeMap is drained (unreused nodes are released).
func (n *Node) Render() *buffer.LinesBuf {
	state := n.state
	if state == nil || len(n.children) == 0 {
		if state != nil {
			return state.renderBuf
		}

		return nil
	}

	state.renderBuf.Reset()
	n.WriteRenderTo(state.renderBuf)
	drainFreeMap(state)

	return state.renderBuf
}

// WriteRenderTo renders the tree directly into the target buffer, bypassing
// the internal renderBuf. This avoids an extra copy when the caller already
// has a target buffer (e.g. build logs content). Does NOT drain the freeMap;
// call DrainFreeMap() separately if needed.
func (n *Node) WriteRenderTo(target *buffer.LinesBuf) {
	if len(n.children) == 0 {
		return
	}

	treeSty := n.treeStyle

	// Stack-allocated prefix buffer — avoids per-render allocation.
	// pfxBuf accumulates the indent prefix bytes (│, spaces) at each depth.
	// pfxEnd tracks the end offset for each depth level.
	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	// Root children ("flakes") render without prefix — tree starts at their children.
	for _, flake := range n.children {
		writeLines(flake.content, target, nil, nil, nil, nil, 0)

		for childIdx, child := range flake.children {
			isLast := childIdx == len(flake.children)-1

			childConn, childInd := treeSty.connMid, treeSty.indMid
			if isLast {
				childConn, childInd = treeSty.connLast, treeSty.indLast
			}

			off := pfxEnd[0]
			copy(pfxBuf[off:], childInd)
			pfxEnd[1] = off + len(childInd)
			child.renderNode(target, pfxBuf[:], pfxEnd[:], 1, childConn, isLast, 0)
		}
	}
}

// DrainFreeMap releases all nodes remaining in the freeMap (nodes from the
// previous frame that weren't reused). Call after WriteRenderTo if not using
// Render() which drains automatically.
func (n *Node) DrainFreeMap() {
	drainFreeMap(n.state)
}

// Reset releases all children and cached state. For root nodes, also resets
// the renderBuf. Safe to call on nil nodes.
func (n *Node) Reset() {
	if n == nil {
		return
	}

	releaseNodes(n.children)

	n.children = n.children[:0]
	if state := n.state; state != nil {
		drainFreeMap(state)

		if state.root == n {
			state.renderBuf.Reset()
		}
	}
}

// InvalidateCache forces all cached leaf content to be re-rendered on the
// next frame. Bumps the generation counter and releases all cache entries.
// Called on resize, navigation, or any event that changes rendering context.
func (n *Node) InvalidateCache() {
	if n == nil || n.state == nil {
		return
	}

	state := n.state
	state.invalidateGen++

	releaseCacheEntries(n.children)

	for _, node := range state.freeMap {
		releaseCacheEntry(node)
	}
}

// releaseCacheEntries recursively releases cache entries in the subtree.
func releaseCacheEntries(children []*Node) {
	for _, child := range children {
		releaseCacheEntry(child)
		releaseCacheEntries(child.children)
	}
}

// releaseCacheEntry releases a single node's cache entry.
func releaseCacheEntry(node *Node) {
	if node.cacheEntry != nil {
		if node.cacheEntry.content != nil {
			node.cacheEntry.content.Release()
			node.cacheEntry.content = nil
		}

		node.cacheEntry = nil
	}
}

// Len returns the number of children.
func (n *Node) Len() int { return len(n.children) }

// renderNode renders this node and its subtree into buf, applying the
// tree prefix (indent + connector) at each level.
//
// For leaf nodes with state (cached), delegates to renderLeaf which handles
// the per-leaf cache. For non-leaf nodes, writes content directly and
// recurses into children.
//
// prefixKey encodes the path from root as a bitmask of isLastChild flags.
// This is used by renderLeaf to detect when a node's tree prefix changes
// (e.g. a sibling is added/removed, changing the connector style).
func (n *Node) renderNode(
	buf *buffer.LinesBuf,
	pfxBuf []byte, pfxEnd []int,
	depth int, conn []byte,
	isLastChild bool,
	prefixKey uint64,
) {
	pfx := pfxBuf[:pfxEnd[depth-1]]

	// Leaf with state → use cached rendering.
	if len(n.children) == 0 && n.state != nil {
		n.renderLeaf(buf, pfx, conn, pfxBuf, pfxEnd, depth, isLastChild, prefixKey)

		return
	}

	// Non-leaf or root child → write content directly.
	writeLines(n.content, buf, pfx, conn, pfxBuf, pfxEnd, depth)

	if len(n.children) == 0 {
		return
	}

	// Recurse into children, building the prefix at each level.
	treeSty := n.treeStyle
	childPrefixKey := (prefixKey << 1) | btoi(isLastChild)

	for childIdx, child := range n.children {
		isLast := childIdx == len(n.children)-1

		childConn, childInd := treeSty.connMid, treeSty.indMid
		if isLast {
			childConn, childInd = treeSty.connLast, treeSty.indLast
		}

		off := pfxEnd[depth]
		copy(pfxBuf[off:], childInd)
		pfxEnd[depth+1] = off + len(childInd)
		child.renderNode(buf, pfxBuf, pfxEnd, depth+1, childConn, isLast, childPrefixKey)
	}
}

// renderLeaf renders a leaf node using the per-node cache. The cache entry
// is invalidated when any of these change:
//   - contentVersion: caller-provided version (content changed)
//   - prefixKey: path from root (sibling added/removed → connector changes)
//   - isLastChild: whether this node is last at its level
//
// On cache hit, the pre-rendered content is appended directly (zero re-render).
// On cache miss, content is re-rendered with the current prefix and cached.
func (n *Node) renderLeaf(
	buf *buffer.LinesBuf,
	pfx, conn []byte,
	pfxBuf []byte, pfxEnd []int,
	depth int,
	isLastChild bool,
	prefixKey uint64,
) {
	entry := n.cacheEntry

	// Cache hit: all key fields match → reuse pre-rendered content.
	if entry != nil && entry.version == n.contentVersion &&
		entry.prefixKey == prefixKey && entry.isLastChild == isLastChild {
		buf.AppendFrom(entry.content)

		return
	}

	// Cache miss: create or reuse entry, render fresh content.
	if entry == nil {
		entry = &CacheEntry{}
		n.cacheEntry = entry
	} else {
		entry.content.Release()
	}

	entry.content = buffer.NewLinesBuf()
	writeLines(n.content, entry.content, pfx, conn, pfxBuf, pfxEnd, depth)
	entry.version = n.contentVersion
	entry.prefixKey = prefixKey
	entry.isLastChild = isLastChild
	buf.AppendFrom(entry.content)
}

// writeLines writes source content into buf with optional prefix and connector.
// When pfx is nil (root children), content is written directly without prefix.
// When pfx is set, each line is prefixed with the tree indent and connector.
func writeLines(src *buffer.LinesBuf, buf *buffer.LinesBuf, pfx, conn, pfxBuf []byte, pfxEnd []int, depth int) {
	nLines := src.Len()

	// No prefix (root children) — write content directly.
	if pfx == nil {
		if nLines == 0 {
			buf.EmptyLine()
		} else {
			buf.AppendFrom(src)
		}

		return
	}

	// Empty content — write just prefix + connector.
	if nLines == 0 {
		buf.WriteLine2(pfx, conn)

		return
	}

	// First line: prefix + connector + content.
	buf.WriteLine3(pfx, conn, src.Line(0))

	// Continuation lines: indent prefix only (no connector).
	if nLines > 1 {
		contPfx := pfxBuf[:pfxEnd[depth]]
		for i := 1; i < nLines; i++ {
			buf.WriteLine2(contPfx, src.Line(i))
		}
	}
}

// btoi converts a bool to uint64 (1 or 0). Used for prefixKey bitmask.
func btoi(b bool) uint64 {
	if b {
		return 1
	}

	return 0
}

// releaseNodes recursively releases content and cache entries in the subtree.
func releaseNodes(children []*Node) {
	for _, child := range children {
		releaseNodes(child.children)

		if child.content != nil {
			child.content.Release()
			child.content = nil
		}

		if child.cacheEntry != nil {
			if child.cacheEntry.content != nil {
				child.cacheEntry.content.Release()
				child.cacheEntry.content = nil
			}

			child.cacheEntry = nil
		}
	}
}

// drainFreeMap releases all nodes in the freeMap and returns them to the pool.
func drainFreeMap(state *treeState) {
	for xpath, node := range state.freeMap {
		releaseNodes(node.children)

		if node.content != nil {
			node.content.Release()
			node.content = nil
		}

		if node.cacheEntry != nil {
			if node.cacheEntry.content != nil {
				node.cacheEntry.content.Release()
				node.cacheEntry.content = nil
			}

			node.cacheEntry = nil
		}

		nodePool.Put(node)
		delete(state.freeMap, xpath)
	}
}

// Helpers

// renderOneLine renders a single-line style and returns the first line as []byte.
func renderOneLine(s style.Style, content string) []byte {
	return s.RenderLine([]byte(content))
}
