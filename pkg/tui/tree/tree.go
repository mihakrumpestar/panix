// Package tree implements a retained-mode tree renderer with frame-based
// node reuse and per-leaf content caching. It is designed for building
// hierarchical UI (e.g. build logs) where the tree structure is rebuilt
// every frame from the same data source.
//
// Usage pattern:
//
//	root := tree.NewTree(style, step)
//	for each frame:
//	    root.BeginFrame()                          // prepare children for reuse
//	    parent.Child(xp, version, calculate)       // rebuild tree depth-first
//	    target := root.Render()                    // render with tree connectors
//
// The tree reuses nodes across frames via in-place positional matching.
// Each node keeps its previous-frame children in prevChildren; during the
// depth-first rebuild, Child() matches by xpath at the expected position
// (O(1) for stable structure) with fallback to linear scan.
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
}

// Node is a tree node that holds content, children, and rendering state.
// Non-root nodes share treeStyle and state via pointers (zero per-node overhead).
type Node struct {
	content        *buffer.LinesBuf // this node's content (set by calculate callback)
	children       []*Node          // child nodes (built by Child() calls)
	prevChildren   []*Node          // children from previous frame, available for reuse
	cacheEntry     *CacheEntry      // leaf-only: cached rendered output with prefix
	treeStyle      *treeStyle       // shared connector/indent byte slices
	state          *treeState       // shared tree state (root-only fields)
	xpath          xpath.Xpath      // unique path identifier for this node
	contentVersion uint64           // caller-provided version (triggers recalculate on change)
	cacheGen       uint64           // matches state.invalidateGen when content was last updated
	depth          int              // depth in tree (root=0, flake=1, cfg=2, ...)
	depthWidth     int              // indent width in columns (depth * step), used for cache invalidation
	step           int              // indent width per depth level (propagated from root)
	reuseIdx       int              // cursor into prevChildren for positional reuse in Child()
}

// CacheEntry stores the rendered output of a leaf node (content + prefix +
// connector). The entry is invalidated when any of its key fields change.
type CacheEntry struct {
	version uint64           // contentVersion when this entry was rendered
	fullKey uint64           // (prefixKey << 1) | isLastChild — encodes full path
	content *buffer.LinesBuf // rendered output: prefix + connector + node content
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

// BeginFrame prepares all nodes in the tree for reuse. Each node's current
// children are saved to prevChildren and the children list is cleared.
// During the subsequent depth-first rebuild, Child() matches nodes from
// prevChildren by xpath at the expected position — O(1) for stable structure.
func (n *Node) BeginFrame() {
	n.beginFrameRecursive()
}

// Child finds or creates a child by xpath. If a node with the same xpath
// exists in prevChildren (from the previous frame), it is reused. Otherwise
// a new node is allocated from the pool. The child is always appended,
// preserving call order.
//
// calculate is called only on cache miss (new node, or version/cacheGen/
// depthWidth changed) with the parent's depth width (parent.depth * step)
// and the node's previous content buffer (nil for new nodes). The callback
// should Reset() and refill old instead of allocating a new buffer — this
// preserves capacity across GC cycles, avoiding repeated buffer growth.
// On cache hit the function is NOT called.
func (n *Node) Child(childXp xpath.Xpath, version uint64, calculate func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf) *Node {
	state := n.state
	childDepth := n.depth + 1
	depthWidth := n.depth * n.step

	prev := n.prevChildren
	reuseIdx := n.reuseIdx

	// Fast path: positional match at reuseIdx (O(1), no hashing).
	// Works when tree structure is stable across frames.
	if reuseIdx < len(prev) && prev[reuseIdx] != nil && prev[reuseIdx].xpath == childXp {
		free := prev[reuseIdx]
		prev[reuseIdx] = nil
		n.reuseIdx = reuseIdx + 1

		return n.reuseNode(free, childDepth, version, state, depthWidth, calculate)
	}

	// Advance past consumed (nil) entries.
	for reuseIdx < len(prev) && prev[reuseIdx] == nil {
		reuseIdx++
	}

	// Slow path: linear scan for structural changes.
	for i := reuseIdx; i < len(prev); i++ {
		if prev[i] != nil && prev[i].xpath == childXp {
			free := prev[i]
			prev[i] = nil
			n.reuseIdx = reuseIdx

			return n.reuseNode(free, childDepth, version, state, depthWidth, calculate)
		}
	}

	n.reuseIdx = reuseIdx

	// No reuse — allocate a new node from the pool.
	return n.allocateNode(childXp, childDepth, version, state, depthWidth, calculate)
}

// Render renders the tree into the internal renderBuf and returns it.
// Only works on the root node; non-root nodes return their renderBuf as-is.
// Does NOT drain unreused nodes — they remain cached in prevChildren for
// potential reuse on subsequent frames. Call Reset() on workflow restart
// to release all nodes.
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

	return state.renderBuf
}

// WriteRenderTo renders the tree directly into the target buffer, bypassing
// the internal renderBuf. This avoids an extra copy when the caller already
// has a target buffer (e.g. build logs content). Does NOT drain unreused nodes;
// call DrainFreeList() separately if needed.
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

