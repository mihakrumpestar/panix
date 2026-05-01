package viewport

import (
	"fmt"
	"image/color"
	"strings"

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

func writeHorizBorder(buf *strings.Builder, n int) {
	buf.Write(horizBorderBytes[:n*3])
}

type Viewport struct {
	lines            []string
	width            int
	height           int
	yOffset          int
	scrollbar        bool
	scrollbarReserve bool
	main             bool
	bordered         bool
	maxHeight        int
	trackChar        string
	thumbChar        string
	trackStyle       string
	thumbStyle       string
	borderStyle      string
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
	return func(m *Viewport) {
		m.scrollbar = true
		m.thumbChar = thumbChar
		m.trackChar = trackChar
		m.thumbStyle = colorToAnsi(thumbColor)
		m.trackStyle = colorToAnsi(trackColor)
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
	}
}

func WithMaxHeight(h int) Option {
	return func(m *Viewport) {
		m.maxHeight = h
	}
}

func New(opts ...Option) Viewport {
	model := Viewport{
		thumbChar: "█",
		trackChar: "│",
	}

	for _, opt := range opts {
		opt(&model)
	}

	return model
}

func (m *Viewport) SetWidth(w int) {
	m.width = w
}

func (m *Viewport) SetHeight(h int) {
	m.height = h
}

func (m *Viewport) SetBorderStyle(borderColor color.Color) {
	m.borderStyle = colorToAnsi(borderColor)
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

func (m *Viewport) MaxLineWidth() int {
	maxW := 0

	for _, line := range m.lines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
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

		return m.validateLineWidths()
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

func (m *Viewport) SetContentLines(lines []string) {
	m.lines = lines

	maxOffset := max(len(lines)-m.contentHeight(), 0)

	if m.yOffset > maxOffset {
		m.yOffset = maxOffset
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

	m.yOffset = max(0, min(offset, maxOffset))
}

func (m *Viewport) GotoBottom() {
	offset := max(len(m.lines)-m.contentHeight(), 0)

	m.yOffset = offset
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

func (m *Viewport) View() string {
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

	m.clampYOffset()

	start := m.yOffset
	end := min(start+contentH, len(m.lines))
	showBar := m.scrollbar && (len(m.lines) > contentH || m.scrollbarReserve)

	if showBar {
		contentW -= scrollbarColWidth
	}

	if m.bordered {
		return m.renderBordered(start, end, contentW, contentH, showBar)
	}

	return m.renderUnbordered(start, end, contentW, contentH, showBar)
}

func (m *Viewport) clampYOffset() {
	if m.yOffset >= len(m.lines) {
		m.yOffset = 0
	}
}

func (m *Viewport) renderBordered(start, end, contentW, contentH int, showBar bool) string {
	var buf strings.Builder
	buf.Grow((m.width+1)*(contentH+borderOverhead) + borderOverhead)
	m.writeBorderTop(&buf, contentW, showBar)
	m.writeContentLines(&buf, start, end, contentW, showBar, true)
	m.writeFillLines(&buf, end-start, contentW, showBar, true, contentH)
	m.writeBorderBottom(&buf, contentW, showBar)

	return buf.String()
}

func (m *Viewport) renderUnbordered(start, end, contentW, contentH int, showBar bool) string {
	var buf strings.Builder
	buf.Grow(m.width*m.height + m.height)
	m.writeContentLines(&buf, start, end, contentW, showBar, false)
	m.writeFillLines(&buf, end-start, contentW, showBar, false, contentH)

	return buf.String()
}

func (m *Viewport) writeContentLines(buf *strings.Builder, start, end, contentW int, showBar, bordered bool) {
	for idx := start; idx < end; idx++ {
		if idx > start || bordered {
			buf.WriteByte('\n')
		}

		if bordered {
			m.writeBorderLeft(buf)
		}

		m.writePaddedLine(buf, m.lines[idx], contentW)

		if showBar {
			m.writeScrollbarCell(buf, idx-start)
		}

		if bordered {
			m.writeBorderRight(buf)
		}
	}
}

func (m *Viewport) writeFillLines(buf *strings.Builder, offset, contentW int, showBar, bordered bool, fillTo int) {
	for idx := offset; idx < fillTo; idx++ {
		buf.WriteByte('\n')

		if bordered {
			m.writeBorderLeft(buf)
		}

		buf.WriteString(spaces[:contentW])

		if showBar {
			m.writeScrollbarCell(buf, idx)
		}

		if bordered {
			m.writeBorderRight(buf)
		}
	}
}

func (m *Viewport) writePaddedLine(buf *strings.Builder, line string, contentW int) {
	lw := lipgloss.Width(line)
	if lw < contentW {
		buf.WriteString(line)
		buf.WriteString(spaces[:contentW-lw])
	} else {
		buf.WriteString(line)
	}
}

func (m *Viewport) writeBorderTop(buf *strings.Builder, contentW int, showBar bool) {
	m.writeStyledBorderChar(buf, "╭")
	m.writeStyledHorizBorder(buf, contentW, showBar)
	m.writeStyledBorderChar(buf, "╮")
}

func (m *Viewport) writeBorderBottom(buf *strings.Builder, contentW int, showBar bool) {
	buf.WriteByte('\n')
	m.writeStyledBorderChar(buf, "╰")
	m.writeStyledHorizBorder(buf, contentW, showBar)
	m.writeStyledBorderChar(buf, "╯")
}

func (m *Viewport) writeStyledHorizBorder(buf *strings.Builder, contentW int, showBar bool) {
	horizLen := contentW
	if showBar {
		horizLen += scrollbarColWidth
	}

	if m.borderStyle != "" {
		buf.WriteString(m.borderStyle)
	}

	writeHorizBorder(buf, horizLen)

	if m.borderStyle != "" {
		buf.WriteString("\x1b[0m")
	}
}

func (m *Viewport) writeBorderLeft(buf *strings.Builder) {
	m.writeStyledBorderChar(buf, "│")
}

func (m *Viewport) writeBorderRight(buf *strings.Builder) {
	m.writeStyledBorderChar(buf, "│")
}

func (m *Viewport) writeStyledBorderChar(buf *strings.Builder, ch string) {
	if m.borderStyle != "" {
		buf.WriteString(m.borderStyle)
	}

	buf.WriteString(ch)

	if m.borderStyle != "" {
		buf.WriteString("\x1b[0m")
	}
}

func (m *Viewport) writeScrollbarCell(buf *strings.Builder, visibleIdx int) {
	contentH := m.contentHeight()
	total := len(m.lines)

	buf.WriteByte(' ')

	if total <= contentH {
		// Content fits: empty scrollbar column
		buf.WriteByte(' ')

		return
	}

	thumb := max(1, contentH*contentH/total)
	pos := int(float64(contentH-thumb) * clamp(float64(m.yOffset)/float64(max(1, total-contentH)), 0, 1))
	end := pos + thumb

	isThumb := visibleIdx >= pos && visibleIdx < end
	style := m.trackStyle
	char := m.trackChar

	if isThumb {
		style = m.thumbStyle
		char = m.thumbChar
	}

	if style != "" {
		buf.WriteString(style)
	}

	buf.WriteString(char)

	if style != "" {
		buf.WriteString("\x1b[0m")
	}
}

func clamp(val, low, high float64) float64 {
	if val < low {
		return low
	}

	if val > high {
		return high
	}

	return val
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

func (m *Viewport) validateLineWidths() error {
	maxW := m.ContentWidth()

	var msgs []string

	for idx, line := range m.lines {
		w := lipgloss.Width(line)
		if w > maxW {
			msgs = append(msgs, fmt.Sprintf("line %d: visual width %d exceeds ContentWidth %d: %q", idx, w, maxW, line))
		}
	}

	if len(msgs) > 0 {
		sep := ";" + " "

		return errors.Wrapf(ErrLineOverWidth, "viewport (main, width=%d): %d line(s) over width: %s", m.width, len(msgs), strings.Join(msgs, sep))
	}

	return nil
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
