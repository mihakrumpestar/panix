package viewport

import (
	"bytes"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

const maxSpacesLen = 512
const scrollbarColWidth = 2
const borderOverhead = 2
const mouseWheelDelta = 3

var spaces = bytes.Repeat([]byte(" "), maxSpacesLen)

var horizBorderBytes = func() []byte {
	const utf8Len = 3

	bytes := make([]byte, maxSpacesLen*utf8Len)
	for idx := range bytes {
		switch idx % utf8Len {
		case 0:
			bytes[idx] = 0xe2
		case 1:
			bytes[idx] = 0x94
		default:
			bytes[idx] = 0x80
		}
	}

	return bytes
}()

// Viewport renders scrollable, optionally bordered content with a scrollbar.
type Viewport struct {
	linesBuf         *buffer.LinesBuf // owned buffer (adopted via adoptLinesBuf), nil if main viewport
	linesLen         int              // cached len(linesBuf.indexes), 0 if empty
	lineWidths       []int
	width            int
	height           int
	yOffset          int
	scrollbar        bool
	scrollbarReserve bool
	main             bool
	bordered         bool
	maxHeight        int
	thumbChar        string
	trackChar        string
	thumbStyle       style.Style
	trackStyle       style.Style
	borderStyle      style.Style

	// Pre-computed scrollbar cell bytes (built once in WithScrollbar).
	thumbCell []byte
	trackCell []byte
	emptyCell []byte

	// Pre-computed border bytes (built once in WithBorder / SetBorderStyle).
	borderLeft  []byte
	borderRight []byte
	borderTopL  []byte
	borderTopR  []byte
	borderBotL  []byte
	borderBotR  []byte

	// Contiguous padded-line buffer: all padded lines joined with '\n'.
	paddedBuf       []byte
	lineOffsets     []int // byte offset of each line start; len = linesLen+1
	paddedBufCW     int   // contentW paddedBuf was built for; -1 = invalid
	paddedLineCount int   // how many lines are in paddedBuf; for incremental builds
	contentChanged  bool  // set by SetContent, cleared by ensurePaddedCache

	// Previous-frame snapshot for diff-based incremental rebuild (main viewport).
	prevPaddedBuf   []byte
	prevLineOffsets []int
	prevContent     buffer.LinesBuf
	prevLineWidths  []int

	// Scratch buffer reused for building individual output lines.
	scratchBuf []byte

	cacheValid   bool
	cachedOutput *buffer.LinesBuf
}

type Option func(*Viewport)

func WithWidth(w int) Option {
	return func(m *Viewport) {
		m.width = w
	}
}

func WithHeight(h int) Option {
	return func(m *Viewport) {
		m.height = h
	}
}

func WithScrollbar(thumbChar, trackChar string, thumbColor, trackColor style.Color) Option {
	return func(viewport *Viewport) {
		viewport.scrollbar = true
		viewport.thumbChar = thumbChar
		viewport.trackChar = trackChar
		viewport.thumbStyle = style.NewStyle().Foreground(thumbColor)
		viewport.trackStyle = style.NewStyle().Foreground(trackColor)
		viewport.buildScrollbarCells()
	}
}

func WithMain() Option {
	return func(m *Viewport) {
		m.main = true
		m.scrollbarReserve = true
	}
}

func WithScrollbarReserve() Option {
	return func(m *Viewport) {
		m.scrollbarReserve = true
	}
}

func WithBorder(borderColor style.Color) Option {
	return func(m *Viewport) {
		m.bordered = true
		m.borderStyle = style.NewStyle().Foreground(borderColor)
		m.buildBorderStrings()
	}
}

func WithMaxHeight(h int) Option {
	return func(m *Viewport) {
		m.maxHeight = h
	}
}

func New(opts ...Option) Viewport {
	model := Viewport{
		paddedBufCW: -1,
	}

	for _, opt := range opts {
		opt(&model)
	}

	return model
}

//nolint:funcorder
func (m *Viewport) buildBorderStrings() {
	m.borderLeft = m.borderStyle.RenderLine([]byte("│"))
	m.borderRight = m.borderStyle.RenderLine([]byte("│"))
	m.borderTopL = m.borderStyle.RenderLine([]byte("╭"))
	m.borderTopR = m.borderStyle.RenderLine([]byte("╮"))
	m.borderBotL = m.borderStyle.RenderLine([]byte("╰"))
	m.borderBotR = m.borderStyle.RenderLine([]byte("╯"))
}

func (m *Viewport) SetWidth(w int) {
	if w != m.width {
		m.width = w
		m.cacheValid = false
		m.paddedBufCW = -1
		m.contentChanged = true
	}
}

func (m *Viewport) SetHeight(h int) {
	if h != m.height {
		m.height = h
		m.cacheValid = false
		m.contentChanged = true
	}
}

func (m *Viewport) SetBorderStyle(borderColor style.Color) {
	newStyle := style.NewStyle().Foreground(borderColor)
	if bytes.Equal(m.borderStyle.RenderLine(nil), newStyle.RenderLine(nil)) {
		return
	}

	m.borderStyle = newStyle
	m.buildBorderStrings()
	m.cacheValid = false
}

func (m *Viewport) HasScrollbar() bool {
	return m.scrollbar
}

func (m *Viewport) IsMain() bool {
	return m.main
}

func (m *Viewport) HasScrollbarReserve() bool {
	return m.scrollbarReserve
}

// MaxLineWidth returns the widest visual line, computing widths on demand.
func (m *Viewport) MaxLineWidth() int {
	m.ensureLineWidths()

	maxW := 0

	for idx, lineWidth := range m.lineWidths {
		if lineWidth < 0 {
			lineWidth = style.CellWidth(m.line(idx))
			m.lineWidths[idx] = lineWidth
		}

		if lineWidth > maxW {
			maxW = lineWidth
		}
	}

	return maxW
}

// ViewLine returns the visible line at the given index within the current view.
func (m *Viewport) ViewLine(idx int) []byte {
	contentH := m.height
	if m.bordered {
		contentH -= borderOverhead
	}

	start := m.yOffset
	if start >= m.linesLen {
		start = 0
	}

	if idx < 0 || idx >= contentH {
		return nil
	}

	lineIdx := start + idx
	if lineIdx < m.linesLen {
		return m.line(lineIdx)
	}

	return nil
}

// SetContent is the single entry point for setting viewport content.
// For main viewports (pre-wrapped): stores lines directly.
// For non-main viewports: wraps to viewport width, adopts the LinesBuf.
// No deep copies. No allocations beyond the wrapping LinesBuf (pooled).
func (m *Viewport) SetContent(content [][]byte) error {
	if m.main {
		m.setLines(content)

		return nil
	}

	// Convert [][]byte to LinesBuf for WrapBuf.
	contentBuf := buffer.NewLinesBuf()
	contentBuf.WriteLines(content)

	return m.SetContentBuf(contentBuf)
}

// SetContentBuf is the LinesBuf version of SetContent. Uses Line(i) for
// zero-alloc access instead of requiring a [][]byte view.
func (m *Viewport) SetContentBuf(content *buffer.LinesBuf) error {
	if m.main {
		m.setLinesBuf(content)

		return nil
	}

	contentW := m.width
	if m.bordered {
		contentW -= borderOverhead
	}

	scrollbarActive := m.scrollbar && (m.scrollbarReserve || m.linesLen > m.contentHeight())
	if scrollbarActive {
		contentW -= scrollbarColWidth
	}

	contentW = max(1, contentW)

	wrapBuf := buffer.NewLinesBuf()
	style.WrapBuf(wrapBuf, content, contentW, "")
	m.adoptLinesBuf(wrapBuf)

	if m.scrollbar && !m.scrollbarReserve && m.linesLen > m.contentHeight() {
		scrollbarW := max(1, contentW-scrollbarColWidth)
		wrapBuf2 := buffer.NewLinesBuf()
		style.WrapBuf(wrapBuf2, content, scrollbarW, "")
		m.adoptLinesBuf(wrapBuf2)
	}

	return nil
}

// SetContentLines stores pre-wrapped lines directly, bypassing wrapping.
// Use when content is already formatted to the viewport's width.
// For content that needs wrapping, use SetContent instead.
func (m *Viewport) SetContentLines(lines [][]byte) {
	m.setLines(lines)
}

// ensureLineWidths lazily allocates and initializes the width cache.
// Uncached entries are represented by -1.
//
//nolint:funcorder
func (m *Viewport) ensureLineWidths() {
	if m.lineWidths != nil {
		return
	}

	m.lineWidths = make([]int, m.linesLen)
	for i := range m.lineWidths {
		m.lineWidths[i] = -1
	}
}

// Sync updates the viewport with new content and dimensions in the correct order.
// If height > 0, it is used as a fixed viewport height (e.g. main/fullscreen).
// If height == 0, the viewport auto-sizes up to maxHeight.
// Sync is a no-op when content, width, and height are all unchanged —
// SetWidth/SetHeight/SetContent all short-circuit internally.
func (m *Viewport) Sync(content [][]byte, width, height int) error {
	wasAtBottom := m.ScrollPercent() == 1 && !m.main
	yOffset := m.yOffset

	m.SetWidth(width)

	prelimH := m.preliminaryHeight(height)
	if prelimH > 0 {
		m.SetHeight(prelimH)
	}

	err := m.SetContent(content)
	if err != nil {
		return err
	}

	m.SetHeight(m.finalHeight(height))

	if wasAtBottom {
		m.GotoBottom()
	} else {
		m.SetYOffset(min(yOffset, max(0, m.linesLen-m.contentHeight())))
	}

	return nil
}

// SyncBuf is the LinesBuf version of Sync. Uses Line(i) for zero-alloc access.
func (m *Viewport) SyncBuf(content *buffer.LinesBuf, width, height int) error {
	wasAtBottom := m.ScrollPercent() == 1 && !m.main
	yOffset := m.yOffset

	m.SetWidth(width)

	prelimH := m.preliminaryHeight(height)
	if prelimH > 0 {
		m.SetHeight(prelimH)
	}

	err := m.SetContentBuf(content)
	if err != nil {
		return err
	}

	m.SetHeight(m.finalHeight(height))

	if wasAtBottom {
		m.GotoBottom()
	} else {
		m.SetYOffset(min(yOffset, max(0, m.linesLen-m.contentHeight())))
	}

	return nil
}

func (m *Viewport) TotalLineCount() int {
	return m.linesLen
}

func (m *Viewport) Height() int {
	return m.height
}

func (m *Viewport) Width() int {
	return m.width
}

func (m *Viewport) ContentWidth() int {
	contentWidth := m.width
	if m.bordered {
		contentWidth -= borderOverhead
	}

	if m.scrollbar && (m.scrollbarReserve || m.linesLen > m.contentHeight()) {
		contentWidth -= scrollbarColWidth
	}

	return max(0, contentWidth)
}

func (m *Viewport) ScrollPercent() float64 {
	ch := m.contentHeight()
	if ch >= m.linesLen || m.linesLen == 0 {
		return 1
	}

	return float64(m.yOffset) / float64(m.linesLen-ch)
}

func (m *Viewport) IsBordered() bool {
	return m.bordered
}

func BorderOverhead() int {
	return borderOverhead
}

func ScrollbarColWidth() int {
	return scrollbarColWidth
}

func (m *Viewport) YOffset() int {
	return m.yOffset
}

func (m *Viewport) SetYOffset(offset int) {
	maxOffset := max(m.linesLen-m.contentHeight(), 0)
	newY := max(0, min(offset, maxOffset))

	if newY != m.yOffset {
		m.yOffset = newY
		m.cacheValid = false
	}
}

func (m *Viewport) GotoBottom() {
	offset := max(m.linesLen-m.contentHeight(), 0)

	if m.yOffset != offset {
		m.yOffset = offset
		m.cacheValid = false
	}
}

func (m *Viewport) ScrollDown(n int) {
	m.SetYOffset(m.yOffset + n)
}

func (m *Viewport) ScrollUp(n int) {
	m.SetYOffset(m.yOffset - n)
}

func (m *Viewport) PageDown() {
	m.ScrollDown(m.contentHeight())
}

func (m *Viewport) PageUp() {
	m.ScrollUp(m.contentHeight())
}

func (m *Viewport) HalfPageDown() {
	m.ScrollDown(m.contentHeight() / 2)
}

func (m *Viewport) HalfPageUp() {
	m.ScrollUp(m.contentHeight() / 2)
}

func (m *Viewport) AtTop() bool {
	return m.yOffset <= 0
}

func (m *Viewport) AtBottom() bool {
	return m.yOffset >= m.maxYOffset()
}

func (m *Viewport) Update(msg zeroterm.Msg) {
	switch msg := msg.(type) {
	case zeroterm.KeyPressMsg:
		m.handleKeyPress(msg)
	case zeroterm.MouseWheelMsg:
		m.handleMouseWheel(msg)
	}
}

// InvalidateCache forces the next Render() to re-render from scratch.
// Useful for benchmarks that change scroll position without changing content.
func (m *Viewport) InvalidateCache() {
	m.cacheValid = false
}

func (m *Viewport) Render() *buffer.LinesBuf {
	if m.cacheValid {
		return m.cachedOutput
	}

	if m.cachedOutput == nil {
		m.cachedOutput = buffer.NewLinesBuf()
	}

	m.cachedOutput.Reset()
	m.renderViewInto(m.cachedOutput)
	m.cacheValid = true

	return m.cachedOutput
}

func (m *Viewport) contentDimensions() (int, int) {
	height, width := m.height, m.width

	if m.bordered {
		height -= borderOverhead
		width -= borderOverhead
	}

	return height, width
}

func (m *Viewport) renderEmpty(buf *buffer.LinesBuf, contentW, contentH int) {
	if !m.main {
		return
	}

	showBar := m.scrollbar && m.scrollbarReserve
	if showBar {
		contentW -= scrollbarColWidth
	}

	m.renderEmptyInto(buf, contentW, contentH, showBar)
}

func (m *Viewport) scrollbarInfo(contentH int) (bool, int, int, bool) {
	showBar := m.scrollbar && (m.linesLen > contentH || m.scrollbarReserve)

	if !showBar || m.linesLen <= contentH {
		return showBar, 0, 0, false
	}

	thumb := max(1, contentH*contentH/m.linesLen)
	maxScroll := max(1, m.linesLen-contentH)
	yOff := m.yOffset
	thumbPos := (contentH - thumb) * yOff / maxScroll

	return true, thumbPos, thumbPos + thumb, true
}

func (m *Viewport) renderViewInto(buf *buffer.LinesBuf) {
	m.ensureLineWidths()

	contentH, contentW := m.contentDimensions()

	if contentH <= 0 || contentW <= 0 {
		return
	}

	if m.linesLen == 0 {
		m.renderEmpty(buf, contentW, contentH)

		return
	}

	if m.yOffset >= m.linesLen {
		m.yOffset = 0
	}

	start := m.yOffset
	end := min(start+contentH, m.linesLen)
	showBar, thumbPos, thumbEnd, hasBar := m.scrollbarInfo(contentH)

	if showBar {
		contentW -= scrollbarColWidth
	}

	m.ensurePaddedCache(contentW)

	if !m.bordered && !showBar && end == start+contentH {
		buf.WritePaddedView(m.paddedBuf, m.lineOffsets, start, end)

		return
	}

	if m.bordered {
		m.renderBorderedInto(buf, start, end, contentW, contentH, showBar, thumbPos, thumbEnd, hasBar)

		return
	}

	m.renderUnborderedInto(buf, start, end, contentW, contentH, showBar, thumbPos, thumbEnd, hasBar)
}

func (m *Viewport) renderEmptyInto(buf *buffer.LinesBuf, contentW, contentH int, showBar bool) {
	if m.bordered {
		line := m.scratchBuf[:0]
		line = append(line, m.borderTopL...)
		line = m.appendHorizBorder(line, contentW, showBar)
		line = append(line, m.borderTopR...)
		buf.WriteLine1(line)

		for range contentH {
			line = line[:0]
			line = append(line, m.borderLeft...)
			line = append(line, spaces[:contentW]...)

			if showBar {
				line = append(line, m.emptyCell...)
			}

			line = append(line, m.borderRight...)
			buf.WriteLine1(line)
		}

		line = line[:0]
		line = append(line, m.borderBotL...)
		line = m.appendHorizBorder(line, contentW, showBar)
		line = append(line, m.borderBotR...)
		buf.WriteLine1(line)

		return
	}

	for range contentH {
		line := m.scratchBuf[:0]
		line = append(line, spaces[:contentW]...)

		if showBar {
			line = append(line, m.emptyCell...)
		}

		buf.WriteLine1(line)
	}
}

// ensurePaddedCache builds the contiguous padded-line buffer if needed.
// All padded lines are joined with '\n' into paddedBuf, and lineOffsets
// records the byte offset of each line start. Widths are computed eagerly
// so that render paths never call fastWidth.
// When only new lines were appended (prefix unchanged), the existing
// paddedBuf is extended rather than rebuilt, avoiding O(n) copy for
// unchanged lines.
//

// ensureBufSizes pre-allocates paddedBuf and lineOffsets capacity,
// accounting for existing content when doing incremental builds.
func (m *Viewport) ensureBufSizes(contentW int) {
	remaining := m.linesLen - m.paddedLineCount

	est := len(m.paddedBuf) + (contentW+1)*remaining + 1
	if cap(m.paddedBuf) < est {
		newBuf := make([]byte, len(m.paddedBuf), est)
		copy(newBuf, m.paddedBuf)
		m.paddedBuf = newBuf
	}

	offsetLen := m.linesLen + 1
	if cap(m.lineOffsets) < offsetLen {
		oldOffsets := m.lineOffsets
		m.lineOffsets = make([]int, offsetLen)
		copy(m.lineOffsets, oldOffsets)
	} else {
		m.lineOffsets = m.lineOffsets[:offsetLen]
	}
}

// padLines builds or extends the padded-line buffer.
// Starts from paddedLineCount, so that previously padded lines are
// preserved when only new lines (or the last line) changed.
// For the main viewport, uses a diff-based incremental approach:
// line content is compared against the previous frame, and unchanged
// lines are bulk-copied from the previous padded buffer.
func (m *Viewport) padLines(contentW int) {
	useBulkCopy := m.main && m.prevPaddedBuf != nil && m.prevContent.Len() > 0 && m.paddedBufCW == contentW

	for idx := m.paddedLineCount; idx < m.linesLen; idx++ {
		m.lineOffsets[idx] = len(m.paddedBuf)

		if useBulkCopy && m.tryBulkCopyLine(idx) {
			continue
		}

		m.padNewLine(idx, contentW)
	}

	m.lineOffsets[m.linesLen] = len(m.paddedBuf)
	m.paddedLineCount = m.linesLen
}

// tryBulkCopyLine attempts to bulk-copy an unchanged line from the previous
// frame's padded buffer, carrying over the cached width. Returns true if
// the line was identical and bulk-copied.
func (m *Viewport) tryBulkCopyLine(idx int) bool {
	if idx >= m.prevContent.Len() || !bytes.Equal(m.line(idx), m.prevContent.Line(idx)) {
		return false
	}

	m.paddedBuf = append(m.paddedBuf, m.prevPaddedBuf[m.prevLineOffsets[idx]:m.prevLineOffsets[idx+1]]...)

	// Carry over cached width from previous frame for unchanged lines.
	if m.prevLineWidths != nil && idx < len(m.prevLineWidths) && m.prevLineWidths[idx] >= 0 {
		m.lineWidths[idx] = m.prevLineWidths[idx]
	}

	return true
}

// padNewLine computes the width for a new or changed line and appends it
// with right-padding to fill contentW.
func (m *Viewport) padNewLine(idx int, contentW int) {
	line := m.line(idx)

	lineWidth := m.lineWidths[idx]
	if lineWidth < 0 {
		lineWidth = style.CellWidth(line)
		m.lineWidths[idx] = lineWidth
	}

	m.paddedBuf = append(m.paddedBuf, line...)

	if lineWidth < contentW {
		m.paddedBuf = append(m.paddedBuf, spaces[:contentW-lineWidth]...)
	}
}

func (m *Viewport) ensurePaddedCache(contentW int) {
	if !m.contentChanged && m.paddedBufCW == contentW && len(m.lineOffsets) == m.linesLen+1 {
		return
	}

	if m.paddedBufCW != contentW {
		m.paddedLineCount = 0
		m.paddedBuf = m.paddedBuf[:0]
	}

	m.ensureBufSizes(contentW)
	m.padLines(contentW)
	m.paddedBufCW = contentW
	m.contentChanged = false

	if m.main {
		m.prevContent.Reset()
		m.prevContent.CopyFrom(m.linesBuf)
	}
}

// appendScrollbarCell appends the appropriate scrollbar cell directly to the
// last line in buf, avoiding the intermediate scratchBuf copy.
func (m *Viewport) appendScrollbarCell(buf *buffer.LinesBuf, showBar, hasBar, inThumb bool) {
	if !showBar {
		return
	}

	switch {
	case !hasBar:
		buf.AppendToLine(m.emptyCell)
	case inThumb:
		buf.AppendToLine(m.thumbCell)
	default:
		buf.AppendToLine(m.trackCell)
	}
}

func (m *Viewport) renderBorderedInto(
	buf *buffer.LinesBuf,
	start, end, contentW, contentH int,
	showBar bool,
	thumbPos, thumbEnd int,
	hasBar bool,
) {
	line := m.scratchBuf[:0]
	line = append(line, m.borderTopL...)
	line = m.appendHorizBorder(line, contentW, showBar)
	line = append(line, m.borderTopR...)
	buf.WriteLine1(line)

	for idx := start; idx < end; idx++ {
		buf.WriteLine2(m.borderLeft, m.paddedBuf[m.lineOffsets[idx]:m.lineOffsets[idx+1]])
		m.appendScrollbarCell(buf, showBar, hasBar, idx-start >= thumbPos && idx-start < thumbEnd)
		buf.AppendToLine(m.borderRight)
	}

	fillStart := end - start
	for idx := fillStart; idx < contentH; idx++ {
		buf.WriteLine2(m.borderLeft, spaces[:contentW])
		m.appendScrollbarCell(buf, showBar, hasBar, idx >= thumbPos && idx < thumbEnd)
		buf.AppendToLine(m.borderRight)
	}

	line = line[:0]
	line = append(line, m.borderBotL...)
	line = m.appendHorizBorder(line, contentW, showBar)
	line = append(line, m.borderBotR...)
	buf.WriteLine1(line)
}

func (m *Viewport) renderUnborderedInto(
	buf *buffer.LinesBuf,
	start, end, contentW, contentH int,
	showBar bool,
	thumbPos, thumbEnd int,
	hasBar bool,
) {
	if start < end {
		buf.WriteLine1(m.paddedBuf[m.lineOffsets[start]:m.lineOffsets[start+1]])
		m.appendScrollbarCell(buf, showBar, hasBar, thumbPos == 0)
	}

	for idx := start + 1; idx < end; idx++ {
		buf.WriteLine1(m.paddedBuf[m.lineOffsets[idx]:m.lineOffsets[idx+1]])
		m.appendScrollbarCell(buf, showBar, hasBar, idx-start >= thumbPos && idx-start < thumbEnd)
	}

	for idx := end - start; idx < contentH; idx++ {
		buf.WriteLine1(spaces[:contentW])
		m.appendScrollbarCell(buf, showBar, hasBar, idx >= thumbPos && idx < thumbEnd)
	}
}

// appendHorizBorder appends the styled horizontal border line.
func (m *Viewport) appendHorizBorder(buf []byte, contentW int, showBar bool) []byte {
	horizLen := contentW
	if showBar {
		horizLen += scrollbarColWidth
	}

	return append(buf, m.borderStyle.RenderLine(horizBorderBytes[:horizLen*3])...)
}

func (m *Viewport) contentHeight() int {
	h := m.height
	if m.bordered {
		h -= borderOverhead
	}

	return max(0, h)
}

func (m *Viewport) maxYOffset() int {
	return max(m.linesLen-m.contentHeight(), 0)
}

func (m *Viewport) handleKeyPress(msg zeroterm.KeyPressMsg) {
	switch msg.String() {
	case "down", "j":
		m.ScrollDown(1)
	case "up", "k":
		m.ScrollUp(1)
	case "pgdown":
		m.PageDown()
	case "pgup":
		m.PageUp()
	case "halfpageup":
		m.HalfPageUp()
	case "halfpagedown":
		m.HalfPageDown()
	}
}

func (m *Viewport) handleMouseWheel(msg zeroterm.MouseWheelMsg) {
	switch msg.Button {
	case zeroterm.MouseWheelUp:
		m.ScrollUp(mouseWheelDelta)
	case zeroterm.MouseWheelDown:
		m.ScrollDown(mouseWheelDelta)
	}
}

func (m *Viewport) preliminaryHeight(height int) int {
	if height > 0 {
		return height
	}

	if m.maxHeight > 0 {
		h := m.maxHeight
		if m.bordered {
			h += borderOverhead
		}

		return h
	}

	return 0
}

func (m *Viewport) finalHeight(height int) int {
	if height > 0 {
		return max(1, height)
	}

	contentH := max(1, m.linesLen)
	if m.maxHeight > 0 && contentH > m.maxHeight {
		contentH = m.maxHeight
	}

	if m.bordered {
		contentH += borderOverhead
	}

	return max(1, contentH)
}

// line returns the i-th line from the owned linesBuf.
// Inlined by the compiler — two array lookups + one slice op.
func (m *Viewport) line(i int) []byte {
	return m.linesBuf.Line(i)
}

// adoptLinesBuf takes ownership of buf, releasing any previous buffer.
// The viewport accesses lines via buf.Line(i) — zero allocations.
// For non-main viewports where content grows by appending, paddedBuf is
// preserved for the unchanged prefix, avoiding O(n) recomputation on
// every content update. lineWidths are always recomputed because the
// wrapping width may have changed, invalidating cached cell widths.
func (m *Viewport) adoptLinesBuf(buf *buffer.LinesBuf) {
	oldLen := m.linesLen
	oldPaddedCount := m.paddedLineCount

	m.releaseLinesBuf()
	m.linesBuf = buf
	m.linesLen = buf.Len()
	m.lineWidths = nil
	m.paddedLineCount = 0
	m.cacheValid = false
	m.contentChanged = true

	if !m.main && oldLen > 0 && m.linesLen >= oldLen && oldPaddedCount == oldLen {
		m.paddedBuf = m.paddedBuf[:m.lineOffsets[oldLen-1]]
		m.paddedLineCount = oldLen - 1
	}

	maxOffset := max(m.linesLen-m.contentHeight(), 0)
	if m.yOffset > maxOffset {
		m.yOffset = maxOffset
	}
}

// releaseLinesBuf releases the owned buffer if any.
func (m *Viewport) releaseLinesBuf() {
	if m.linesBuf != nil {
		m.linesBuf.Release()
		m.linesBuf = nil
	}

	m.linesLen = 0
}

func (m *Viewport) buildScrollbarCells() {
	m.thumbCell = m.thumbStyle.RenderLine([]byte(m.thumbChar))
	if len(m.thumbCell) == 0 {
		m.thumbCell = []byte(m.thumbChar)
	} else {
		m.thumbCell = append([]byte(" "), m.thumbCell...)
	}

	m.trackCell = m.trackStyle.RenderLine([]byte(m.trackChar))
	if len(m.trackCell) == 0 {
		m.trackCell = []byte(m.trackChar)
	} else {
		m.trackCell = append([]byte(" "), m.trackCell...)
	}

	m.emptyCell = []byte("  ")
}

// setLines stores pre-wrapped lines directly into a LinesBuf, resetting the
// incremental cache so that the next render does a full rebuild.
// Used for the main viewport path and tests where content is already formatted.
func (m *Viewport) setLines(lines [][]byte) {
	m.lineWidths = nil
	m.paddedLineCount = 0
	m.paddedBuf = m.paddedBuf[:0]

	buf := buffer.NewLinesBuf()
	for _, line := range lines {
		buf.WriteLine1(line)
	}

	m.adoptLinesBuf(buf)
}

// setLinesBuf adopts a LinesBuf directly without copying, resetting the
// incremental cache so that the next render does a full rebuild.
// Used for the main viewport path when content is already in LinesBuf format.
func (m *Viewport) setLinesBuf(content *buffer.LinesBuf) {
	if m.main {
		m.prevPaddedBuf = append(m.prevPaddedBuf[:0], m.paddedBuf...)
		m.prevLineOffsets = append(m.prevLineOffsets[:0], m.lineOffsets...)
		m.prevLineWidths = append(m.prevLineWidths[:0], m.lineWidths...)
	}

	m.lineWidths = nil
	m.paddedLineCount = 0
	m.paddedBuf = m.paddedBuf[:0]

	m.adoptLinesBuf(content)
}