// DrainFreeList explicitly releases all unreused nodes from prevChildren.
// Normally not needed — nodes remain cached for reuse across frames.
// Call only if you want to free memory without a full Reset().
func (n *Node) DrainFreeList() {
	drainRoot(n.state)
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
		drainRoot(state)

		if state.root == n {
			state.renderBuf.Reset()
		}
	}
}

// InvalidateCache forces all cached leaf content to be re-rendered on the
// next frame. Bumps the generation counter and releases all cache entries
// in both the active tree (children) and dormant nodes (prevChildren).
// Called on resize, navigation, or any event that changes rendering context.
func (n *Node) InvalidateCache() {
	if n == nil || n.state == nil {
		return
	}

	state := n.state
	state.invalidateGen++

	releaseCacheEntries(n.children)

	// Also release cache entries on dormant (untouched) nodes in prevChildren.
	releasePrevCacheEntries(n.prevChildren)
}

// releasePrevCacheEntries recursively releases cache entries from dormant
// nodes stored in prevChildren slices.
func releasePrevCacheEntries(prev []*Node) {
	for _, node := range prev {
		if node != nil {
			releaseCacheEntry(node)
			releaseCacheEntries(node.children)
			releasePrevCacheEntries(node.prevChildren)
		}
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

func (n *Node) beginFrameRecursive() {
	// Swap children and prevChildren — O(1) pointer swap instead of O(k) copy.
	// After swap: prevChildren holds the old children for reuse,
	// children is the old prevChildren buffer (cleared for rebuild).
	n.prevChildren, n.children = n.children, n.prevChildren
	n.children = n.children[:0]
	n.reuseIdx = 0

	// Recursively prepare all descendants.
	for _, child := range n.prevChildren {
		child.beginFrameRecursive()
	}
}

// reuseNode prepares a previously-used node for its new position. Content is
// recalculated only when version, cache generation, or depth width changed.
func (n *Node) reuseNode(
	free *Node,
	childDepth int,
	version uint64,
	state *treeState,
	depthWidth int,
	calculate func(int, *buffer.LinesBuf) *buffer.LinesBuf,
) *Node {
	free.depth = childDepth

	// Recalculate if version changed, cache was invalidated, or node
	// moved to a different depth (depthWidth changed).
	if free.contentVersion != version || free.cacheGen != state.invalidateGen || free.depthWidth != depthWidth {
		// Pass old content buffer to calculate for Reset()+refill —
		// preserves capacity across GC cycles.
		free.content = calculate(depthWidth, free.content)
		free.contentVersion = version
		free.cacheGen = state.invalidateGen
		free.depthWidth = depthWidth
	}

	n.children = append(n.children, free)

	return free
}

// allocateNode creates a new node from the pool and populates it.
func (n *Node) allocateNode(
	childXp xpath.Xpath,
	childDepth int,
	version uint64,
	state *treeState,
	depthWidth int,
	calculate func(int, *buffer.LinesBuf) *buffer.LinesBuf,
) *Node {
	node := nodePool.Get().(*Node) //nolint:forcetypeassert // pool always returns *Node
	node.xpath = childXp
	node.content = calculate(depthWidth, nil)
	node.contentVersion = version
	node.depth = childDepth
	node.depthWidth = depthWidth
	node.step = n.step
	node.cacheGen = state.invalidateGen
	node.treeStyle = n.treeStyle
	node.state = state
	node.cacheEntry = nil
	node.children = node.children[:0]
	node.prevChildren = node.prevChildren[:0]
	node.reuseIdx = 0

	n.children = append(n.children, node)

	return node
}

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
		fullKey := (prefixKey << 1) | btoi(isLastChild)
		n.renderLeaf(buf, pfx, conn, pfxBuf, pfxEnd, depth, fullKey)

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
//   - fullKey: (prefixKey << 1) | isLastChild — encodes full path from root
//
// On cache hit, the pre-rendered content is appended directly (zero re-render).
// On cache miss, content is re-rendered with the current prefix and cached.
func (n *Node) renderLeaf(
	buf *buffer.LinesBuf,
	pfx, conn []byte,
	pfxBuf []byte, pfxEnd []int,
	depth int,
	fullKey uint64,
) {
	entry := n.cacheEntry

	// Cache hit: all key fields match → reuse pre-rendered content.
	if entry != nil && entry.version == n.contentVersion && entry.fullKey == fullKey {
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
	entry.fullKey = fullKey
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

			return
		}

		buf.AppendFrom(src)

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
		for idx := 1; idx < nLines; idx++ {
			buf.WriteLine2(contPfx, src.Line(idx))
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

// drainRoot releases all unreused nodes remaining in prevChildren at every
// level of the tree. Called after rendering to return unused nodes to the pool.
func drainRoot(state *treeState) {
	drainNodePrevChildren(state.root)
}

// drainNodePrevChildren recursively releases unreused nodes from prevChildren.
func drainNodePrevChildren(node *Node) {
	for _, child := range node.prevChildren {
		if child != nil {
			drainNodePrevChildren(child)
			releaseNodeAndPool(child)
		}
	}

	node.prevChildren = node.prevChildren[:0]
}

// releaseNodeAndPool releases a single node's resources and returns it to the pool.
func releaseNodeAndPool(node *Node) {
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
}

// Helpers

// renderOneLine renders a single-line style and returns the first line as []byte.
func renderOneLine(s style.Style, content string) []byte {
	return s.RenderLine([]byte(content))
}
