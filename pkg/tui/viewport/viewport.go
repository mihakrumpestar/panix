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
	lines            [][]byte
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
	paddedBuf      []byte
	lineOffsets    []int // byte offset of each line start; len = len(lines)+1
	paddedBufCW    int   // contentW paddedBuf was built for; -1 = invalid
	cachedLines    [][]byte
	contentChanged bool // set by SetContentLines, cleared by ensurePaddedCache

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
			lineWidth = style.CellWidth(m.lines[idx])
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
	if start >= len(m.lines) {
		start = 0
	}

	if idx < 0 || idx >= contentH {
		return nil
	}

	lineIdx := start + idx
	if lineIdx < len(m.lines) {
		return m.lines[lineIdx]
	}

	return nil
}

func (m *Viewport) SetContent(content [][]byte) error {
	if m.main {
		m.SetContentLines(content)

		return nil
	}

	contentW := m.width
	if m.bordered {
		contentW -= borderOverhead
	}

	scrollbarActive := m.scrollbar && (m.scrollbarReserve || len(m.lines) > m.contentHeight())
	if scrollbarActive {
		contentW -= scrollbarColWidth
	}

	contentW = max(1, contentW)

	wrapBuf := buffer.NewLinesBuf()
	style.Wrap(wrapBuf, content, contentW, "")
	m.SetContentLines(copyLines(wrapBuf.Lines()))
	wrapBuf.Release()

	if m.scrollbar && !m.scrollbarReserve && len(m.lines) > m.contentHeight() {
		scrollbarW := max(1, contentW-scrollbarColWidth)
		wrapBuf = buffer.NewLinesBuf()
		style.Wrap(wrapBuf, content, scrollbarW, "")
		m.SetContentLines(copyLines(wrapBuf.Lines()))
		wrapBuf.Release()
	}

	return nil
}

// copyLines returns deep copies of lines so the caller can safely release
// the source buffer (e.g. a pooled LinesBuf) without corrupting the copies.
func copyLines(lines [][]byte) [][]byte {
	cloned := make([][]byte, len(lines))
	for i, line := range lines {
		cloned[i] = bytes.Clone(line)
	}

	return cloned
}

// SetContentLines sets the content lines. Visual widths are computed lazily
// on first access so that SetContentLines itself is O(1).
// If the lines slice is identical to the current content (same length,
// same strings), no caches are invalidated — subsequent Render() calls
// hit the cached result directly.
// When content changes, cached line widths are preserved for unchanged
// lines so that CellWidth is only recomputed for new or modified lines.
func (m *Viewport) SetContentLines(lines [][]byte) {
	oldLines := m.lines
	m.lines = lines
	m.lineWidths = m.buildPreservedWidths(lines, oldLines)
	m.cacheValid = false
	m.contentChanged = true

	maxOffset := max(len(lines)-m.contentHeight(), 0)

	if m.yOffset > maxOffset {
		m.yOffset = maxOffset
	}
}

// ensureLineWidths lazily allocates and initializes the width cache.
// Uncached entries are represented by -1.
//
//nolint:funcorder
func (m *Viewport) ensureLineWidths() {
	if m.lineWidths != nil {
		return
	}

	m.lineWidths = make([]int, len(m.lines))
	for i := range m.lineWidths {
		m.lineWidths[i] = -1
	}
}

// Sync updates the viewport with new content and dimensions in the correct order.
// If height > 0, it is used as a fixed viewport height (e.g. main/fullscreen).
// If height == 0, the viewport auto-sizes up to maxHeight.
// Sync is a no-op when content, width, and height are all unchanged —
// SetWidth/SetHeight/SetContentLines all short-circuit internally.
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
		m.SetYOffset(min(yOffset, max(0, len(m.lines)-m.contentHeight())))
	}

	return nil
}

func (m *Viewport) TotalLineCount() int {
	return len(m.lines)
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

	if m.scrollbar && (m.scrollbarReserve || len(m.lines) > m.contentHeight()) {
		contentWidth -= scrollbarColWidth
	}

	return max(0, contentWidth)
}

