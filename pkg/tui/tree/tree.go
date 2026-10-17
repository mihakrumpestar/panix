// Package tree implements a retained-mode tree renderer with frame-based node
// reuse and per-leaf content caching. Child matches nodes by xpath position
// across frames; leaf content re-renders only when its version, generation,
// depth width, or prefix changes.
package tree

import (
	"strings"
	"sync"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// maxPrefixBytes supports trees up to 32 levels deep with 3-byte connectors.
const maxPrefixBytes = 32 * 32

// nodePool recycles Node allocations across frames to avoid GC pressure.
var nodePool = sync.Pool{
	New: func() any {
		return &Node{}
	},
}

// treeState is shared by every node in a tree; only the root owns it.
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
	fullKey uint64           // (prefixKey << 1) | isLastChild, encodes full path
	content *buffer.LinesBuf // rendered output: prefix + connector + node content
}

// treeStyle holds the pre-rendered byte slices for tree connectors and indents.
// Shared by all nodes in the tree (allocated once in NewTree).
type treeStyle struct {
	connMid  []byte // branch + horizontal run, styled
	connLast []byte // branch_last + horizontal run, styled
	indMid   []byte // vertical + spaces (continuation line indent), styled
	indLast  []byte // spaces (continuation line indent after last child), styled
}

// Glyphs carries the single-cell connector glyphs a theme supplies. Prefixes
// are composed from them so rendered width always equals the indent step.
type Glyphs struct {
	Branch     []byte // connector before a non-last child
	BranchLast []byte // connector before the last child
	Vertical   []byte // continuation glyph for a non-last parent
	Horizontal []byte // horizontal run inside the connectors
}

// DefaultGlyphs returns the default connectors (├ ╰ │ ─); with step 3 they
// compose to "├──", "╰──", "│  ", "   ".
func DefaultGlyphs() Glyphs {
	return Glyphs{
		Branch:     []byte("├"),
		BranchLast: []byte("╰"),
		Vertical:   []byte("│"),
		Horizontal: []byte("─"),
	}
}

// NewTree creates a tree root with the given style, indent step and glyphs.
// The root is the only node that owns treeState.renderBuf.
func NewTree(sty style.Style, step int, glyphs Glyphs) *Node {
	state := &treeState{
		renderBuf: buffer.NewLinesBuf(),
	}

	node := nodePool.Get().(*Node) //nolint:forcetypeassert // pool always returns *Node
	node.resetForReuse()
	node.treeStyle = &treeStyle{}
	node.step = step
	node.state = state
	state.root = node

	node.ApplyTheme(sty, glyphs)

	return node
}

// ApplyTheme re-renders the shared connector/indent bytes from sty, glyphs and
// the indent step, then invalidates all cached content. It is nil-safe and only
// meaningful on the root, which owns the shared treeStyle.
func (n *Node) ApplyTheme(sty style.Style, glyphs Glyphs) {
	if n == nil || n.treeStyle == nil {
		return
	}

	// The layout math uses n.step; guard the composed run against a
	// non-positive step so strings.Repeat never panics.
	step := max(n.step, 1)

	horizontal := strings.Repeat(string(glyphs.Horizontal), step-1)

	n.treeStyle.connMid = renderOneLine(sty, string(glyphs.Branch)+horizontal)
	n.treeStyle.connLast = renderOneLine(sty, string(glyphs.BranchLast)+horizontal)
	n.treeStyle.indMid = renderOneLine(sty, string(glyphs.Vertical)+strings.Repeat(" ", step-1))
	n.treeStyle.indLast = renderOneLine(sty, strings.Repeat(" ", step))

	n.InvalidateCache()
}

// BeginFrame saves each node's children to prevChildren and clears them for the
// next depth-first rebuild, where Child matches them by xpath position.
func (n *Node) BeginFrame() {
	n.beginFrameRecursive()
}

// Child finds or creates a child by xpath, reusing the node at the same
// previous-frame position when possible, and appends it in call order.
// calculate runs only on cache miss with the parent's depth width and the
// node's previous buffer (nil for new nodes); it should Reset and refill old to
// preserve capacity across frames.
func (n *Node) Child(childXp xpath.Xpath, version uint64, calculate func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf) *Node {
	state := n.state
	childDepth := n.depth + 1
	depthWidth := n.depth * n.step

	prev := n.prevChildren
	reuseIdx := n.reuseIdx

	// Fast path: stable structure matches positionally in O(1).
	if reuseIdx < len(prev) && prev[reuseIdx] != nil && prev[reuseIdx].xpath == childXp {
		free := prev[reuseIdx]
		prev[reuseIdx] = nil
		n.reuseIdx = reuseIdx + 1

		return n.reuseNode(free, childDepth, version, state, depthWidth, calculate)
	}

	for reuseIdx < len(prev) && prev[reuseIdx] == nil {
		reuseIdx++
	}

	// Slow path: linear scan after a structural change.
	for i := reuseIdx; i < len(prev); i++ {
		if prev[i] != nil && prev[i].xpath == childXp {
			free := prev[i]
			prev[i] = nil
			n.reuseIdx = reuseIdx

			return n.reuseNode(free, childDepth, version, state, depthWidth, calculate)
		}
	}

	n.reuseIdx = reuseIdx

	return n.allocateNode(childXp, childDepth, version, state, depthWidth, calculate)
}

