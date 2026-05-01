package viewport

import (
	"image/color"
	"strings"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pkg/errors"
)

const maxSpacesLen = 512
const scrollbarColWidth = 2
const borderOverhead = 2
const mouseWheelDelta = 3

var spaces = strings.Repeat(" ", maxSpacesLen)

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

// fastWidth returns the visual cell width of a single-line string.
// For ASCII-only text (no ANSI escapes, no multi-byte Unicode), len(s)
// equals the visual width. Falls back to lipgloss.Width otherwise.
func fastWidth(str string) int {
	for i := range len(str) {
		if str[i] == '\x1b' || str[i] >= 0x80 {
			return lipgloss.Width(str)
		}
	}

	return len(str)
}

// Viewport renders scrollable, optionally bordered content with a scrollbar.
type Viewport struct {
	lines            []string
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
	thumbStyle       string
	trackStyle       string
	borderStyle      string

	// Pre-computed scrollbar cell strings (built once in WithScrollbar).
	thumbCell string
	trackCell string
	emptyCell string

	// Pre-computed border strings (built once in WithBorder / SetBorderStyle).
	borderLeft  string
	borderRight string
	borderTopL  string
	borderTopR  string
	borderBotL  string
	borderBotR  string

	// Contiguous padded-line buffer: all padded lines joined with '\n'.
	// Enables zero-copy View() for unbordered no-scrollbar viewports:
	// the output is a direct subslice of paddedBuf.
	paddedBuf   []byte
	lineOffsets []int // byte offset of each line start; len = len(lines)+1
	paddedBufCW int   // contentW paddedBuf was built for; -1 = invalid

	// Render buffer, reused between renders to avoid allocation.
	renderBuf []byte

	cachedView string
	cacheValid bool
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

func WithScrollbar(thumbChar, trackChar string, thumbColor, trackColor color.Color) Option {
	return func(viewport *Viewport) {
		viewport.scrollbar = true
		viewport.thumbChar = thumbChar
		viewport.trackChar = trackChar
		viewport.thumbStyle = colorToAnsi(thumbColor)
		viewport.trackStyle = colorToAnsi(trackColor)
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

func WithBorder(borderColor color.Color) Option {
	return func(m *Viewport) {
		m.bordered = true
		m.borderStyle = colorToAnsi(borderColor)
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

// buildScrollbarCells pre-computes the styled scrollbar cell strings
// so that render only needs append() per line, no string concatenation.
//
//nolint:funcorder
func (m *Viewport) buildScrollbarCells() {
	if m.thumbStyle != "" {
		m.thumbCell = " " + m.thumbStyle + m.thumbChar + "\x1b[0m"
	} else {
		m.thumbCell = " " + m.thumbChar
	}

	if m.trackStyle != "" {
		m.trackCell = " " + m.trackStyle + m.trackChar + "\x1b[0m"
	} else {
		m.trackCell = " " + m.trackChar
	}

	m.emptyCell = "  "
}

// buildBorderStrings pre-computes styled border character strings
// so that render only needs append() per line, no conditional + concat.
//
//nolint:funcorder
func (m *Viewport) buildBorderStrings() {
	if m.borderStyle != "" {
		borderSeq := m.borderStyle
		reset := "\x1b[0m"
		m.borderLeft = borderSeq + "│" + reset
		m.borderRight = borderSeq + "│" + reset
		m.borderTopL = borderSeq + "╭" + reset
		m.borderTopR = borderSeq + "╮" + reset
		m.borderBotL = borderSeq + "╰" + reset
		m.borderBotR = borderSeq + "╯" + reset
	} else {
		m.borderLeft = "│"
		m.borderRight = "│"
		m.borderTopL = "╭"
		m.borderTopR = "╮"
		m.borderBotL = "╰"
		m.borderBotR = "╯"
	}
}

func (m *Viewport) SetWidth(w int) {
	if w != m.width {
		m.width = w
		m.cacheValid = false
		m.paddedBufCW = -1
	}
}

func (m *Viewport) SetHeight(h int) {
	if h != m.height {
		m.height = h
		m.cacheValid = false
		m.paddedBufCW = -1
	}
}

func (m *Viewport) SetBorderStyle(borderColor color.Color) {
	m.borderStyle = colorToAnsi(borderColor)
	m.cacheValid = false
	m.buildBorderStrings()
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
			lineWidth = fastWidth(m.lines[idx])
			m.lineWidths[idx] = lineWidth
		}

		if lineWidth > maxW {
			maxW = lineWidth
		}
	}

	return maxW
}

// ViewLine returns the visible line at the given index within the current view.
func (m *Viewport) ViewLine(idx int) string {
	contentH := m.height
	if m.bordered {
		contentH -= borderOverhead
	}

	start := m.yOffset
	if start >= len(m.lines) {
		start = 0
	}

	if idx < 0 || idx >= contentH {
		return ""
	}

	lineIdx := start + idx
	if lineIdx < len(m.lines) {
		return m.lines[lineIdx]
	}

	return ""
}

func (m *Viewport) SetContent(content string) error {
	if m.main {
		m.SetContentLines(strings.Split(content, "\n"))

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

	wrapped := lipgloss.Wrap(content, contentW, "")
	m.SetContentLines(strings.Split(wrapped, "\n"))

	if m.scrollbar && !m.scrollbarReserve && len(m.lines) > m.contentHeight() {
		scrollbarW := max(1, contentW-scrollbarColWidth)
		wrapped = lipgloss.Wrap(content, scrollbarW, "")
		m.SetContentLines(strings.Split(wrapped, "\n"))
	}

	return nil
}

var ErrLineOverWidth = errors.New("line exceeds ContentWidth")

// SetContentLines sets the content lines. Visual widths are computed lazily
// on first access so that SetContentLines itself is O(1).
func (m *Viewport) SetContentLines(lines []string) {
	m.lines = lines
	m.lineWidths = nil
	m.cacheValid = false
	m.paddedBufCW = -1
	m.paddedBuf = nil
	m.lineOffsets = nil

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
func (m *Viewport) Sync(content string, width, height int) error {
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

const halfDivisor = 2

func (m *Viewport) HalfPageDown() {
	m.ScrollDown(m.contentHeight() / halfDivisor)
}

func (m *Viewport) HalfPageUp() {
	m.ScrollUp(m.contentHeight() / halfDivisor)
}

func (m *Viewport) AtTop() bool {
	return m.yOffset <= 0
}

func (m *Viewport) AtBottom() bool {
	return m.yOffset >= m.maxYOffset()
}

func (m *Viewport) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.handleKeyPress(msg)
	case tea.MouseWheelMsg:
		m.handleMouseWheel(msg)
	}
}

// View returns the rendered viewport string. Results are cached: if no
// mutable state has changed since the last call, the previous string
// is returned directly (~0.75ns on cache hit).
func (m *Viewport) View() string {
	if m.cacheValid {
		return m.cachedView
	}

	result := m.renderView()
	m.cachedView = result
	m.cacheValid = true

	return result
}

//nolint:cyclop
func (m *Viewport) renderView() string {
	m.ensureLineWidths()

	contentH := m.height
	if m.bordered {
		contentH -= borderOverhead
	}

	contentW := m.width
	if m.bordered {
		contentW -= borderOverhead
	}

	if contentH <= 0 || contentW <= 0 || len(m.lines) == 0 {
		return ""
	}

	if m.yOffset >= len(m.lines) {
		m.yOffset = 0
	}

	start := m.yOffset
	end := min(start+contentH, len(m.lines))
	showBar := m.scrollbar && (len(m.lines) > contentH || m.scrollbarReserve)

	if showBar {
		contentW -= scrollbarColWidth
	}

	m.ensurePaddedCache(contentW)

	// Zero-copy fast path: unbordered, no scrollbar, content fills viewport.
	// The output is a contiguous window into paddedBuf.
	if !m.bordered && !showBar && end == start+contentH {
		s := m.lineOffsets[start]
		e := m.lineOffsets[end] - 1

		//nolint:gosec // Zero-copy: returned string is valid until next render modifies paddedBuf.
		return unsafe.String(unsafe.SliceData(m.paddedBuf[s:e]), e-s)
	}

	// Compute scrollbar thumb position inline (no struct allocation).
	var thumbPos, thumbEnd int

	hasBar := false

	if showBar && len(m.lines) > contentH {
		hasBar = true
		thumb := max(1, contentH*contentH/len(m.lines))
		maxScroll := max(1, len(m.lines)-contentH)
		thumbPos = (contentH - thumb) * m.yOffset / maxScroll
		thumbEnd = thumbPos + thumb
	}

	if m.bordered {
		return m.renderBordered(start, end, contentW, contentH, showBar, thumbPos, thumbEnd, hasBar)
	}

	return m.renderUnbordered(start, end, contentW, contentH, showBar, thumbPos, thumbEnd, hasBar)
}

// ensurePaddedCache builds the contiguous padded-line buffer if needed.
// All padded lines are joined with '\n' into paddedBuf, and lineOffsets
// records the byte offset of each line start. Widths are computed eagerly
// so that render paths never call fastWidth.
func (m *Viewport) ensurePaddedCache(contentW int) {
	if m.paddedBufCW == contentW && len(m.lineOffsets) == len(m.lines)+1 {
		return
	}

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

	buf := m.paddedBuf[:0]
	for idx, line := range m.lines {
		m.lineOffsets[idx] = len(buf)

		lineWidth := m.lineWidths[idx]
		if lineWidth < 0 {
			lineWidth = fastWidth(line)
			m.lineWidths[idx] = lineWidth
		}

		buf = append(buf, line...)

		if lineWidth < contentW {
			buf = append(buf, spaces[:contentW-lineWidth]...)
		}

		buf = append(buf, '\n')
	}

	m.lineOffsets[len(m.lines)] = len(buf)
	m.paddedBuf = buf
	m.paddedBufCW = contentW
}

//nolint:cyclop,funlen
func (m *Viewport) renderBordered(start, end, contentW, contentH int, showBar bool, thumbPos, thumbEnd int, hasBar bool) string {
	est := 2*m.width*contentH + contentH + borderOverhead

	buf := m.renderBuf[:0]
	if cap(buf) < est {
		buf = make([]byte, 0, est)
	}

	// Top border
	buf = append(buf, m.borderTopL...)
	buf = m.appendHorizBorder(buf, contentW, showBar)
	buf = append(buf, m.borderTopR...)

	// Content lines
	for idx := start; idx < end; idx++ {
		buf = append(buf, '\n')
		buf = append(buf, m.borderLeft...)

		ls := m.lineOffsets[idx]
		le := m.lineOffsets[idx+1] - 1
		buf = append(buf, m.paddedBuf[ls:le]...)

		if showBar {
			switch {
			case !hasBar:
				buf = append(buf, m.emptyCell...)
			case idx-start >= thumbPos && idx-start < thumbEnd:
				buf = append(buf, m.thumbCell...)
			default:
				buf = append(buf, m.trackCell...)
			}
		}

		buf = append(buf, m.borderRight...)
	}

	// Fill lines
	fillStart := end - start
	for idx := fillStart; idx < contentH; idx++ {
		buf = append(buf, '\n')
		buf = append(buf, m.borderLeft...)
		buf = append(buf, spaces[:contentW]...)

		if showBar {
			switch {
			case !hasBar:
				buf = append(buf, m.emptyCell...)
			case idx >= thumbPos && idx < thumbEnd:
				buf = append(buf, m.thumbCell...)
			default:
				buf = append(buf, m.trackCell...)
			}
		}

		buf = append(buf, m.borderRight...)
	}

	// Bottom border
	buf = append(buf, '\n')
	buf = append(buf, m.borderBotL...)
	buf = m.appendHorizBorder(buf, contentW, showBar)
	buf = append(buf, m.borderBotR...)

	m.renderBuf = buf

	//nolint:gosec // Zero-copy: returned string is valid until next render overwrites the buffer.
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

//nolint:cyclop,funlen
func (m *Viewport) renderUnbordered(start, end, contentW, contentH int, showBar bool, thumbPos, thumbEnd int, hasBar bool) string {
	est := 2*m.width*contentH + contentH

	buf := m.renderBuf[:0]
	if cap(buf) < est {
		buf = make([]byte, 0, est)
	}

	// First line — no leading newline, avoids per-line branch.
	if start < end {
		ls := m.lineOffsets[start]
		le := m.lineOffsets[start+1] - 1
		buf = append(buf, m.paddedBuf[ls:le]...)

		if showBar {
			switch {
			case !hasBar:
				buf = append(buf, m.emptyCell...)
			case thumbPos == 0:
				buf = append(buf, m.thumbCell...)
			default:
				buf = append(buf, m.trackCell...)
			}
		}
	}

	// Remaining content lines
	for idx := start + 1; idx < end; idx++ {
		buf = append(buf, '\n')

		ls := m.lineOffsets[idx]
		le := m.lineOffsets[idx+1] - 1
		buf = append(buf, m.paddedBuf[ls:le]...)

		if showBar {
			visibleIdx := idx - start
			switch {
			case !hasBar:
				buf = append(buf, m.emptyCell...)
			case visibleIdx >= thumbPos && visibleIdx < thumbEnd:
				buf = append(buf, m.thumbCell...)
			default:
				buf = append(buf, m.trackCell...)
			}
		}
	}

	// Fill lines
	fillStart := end - start
	for idx := fillStart; idx < contentH; idx++ {
		buf = append(buf, '\n')
		buf = append(buf, spaces[:contentW]...)

		if showBar {
			switch {
			case !hasBar:
				buf = append(buf, m.emptyCell...)
			case idx >= thumbPos && idx < thumbEnd:
				buf = append(buf, m.thumbCell...)
			default:
				buf = append(buf, m.trackCell...)
			}
		}
	}

	m.renderBuf = buf

	//nolint:gosec // Zero-copy: returned string is valid until next render overwrites the buffer.
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// appendHorizBorder appends the styled horizontal border line.
func (m *Viewport) appendHorizBorder(buf []byte, contentW int, showBar bool) []byte {
	horizLen := contentW
	if showBar {
		horizLen += scrollbarColWidth
	}

	if m.borderStyle != "" {
		buf = append(buf, m.borderStyle...)
	}

	buf = append(buf, horizBorderBytes[:horizLen*3]...)

	if m.borderStyle != "" {
		buf = append(buf, "\x1b[0m"...)
	}

	return buf
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

func (m *Viewport) handleKeyPress(msg tea.KeyPressMsg) {
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

func (m *Viewport) handleMouseWheel(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		m.ScrollUp(mouseWheelDelta)
	case tea.MouseWheelDown:
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

// colorToAnsi renders a space with the given foreground color and extracts
// the ANSI escape sequence prefix. Returns "" if extraction fails.
func colorToAnsi(c color.Color) string {
	rendered := lipgloss.NewStyle().Foreground(c).Render(" ")

	prefixEnd := strings.LastIndex(rendered, "\x1b[")
	if prefixEnd > 0 {
		suffixStart := strings.Index(rendered, " ")
		if suffixStart > 0 {
			return rendered[:suffixStart]
		}
	}

	return ""
}