func (m *Viewport) ScrollPercent() float64 {
	ch := m.contentHeight()
	if ch >= len(m.lines) || len(m.lines) == 0 {
		return 1
	}

	return float64(m.yOffset) / float64(len(m.lines)-ch)
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
	maxOffset := max(len(m.lines)-m.contentHeight(), 0)
	newY := max(0, min(offset, maxOffset))

	if newY != m.yOffset {
		m.yOffset = newY
		m.cacheValid = false
	}
}

func (m *Viewport) GotoBottom() {
	offset := max(len(m.lines)-m.contentHeight(), 0)

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

// buildPreservedWidths creates a lineWidths slice that preserves cached
// widths for lines that haven't changed. Lines that are new or modified
// get -1 (uncached), so CellWidth is only recomputed when necessary.
// Returns nil when there are no old widths to preserve.
func (m *Viewport) buildPreservedWidths(newLines, oldLines [][]byte) []int {
	if m.lineWidths == nil {
		return nil
	}

	height := len(newLines)
	if cap(m.lineWidths) < height {
		newWidths := make([]int, height)
		copy(newWidths, m.lineWidths)
		m.lineWidths = newWidths
	} else {
		m.lineWidths = m.lineWidths[:height]
	}

	minLen := min(len(newLines), len(oldLines))
	copyCount := min(len(m.lineWidths), minLen)

	for i := copyCount; i < height; i++ {
		m.lineWidths[i] = -1
	}

	for idx := range minLen {
		pNew, pOld := &newLines[idx], &oldLines[idx]

		nLen, oLen := len(*pNew), len(*pOld)
		if nLen == oLen && (nLen == 0 || &(*pNew)[0] == &(*pOld)[0]) {
			continue
		}

		if !bytes.Equal(*pNew, *pOld) {
			m.lineWidths[idx] = -1
		}
	}

	return m.lineWidths
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
	showBar := m.scrollbar && (len(m.lines) > contentH || m.scrollbarReserve)

	if !showBar || len(m.lines) <= contentH {
		return showBar, 0, 0, false
	}

	thumb := max(1, contentH*contentH/len(m.lines))
	maxScroll := max(1, len(m.lines)-contentH)
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

	if len(m.lines) == 0 {
		m.renderEmpty(buf, contentW, contentH)

		return
	}

	if m.yOffset >= len(m.lines) {
		m.yOffset = 0
	}

	start := m.yOffset
	end := min(start+contentH, len(m.lines))
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

func (m *Viewport) ensureBufSizes(contentW int) {
	est := (contentW+1)*len(m.lines) + 1
	if cap(m.paddedBuf) < est {
		m.paddedBuf = make([]byte, 0, est)
	}

	n := len(m.lines) + 1
	if cap(m.lineOffsets) < n {
		m.lineOffsets = make([]int, n)
	} else {
		m.lineOffsets = m.lineOffsets[:n]
	}
}

func (m *Viewport) padLines(contentW int) {
	buf := m.paddedBuf[:0]
	for idx, line := range m.lines {
		m.lineOffsets[idx] = len(buf)

		lineWidth := m.lineWidths[idx]
		if lineWidth < 0 {
			lineWidth = style.CellWidth(line)
			m.lineWidths[idx] = lineWidth
		}

		buf = append(buf, line...)

		if lineWidth < contentW {
			buf = append(buf, spaces[:contentW-lineWidth]...)
		}
	}

	m.lineOffsets[len(m.lines)] = len(buf)
	m.paddedBuf = buf
}

func (m *Viewport) ensurePaddedCache(contentW int) {
	if !m.contentChanged && m.paddedBufCW == contentW && len(m.lineOffsets) == len(m.lines)+1 {
		return
	}

	if m.contentChanged && m.paddedBufCW == contentW && m.cachedLines != nil {
		if m.tryAppendPaddedLines(contentW) {
			return
		}
	}

	m.ensureBufSizes(contentW)
	m.padLines(contentW)
	m.paddedBufCW = contentW
	m.cachedLines = m.lines
	m.contentChanged = false
}

func (m *Viewport) linesUnchangedFromCache() bool {
	for i := range len(m.cachedLines) {
		cachedLineLen, cachedLine := &m.lines[i], &m.cachedLines[i]

		nLen, cLen := len(*cachedLineLen), len(*cachedLine)
		if nLen == cLen && (nLen == 0 || &(*cachedLineLen)[0] == &(*cachedLine)[0]) {
			continue
		}

		if !bytes.Equal(*cachedLineLen, *cachedLine) {
			return false
		}
	}

	return true
}

func (m *Viewport) extendLineOffsets(newLineCount int) {
	if cap(m.lineOffsets) < newLineCount+1 {
		newOffsets := make([]int, newLineCount+1)
		copy(newOffsets, m.lineOffsets)
		m.lineOffsets = newOffsets
	} else {
		m.lineOffsets = m.lineOffsets[:newLineCount+1]
	}
}

func (m *Viewport) appendNewLines(contentW, oldLineCount, newLineCount int) []byte {
	buf := m.paddedBuf
	for idx := oldLineCount; idx < newLineCount; idx++ {
		m.lineOffsets[idx] = len(buf)

		lineWidth := m.lineWidths[idx]
		if lineWidth < 0 {
			lineWidth = style.CellWidth(m.lines[idx])
			m.lineWidths[idx] = lineWidth
		}

		buf = append(buf, m.lines[idx]...)
		if lineWidth < contentW {
			buf = append(buf, spaces[:contentW-lineWidth]...)
		}
	}

	return buf
}

// tryAppendPaddedLines attempts a delta update: if the existing paddedBuf
// still covers the old lines and only new lines were appended, extend
// paddedBuf with the new lines instead of rebuilding from scratch.
func (m *Viewport) tryAppendPaddedLines(contentW int) bool {
	oldLineCount := len(m.cachedLines)
	if oldLineCount >= len(m.lines) || oldLineCount == 0 {
		return false
	}

	if !m.linesUnchangedFromCache() {
		return false
	}

	newLineCount := len(m.lines)
	m.extendLineOffsets(newLineCount)

	buf := m.appendNewLines(contentW, oldLineCount, newLineCount)

	m.lineOffsets[newLineCount] = len(buf)
	m.paddedBuf = buf
	m.cachedLines = m.lines
	m.contentChanged = false

	return true
}

func (m *Viewport) renderScrollbarInLine(line []byte, showBar, hasBar, inThumb bool) []byte {
	if !showBar {
		return line
	}

	switch {
	case !hasBar:
		return append(line, m.emptyCell...)
	case inThumb:
		return append(line, m.thumbCell...)
	default:
		return append(line, m.trackCell...)
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
		line = line[:0]
		line = append(line, m.borderLeft...)
		line = append(line, m.paddedBuf[m.lineOffsets[idx]:m.lineOffsets[idx+1]]...)
		line = m.renderScrollbarInLine(line, showBar, hasBar, idx-start >= thumbPos && idx-start < thumbEnd)
		line = append(line, m.borderRight...)
		buf.WriteLine1(line)
	}

	fillStart := end - start
	for idx := fillStart; idx < contentH; idx++ {
		line = line[:0]
		line = append(line, m.borderLeft...)
		line = append(line, spaces[:contentW]...)
		line = m.renderScrollbarInLine(line, showBar, hasBar, idx >= thumbPos && idx < thumbEnd)
		line = append(line, m.borderRight...)
		buf.WriteLine1(line)
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
		line := m.scratchBuf[:0]
		line = append(line, m.paddedBuf[m.lineOffsets[start]:m.lineOffsets[start+1]]...)
		line = m.renderScrollbarInLine(line, showBar, hasBar, thumbPos == 0)
		buf.WriteLine1(line)
	}

	for idx := start + 1; idx < end; idx++ {
		line := m.scratchBuf[:0]
		line = append(line, m.paddedBuf[m.lineOffsets[idx]:m.lineOffsets[idx+1]]...)
		line = m.renderScrollbarInLine(line, showBar, hasBar, idx-start >= thumbPos && idx-start < thumbEnd)
		buf.WriteLine1(line)
	}

	for idx := end - start; idx < contentH; idx++ {
		line := m.scratchBuf[:0]
		line = append(line, spaces[:contentW]...)
		line = m.renderScrollbarInLine(line, showBar, hasBar, idx >= thumbPos && idx < thumbEnd)
		buf.WriteLine1(line)
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
	return max(len(m.lines)-m.contentHeight(), 0)
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

	contentH := max(1, len(m.lines))
	if m.maxHeight > 0 && contentH > m.maxHeight {
		contentH = m.maxHeight
	}

	if m.bordered {
		contentH += borderOverhead
	}

	return max(1, contentH)
}