// Render renders this node's subtree into the shared tree render buffer and
// returns it. A node without children returns the buffer unchanged (nil when it
// has no tree state). Unreused nodes stay cached in prevChildren until Reset.
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

// WriteRenderTo renders directly into target, avoiding an extra copy. Unreused
// nodes stay cached in prevChildren; call DrainFreeList to release them.
func (n *Node) WriteRenderTo(target *buffer.LinesBuf) {
	if len(n.children) == 0 {
		return
	}

	treeSty := n.treeStyle

	// Stack-allocated prefix buffer; avoids per-render allocation.
	// pfxBuf accumulates the indent prefix bytes (│, spaces) at each depth.
	// pfxEnd tracks the end offset for each depth level.
	var (
		pfxBuf [maxPrefixBytes]byte
		pfxEnd [32]int
	)

	// Root children ("flakes") render without prefix; tree starts at their children.
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

// DrainFreeList releases all unreused nodes from prevChildren. Normally not
// needed: they stay cached for reuse across frames.
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

// InvalidateCache bumps the generation and releases cached leaf content in both
// the active tree and dormant prevChildren, forcing a re-render next frame.
func (n *Node) InvalidateCache() {
	if n == nil || n.state == nil {
		return
	}

	state := n.state
	state.invalidateGen++

	releaseCacheEntries(n.children)

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
	// Swap in O(1): prevChildren keeps the old children for reuse and
	// children reuses the old prevChildren buffer, cleared for the rebuild.
	n.prevChildren, n.children = n.children, n.prevChildren
	n.children = n.children[:0]
	n.reuseIdx = 0

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

	// Recalculate when the version, generation, or depth width changed.
	if free.contentVersion != version || free.cacheGen != state.invalidateGen || free.depthWidth != depthWidth {
		// Pass the old buffer so calculate can Reset and refill in place.
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
	node.resetForReuse()
	node.xpath = childXp
	node.content = calculate(depthWidth, nil)
	node.contentVersion = version
	node.depth = childDepth
	node.depthWidth = depthWidth
	node.step = n.step
	node.cacheGen = state.invalidateGen
	node.treeStyle = n.treeStyle
	node.state = state

	n.children = append(n.children, node)

	return node
}

// renderNode renders this node and its subtree, applying the indent and
// connector prefix at each level. Leaves with state go through the per-leaf
// cache. prefixKey is the path from root as a bitmask of isLastChild flags and
// changes whenever the node's prefix changes.
func (n *Node) renderNode(
	buf *buffer.LinesBuf,
	pfxBuf []byte, pfxEnd []int,
	depth int, conn []byte,
	isLastChild bool,
	prefixKey uint64,
) {
	pfx := pfxBuf[:pfxEnd[depth-1]]

	if len(n.children) == 0 && n.state != nil {
		fullKey := (prefixKey << 1) | btoi(isLastChild)
		n.renderLeaf(buf, pfx, conn, pfxBuf, pfxEnd, depth, fullKey)

		return
	}

	writeLines(n.content, buf, pfx, conn, pfxBuf, pfxEnd, depth)

	if len(n.children) == 0 {
		return
	}

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

// renderLeaf renders a leaf through its cache, keyed by contentVersion and
// fullKey ((prefixKey << 1) | isLastChild, the full path from root). A hit
// appends the pre-rendered content; a miss re-renders and caches it.
func (n *Node) renderLeaf(
	buf *buffer.LinesBuf,
	pfx, conn []byte,
	pfxBuf []byte, pfxEnd []int,
	depth int,
	fullKey uint64,
) {
	entry := n.cacheEntry

	if entry != nil && entry.version == n.contentVersion && entry.fullKey == fullKey {
		buf.AppendFrom(entry.content)

		return
	}

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

// writeLines writes src into buf. A nil pfx (root children) writes content
// directly; otherwise the first line gets the indent and connector and the
// rest get the indent only.
func writeLines(src *buffer.LinesBuf, buf *buffer.LinesBuf, pfx, conn, pfxBuf []byte, pfxEnd []int, depth int) {
	nLines := src.Len()

	if pfx == nil {
		if nLines == 0 {
			buf.EmptyLine()

			return
		}

		buf.AppendFrom(src)

		return
	}

	if nLines == 0 {
		buf.WriteLine2(pfx, conn)

		return
	}

	buf.WriteLine3(pfx, conn, src.Line(0))

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

// drainRoot releases unreused nodes from prevChildren at every level.
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

// resetForReuse clears every per-life and per-frame field so a pooled node
// behaves like a fresh one. Stale depth, xpath, children, versions or style
// pointers must never leak into a new node or shift a new tree's depths; slice
// capacity is preserved so reuse stays allocation free.
func (n *Node) resetForReuse() {
	n.content = nil
	n.cacheEntry = nil
	n.xpath = xpath.Xpath{}
	n.contentVersion = 0
	n.cacheGen = 0
	n.depth = 0
	n.depthWidth = 0
	n.step = 0
	n.treeStyle = nil
	n.state = nil
	n.children = n.children[:0]
	n.prevChildren = n.prevChildren[:0]
	n.reuseIdx = 0
}

// renderOneLine renders a single-line style and returns the first line as []byte.
func renderOneLine(s style.Style, content string) []byte {
	return s.RenderLine([]byte(content))
}
