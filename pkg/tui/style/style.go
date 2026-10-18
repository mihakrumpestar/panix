package style

import (
	"bytes"
	"slices"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

// Style defines terminal styling properties: colors, bold, width, padding,
// borders, and alignment. Like lipgloss v2, Width includes borders and
// padding (the total rendered block width). All setter methods return a copy
// and do not mutate the receiver.
type Style struct {
	fgPrefix  []byte
	bgPrefix  []byte
	bold      bool
	faint     bool
	width     int
	maxWidth  int
	ellipsis  bool
	align     Position
	padTop    int
	padRight  int
	padBottom int
	padLeft   int

	border       Border
	borderFg     []byte
	hasBorder    bool
	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool

	fgColor Color
	bgColor Color

	prefix []byte
}

func NewStyle() Style {
	return Style{}
}

// GetAlign returns the alignment position.
func (s Style) GetAlign() Position {
	return s.align
}

// Foreground sets the text foreground color. Returns a copy.
func (s Style) Foreground(c Color) Style {
	s.fgPrefix = colorToFgPrefix(c)
	s.fgColor = c
	s.prefix = s.computePrefix()

	return s
}

// Background sets the text background color. Returns a copy.
func (s Style) Background(c Color) Style {
	s.bgPrefix = colorToBgPrefix(c)
	s.bgColor = c
	s.prefix = s.computePrefix()

	return s
}

// Bold sets bold text rendering. Returns a copy.
func (s Style) Bold(v bool) Style {
	s.bold = v
	s.prefix = s.computePrefix()

	return s
}

// Faint sets faint (dimmed) text rendering. Returns a copy. Used e.g. to dim
// app content behind a focused modal while keeping it visible.
func (s Style) Faint(v bool) Style {
	s.faint = v
	s.prefix = s.computePrefix()

	return s
}

// Width sets the total block width including borders and padding (lipgloss v2
// semantics). Shorter content lines are padded to fill; longer lines are not
// wrapped (use MaxWidth for truncation). Width = content + padding + borders.
// Returns a copy.
func (s Style) Width(w int) Style {
	s.width = w

	return s
}

// MaxWidth sets the maximum allowed output width. Any rendered line exceeding
// MaxWidth is truncated. When MaxWidth < Width, MaxWidth takes precedence:
// the effective block width is capped to MaxWidth so that borders, padding,
// and content all fit within the limit. Returns a copy.
func (s Style) MaxWidth(w int) Style {
	s.maxWidth = w

	return s
}

// TruncateEllipsis appends ".." to lines that are truncated due to MaxWidth,
// so overflowing content is visually distinguishable. The ellipsis replaces
// the last 2 cells of the truncated line. Returns a copy.
func (s Style) TruncateEllipsis(v bool) Style {
	s.ellipsis = v

	return s
}

// Align sets horizontal text alignment within the block (Left, Center, Right).
// Returns a copy.
func (s Style) Align(p Position) Style {
	s.align = p

	return s
}

// Padding sets uniform vertical and horizontal padding (in cells). Padding is
// clamped to [0, maxPaddingCells] so the pre-rendered pad buffer can never be
// indexed out of bounds. Padding is included in the block Width. Returns a copy.
func (s Style) Padding(vertical, horizontal int) Style {
	vertical = clampPadding(vertical)
	horizontal = clampPadding(horizontal)
	s.padTop = vertical
	s.padBottom = vertical
	s.padLeft = horizontal
	s.padRight = horizontal

	return s
}

// PaddingTop sets top padding, clamped to [0, maxPaddingCells]. Returns a copy.
func (s Style) PaddingTop(v int) Style {
	s.padTop = clampPadding(v)

	return s
}

// PaddingRight sets right padding, clamped to [0, maxPaddingCells]. Returns a copy.
func (s Style) PaddingRight(v int) Style {
	s.padRight = clampPadding(v)

	return s
}

// PaddingBottom sets bottom padding, clamped to [0, maxPaddingCells]. Returns a copy.
func (s Style) PaddingBottom(v int) Style {
	s.padBottom = clampPadding(v)

	return s
}

// PaddingLeft sets left padding, clamped to [0, maxPaddingCells]. Returns a copy.
func (s Style) PaddingLeft(v int) Style {
	s.padLeft = clampPadding(v)

	return s
}

// Border sets the border style and which sides to draw. With no sides
// argument, all four sides are drawn. With 4 bools: top, right, bottom, left.
// When Width is not set, the width is auto-derived from the widest content
// line plus padding and borders. Returns a copy.
//

func (s Style) Border(b Border, sides ...bool) Style {
	s.border = b
	s.hasBorder = true

	switch len(sides) {
	case 0:
		s.borderTop = true
		s.borderRight = true
		s.borderBottom = true
		s.borderLeft = true
	case 4: //nolint:mnd
		s.borderTop = sides[0]
		s.borderRight = sides[1]
		s.borderBottom = sides[2]
		s.borderLeft = sides[3]
	default:
		s.borderTop = true
		s.borderRight = true
		s.borderBottom = true
		s.borderLeft = true
	}

	return s
}

// BorderForeground sets the foreground color for all border edges. Returns a copy.
func (s Style) BorderForeground(c Color) Style {
	s.borderFg = colorToFgPrefix(c)

	return s
}

// BorderTopForeground sets the foreground color for the top border edge. Returns a copy.
func (s Style) BorderTopForeground(c Color) Style {
	s.border.topFg = colorToFgPrefix(c)

	return s
}

// BorderRightForeground sets the foreground color for the right border edge. Returns a copy.
func (s Style) BorderRightForeground(c Color) Style {
	s.border.rightFg = colorToFgPrefix(c)

	return s
}

// BorderBottomForeground sets the foreground color for the bottom border edge. Returns a copy.
func (s Style) BorderBottomForeground(c Color) Style {
	s.border.bottomFg = colorToFgPrefix(c)

	return s
}

// BorderLeftForeground sets the foreground color for the left border edge. Returns a copy.
func (s Style) BorderLeftForeground(c Color) Style {
	s.border.leftFg = colorToFgPrefix(c)

	return s
}

// GetForeground returns the foreground color, or nil if unset.
func (s Style) GetForeground() Color { return s.fgColor }

func (s Style) GetBackground() Color { return s.bgColor }

// BorderSides tells which border sides a style draws.
type BorderSides struct {
	Top    bool
	Right  bool
	Bottom bool
	Left   bool
}

// GetBorder returns the border glyphs and which sides are drawn. When no
// border is set the glyphs are empty and no side is drawn.
func (s Style) GetBorder() (Border, BorderSides) {
	return s.border, BorderSides{
		Top:    s.borderTop,
		Right:  s.borderRight,
		Bottom: s.borderBottom,
		Left:   s.borderLeft,
	}
}

// GetWidth returns the explicit width set on the style, or 0 if none.
func (s Style) GetWidth() int { return s.width }

// TruncateToWidth truncates str to at most maxW cells, preserving ANSI escape
// sequences. When ellipsis is true and truncation happens, ".." replaces the
// last 2 cells. Exported for zero-allocation cell rendering.
func TruncateToWidth(str []byte, maxW int, ellipsis bool) []byte {
	return truncateToWidth(str, maxW, ellipsis)
}

// Bytes returns the ANSI escape sequence for this style's color/bold
// attributes, without applying any layout.
func (s Style) Bytes() []byte {
	buf := s.stylePrefix()

	if len(buf) > 0 {
		buf = append(buf, ansiReset...)
	}

	return buf
}

// RenderIntoBuf renders content through the full style pipeline into dst. The
// color-only path avoids a [][]byte allocation; call Reset on dst to overwrite.
func (s Style) RenderIntoBuf(dst *buffer.LinesBuf, content *buffer.LinesBuf) {
	if content == nil || content.Len() == 0 {
		if !s.hasBorder && s.padTop == 0 && s.padBottom == 0 && s.width == 0 {
			return
		}
	}

	if !s.hasLayoutProperties() {
		s.renderColorOnlyIntoBuf(dst, content)

		return
	}

	// Layout requires [][]byte; build a temporary view.
	lines := content.Lines()
	result := s.applyLayout(lines)
	s.renderColorOnlyInto(dst, result)
}

// RenderLine renders a single line through the full style pipeline
// (padding, alignment, borders, color) and returns the first result line.
// For the common color-only case, avoids the [][]byte allocation.
func (s Style) RenderLine(line []byte) []byte {
	if !s.hasLayoutProperties() {
		if s.fgPrefix == nil && s.bgPrefix == nil && !s.bold && !s.faint {
			return line
		}

		prefix := s.stylePrefix()
		reset := ansiReset

		if !bytes.Contains(line, reset) {
			buf := make([]byte, 0, len(prefix)+len(line)+len(reset))
			buf = append(buf, prefix...)
			buf = append(buf, line...)
			buf = append(buf, reset...)

			return buf
		}

		var buf []byte

		buf = append(buf, prefix...)
		buf = appendWithResetReemitBytes(buf, line, prefix, reset)
		buf = append(buf, reset...)

		return buf
	}

	result := s.applyLayout([][]byte{line})

	styled := s.renderColorOnly(result)
	if len(styled) == 0 {
		return nil
	}

	return styled[0]
}

// AppendStyledLine is the zero-allocation form of writing s.RenderLine(cell) for
// a style without layout properties. The prefix is re-emitted after every ANSI
// reset in cell so styling persists across pre-styled content.
func (s Style) AppendStyledLine(buf *buffer.LineBuf, cell []byte) {
	prefix := s.stylePrefix()
	reset := ansiReset

	if len(prefix) == 0 && len(reset) == 0 {
		buf.Write(cell)

		return
	}

	if len(prefix) == 0 {
		buf.Write(cell)
		buf.Write(reset)

		return
	}

	buf.Write(prefix)
	writeWithResetReemit(buf, cell, prefix, reset)
	buf.Write(reset)
}

// AppendStyledPad appends n space bytes styled with the receiver's
// color/bold attributes directly into buf. This is the zero-allocation
// equivalent of buf.Write(s.RenderLine(spaces[:n])...).
func (s Style) AppendStyledPad(buf *buffer.LineBuf, n int) {
	if n <= 0 {
		return
	}

	pad := prerenderedPadding[:min(n, len(prerenderedPadding))]

	s.AppendStyledLine(buf, pad)
}

// RenderLineInto renders a single line through the full style pipeline and
// writes the result into dst. For the common no-layout case (color-only),
// this is zero-allocation: it writes prefix+line+reset directly.
func (s Style) RenderLineInto(dst *buffer.LinesBuf, line []byte) {
	if !s.hasLayoutProperties() {
		prefix := s.stylePrefix()
		reset := ansiReset

		switch {
		case len(prefix) == 0 && len(reset) == 0:
			dst.WriteLine(line)
		case len(prefix) == 0:
			dst.WriteLine2(line, reset)
		case !bytes.Contains(line, reset):
			dst.WriteLine3(prefix, line, reset)
		default:
			dst.EmptyLine()
			s.RenderAppend(dst, line)
		}

		return
	}

	result := s.applyLayout([][]byte{line})
	s.renderColorOnlyInto(dst, result)
}

// RenderAppend appends content with the style's colors and bold to the current
// line of dst, ignoring layout properties. The prefix is re-emitted after every
// ANSI reset in content so styling persists across pre-styled cells.
func (s Style) RenderAppend(dst *buffer.LinesBuf, content []byte) {
	if dst.Len() == 0 {
		s.RenderLineInto(dst, content)

		return
	}

	prefix := s.stylePrefix()
	reset := ansiReset

	if len(prefix) == 0 && len(reset) == 0 {
		dst.AppendToLine(content)

		return
	}

	if len(prefix) == 0 {
		dst.AppendToLine(content, reset)

		return
	}

	dst.AppendToLine(prefix)
	appendWithResetReemit(dst, content, prefix, reset)
	dst.AppendToLine(reset)
}

// renderColorOnlyInto writes color/bold styling directly into dst, avoiding the
// contiguous buffer and [][]byte of renderColorOnly. The prefix is re-emitted
// after every ANSI reset so styling persists across pre-styled content.
func (s Style) renderColorOnlyInto(dst *buffer.LinesBuf, content [][]byte) {
	if content == nil {
		return
	}

	prefix := s.stylePrefix()
	reset := ansiReset

	if len(prefix) == 0 && len(reset) == 0 {
		for _, line := range content {
			dst.WriteLine(line)
		}

		return
	}

	if len(prefix) == 0 {
		for _, line := range content {
			dst.WriteLine2(line, reset)
		}

		return
	}

	for _, line := range content {
		if !bytes.Contains(line, reset) {
			dst.WriteLine3(prefix, line, reset)

			continue
		}

		dst.EmptyLine()
		dst.AppendToLine(prefix)
		appendWithResetReemit(dst, line, prefix, reset)
	}
}

// renderColorOnlyIntoBuf styles every line into dst with zero-alloc Line access,
// re-emitting the prefix after every ANSI reset.
func (s Style) renderColorOnlyIntoBuf(dst *buffer.LinesBuf, content *buffer.LinesBuf) {
	if content.Len() == 0 {
		return
	}

	if s.fgPrefix == nil && s.bgPrefix == nil && !s.bold && !s.faint {
		writeBufLines(dst, content)

		return
	}

	prefix := s.stylePrefix()
	reset := ansiReset

	if len(prefix) == 0 && len(reset) == 0 {
		writeBufLines(dst, content)

		return
	}

	if len(prefix) == 0 {
		writeBufLinesWithReset(dst, content, reset)

		return
	}

	writeBufLinesStyled(dst, content, prefix, reset)
}

// writeBufLines writes all lines from content into dst as-is.
func writeBufLines(dst *buffer.LinesBuf, content *buffer.LinesBuf) {
	for i := range content.Len() {
		dst.WriteLine(content.Line(i))
	}
}

// writeBufLinesWithReset writes all lines from content with a reset suffix.
func writeBufLinesWithReset(dst *buffer.LinesBuf, content *buffer.LinesBuf, reset []byte) {
	for i := range content.Len() {
		dst.WriteLine2(content.Line(i), reset)
	}
}

// writeBufLinesStyled writes all lines with prefix and reset, handling embedded resets.
func writeBufLinesStyled(dst *buffer.LinesBuf, content *buffer.LinesBuf, prefix, reset []byte) {
	for i := range content.Len() {
		line := content.Line(i)
		if !bytes.Contains(line, reset) {
			dst.WriteLine3(prefix, line, reset)

			continue
		}

		dst.EmptyLine()
		dst.AppendToLine(prefix)
		appendWithResetReemit(dst, line, prefix, reset)
	}
}

// appendWithResetReemit appends content to the current line of dst.
// After every occurrence of reset in content, prefix is re-emitted.
func appendWithResetReemit(dst *buffer.LinesBuf, content, prefix, reset []byte) {
	for len(content) > 0 {
		idx := bytes.Index(content, reset)
		if idx < 0 {
			dst.AppendToLine(content)

			break
		}

		end := idx + len(reset)
		dst.AppendToLine(content[:end])
		dst.AppendToLine(prefix)

		content = content[end:]
	}
}

// applyLayout applies padding, alignment, borders, and width constraints.
func (s Style) applyLayout(content [][]byte) [][]byte {
	if derived := s.deriveBorderWidth(content); derived > 0 {
		s.width = derived
	}

	if s.maxWidth > 0 && s.width > s.maxWidth {
		s.width = s.maxWidth
	}

	content = s.applyHorizontalPadding(content)
	content = s.applyVerticalPadding(content)

	borderWidth := s.borderCharWidth()
	innerWidth := max(s.width-borderWidth, 0)
	alignLines(content, innerWidth, s.maxWidth, borderWidth, s.align, s.ellipsis)

	if s.hasBorder {
		content = s.applyBorder(content)
	}

	return content
}

// hasLayoutProperties reports whether any layout properties are set.
func (s Style) hasLayoutProperties() bool {
	return s.width > 0 || s.maxWidth > 0 || s.hasBorder ||
		s.padTop > 0 || s.padRight > 0 || s.padBottom > 0 || s.padLeft > 0
}

// maxPaddingCells is the largest padding the renderer accepts on one side. The
// pre-rendered space buffer is this size; padding values are clamped to it so any
// caller (including a malformed theme) can never panic on a slice bound.
const maxPaddingCells = 1000

var prerenderedPadding = slices.Repeat([]byte(" "), maxPaddingCells)

// clampPadding confines a padding value to the pre-rendered buffer range.
func clampPadding(n int) int {
	return max(min(n, maxPaddingCells), 0)
}

// PaddingBytes returns a slice of n space bytes for padding.
// The returned slice shares a static buffer; do not modify it.
func PaddingBytes(n int) []byte {
	return prerenderedPadding[:clampPadding(n)]
}

// applyHorizontalPadding adds left/right padding to each line in-place.
func (s Style) applyHorizontalPadding(content [][]byte) [][]byte {
	left := clampPadding(s.padLeft)
	right := clampPadding(s.padRight)

	if left <= 0 && right <= 0 {
		return content
	}

	for idx := range content {
		line := content[idx]
		size := left + len(line) + right
		buf := make([]byte, size)
		off := copy(buf, PaddingBytes(left))
		off += copy(buf[off:], line)
		copy(buf[off:], PaddingBytes(right))
		content[idx] = buf
	}

	return content
}

// applyVerticalPadding prepends top and appends bottom padding lines.
func (s Style) applyVerticalPadding(content [][]byte) [][]byte {
	top := clampPadding(s.padTop)
	bottom := clampPadding(s.padBottom)

	if top > 0 {
		padLine := s.padLine()

		result := make([][]byte, top+len(content))
		for i := range top {
			result[i] = padLine
		}

		copy(result[top:], content)
		content = result
	}

	if bottom > 0 {
		padLine := s.padLine()
		oldLen := len(content)

		content = append(content, make([][]byte, bottom)...)
		for i := range bottom {
			content[oldLen+i] = padLine
		}
	}

	return content
}

// padLine returns a blank line matching the style's width and horizontal padding.
func (s Style) padLine() []byte {
	toPad := clampPadding(s.padLeft) + clampPadding(s.padRight)

	// Content width
	if s.width != 0 {
		toPad = s.width
	}

	return PaddingBytes(toPad)
}

// borderCharWidth returns the number of border character columns (left + right).
// Border glyphs are single-cell by registry invariant (see
// TestBorderPresets_SingleCellGlyphs), so each drawn side is one column.
func (s Style) borderCharWidth() int {
	if !s.hasBorder {
		return 0
	}

	borderWidth := 0
	if s.borderLeft {
		borderWidth++
	}

	if s.borderRight {
		borderWidth++
	}

	return borderWidth
}

// alignLines pads or truncates each line to targetWidth according to the
// given alignment, falling back to maxWidthInner when innerWidth is 0.
func alignLines(lines [][]byte, innerWidth, maxWidth, borderWidth int, align Position, ellipsis bool) {
	if innerWidth <= 0 && maxWidth <= 0 {
		return
	}

	maxWidthInner := computeMaxWidthInner(maxWidth, innerWidth, borderWidth)
	targetWidth := resolveTargetWidth(innerWidth, maxWidthInner)

	for lineIdx, line := range lines {
		alignLine(lines, lineIdx, line, targetWidth, align, ellipsis)
	}
}

func computeMaxWidthInner(maxWidth, innerWidth, borderWidth int) int {
	if maxWidth > 0 && innerWidth == 0 {
		return max(maxWidth-borderWidth, 0)
	}

	return 0
}

func resolveTargetWidth(innerWidth, maxWidthInner int) int {
	if innerWidth > 0 {
		return innerWidth
	}

	return maxWidthInner
}

func alignLine(lines [][]byte, lineIdx int, line []byte, targetWidth int, align Position, ellipsis bool) {
	lineWidth := CellWidth(line)

	if lineWidth < targetWidth {
		lines[lineIdx] = padLineAlignment(line, targetWidth-lineWidth, align)
	} else if lineWidth > targetWidth && targetWidth > 0 {
		lines[lineIdx] = truncateToWidth(line, targetWidth, ellipsis)
	}
}

// padLineAlignment pads a line to the given alignment.
func padLineAlignment(line []byte, pad int, align Position) []byte {
	size := len(line) + pad
	buf := make([]byte, size)

	switch {
	case align == Center:
		left := pad / 2
		right := pad - left
		off := copy(buf, PaddingBytes(left))
		off += copy(buf[off:], line)
		copy(buf[off:], PaddingBytes(right))
	case align >= Right:
		off := copy(buf, PaddingBytes(pad))
		copy(buf[off:], line)
	default:
		off := copy(buf, line)
		copy(buf[off:], PaddingBytes(pad))
	}

	return buf
}

// deriveBorderWidth returns the width derived from the widest content line when
// a border is present but no explicit width has been set. Returns 0 if the
// width should not be overridden.
func (s Style) deriveBorderWidth(content [][]byte) int {
	if !s.hasBorder || s.width != 0 {
		return 0
	}

	maxW := 0
	for _, line := range content {
		if w := CellWidth(line); w > maxW {
			maxW = w
		}
	}

	if maxW == 0 {
		return 0
	}

	borderWidth := 0
	if s.borderLeft {
		borderWidth++
	}

	if s.borderRight {
		borderWidth++
	}

	return maxW + s.padLeft + s.padRight + borderWidth
}

// renderColorOnly applies ANSI foreground/background/bold sequences to every
// line of content, without any layout (width, padding, borders).

func (s Style) renderColorOnly(content [][]byte) [][]byte {
	if content == nil {
		return nil
	}

	if s.fgPrefix == nil && s.bgPrefix == nil && !s.bold && !s.faint {
		return content
	}

	prefix := s.stylePrefix()
	reset := ansiReset

	if !hasEmbeddedReset(content, reset) {
		return renderColorOnlyFast(content, prefix, reset)
	}

	result := make([][]byte, len(content))
	for idx, line := range content {
		var buf []byte

		buf = append(buf, prefix...)
		buf = appendWithResetReemitBytes(buf, line, prefix, reset)
		buf = append(buf, reset...)
		result[idx] = buf
	}

	return result
}

func hasEmbeddedReset(content [][]byte, reset []byte) bool {
	for _, line := range content {
		if bytes.Contains(line, reset) {
			return true
		}
	}

	return false
}

func renderColorOnlyFast(content [][]byte, prefix, reset []byte) [][]byte {
	prefixLen := len(prefix)
	resetLen := len(reset)
	perLine := prefixLen + resetLen

	totalSize := 0
	for _, line := range content {
		totalSize += perLine + len(line)
	}

	buf := make([]byte, totalSize)
	result := make([][]byte, len(content))

	off := 0
	for idx, line := range content {
		start := off
		off += copy(buf[off:], prefix)
		off += copy(buf[off:], line)
		off += copy(buf[off:], reset)
		result[idx] = buf[start:off]
	}

	return result
}

func appendWithResetReemitBytes(buf, content, prefix, reset []byte) []byte {
	for len(content) > 0 {
		idx := bytes.Index(content, reset)
		if idx < 0 {
			buf = append(buf, content...)

			break
		}

		end := idx + len(reset)
		buf = append(buf, content[:end]...)
		buf = append(buf, prefix...)
		content = content[end:]
	}

	return buf
}

// writeWithResetReemit writes content into buf, re-emitting prefix after every
// ANSI reset so a parent style persists across pre-styled content.
func writeWithResetReemit(buf *buffer.LineBuf, content, prefix, reset []byte) {
	for len(content) > 0 {
		idx := bytes.Index(content, reset)
		if idx < 0 {
			buf.Write(content)

			break
		}

		end := idx + len(reset)
		buf.Write(content[:end])
		buf.Write(prefix)

		content = content[end:]
	}
}

// stylePrefix builds the ANSI escape sequence for fg/bg/bold.
func (s Style) stylePrefix() []byte {
	return s.prefix
}

func (s Style) computePrefix() []byte {
	size := 0
	if s.fgPrefix != nil {
		size += len(s.fgPrefix)
	}

	if s.bgPrefix != nil {
		size += len(s.bgPrefix)
	}

	if s.bold {
		size += len(ansiBold)
	}

	if s.faint {
		size += len(ansiFaint)
	}

	if size == 0 {
		return nil
	}

	buf := make([]byte, size)
	off := 0

	if s.fgPrefix != nil {
		off += copy(buf[off:], s.fgPrefix)
	}

	if s.bgPrefix != nil {
		off += copy(buf[off:], s.bgPrefix)
	}

	if s.bold {
		copy(buf[off:], ansiBold)
		off += len(ansiBold)
	}

	if s.faint {
		copy(buf[off:], ansiFaint)
	}

	return buf
}

// applyBorder wraps each line with vertical border runes and adds
// horizontal border lines at the top and/or bottom.
func (s Style) applyBorder(lines [][]byte) [][]byte {
	if len(lines) == 0 {
		return lines
	}

	topFg, rightFg, bottomFg, leftFg, reset := s.borderColors()

	resultCap := len(lines)
	if s.borderTop {
		resultCap++
	}

	if s.borderBottom {
		resultCap++
	}

	result := make([][]byte, 0, resultCap)

	if s.borderTop {
		topBorder := s.buildHorizontalBorder(s.border.TopLeft, s.border.Horizontal, s.border.TopRight, topFg, reset)
		result = append(result, topBorder)
	}

	for _, line := range lines {
		result = append(result, s.wrapBorderLine(line, leftFg, rightFg, reset))
	}

	if s.borderBottom {
		bottomBorder := s.buildHorizontalBorder(s.border.BottomLeft, s.border.Horizontal, s.border.BottomRight, bottomFg, reset)
		result = append(result, bottomBorder)
	}

	return result
}

// borderColors resolves the per-side border foreground colors and reset sequence.
func (s Style) borderColors() ([]byte, []byte, []byte, []byte, []byte) {
	borderFg := s.borderFg
	if len(borderFg) == 0 {
		borderFg = s.fgPrefix
	}

	topFg := s.border.topFg
	if len(topFg) == 0 {
		topFg = borderFg
	}

	rightFg := s.border.rightFg
	if len(rightFg) == 0 {
		rightFg = borderFg
	}

	bottomFg := s.border.bottomFg
	if len(bottomFg) == 0 {
		bottomFg = borderFg
	}

	leftFg := s.border.leftFg
	if len(leftFg) == 0 {
		leftFg = borderFg
	}

	var reset []byte
	if len(topFg) > 0 || len(rightFg) > 0 || len(bottomFg) > 0 || len(leftFg) > 0 {
		reset = ansiReset
	}

	return topFg, rightFg, bottomFg, leftFg, reset
}

// wrapBorderLine wraps a content line with left/right vertical borders.
func (s Style) wrapBorderLine(line, leftFg, rightFg, reset []byte) []byte {
	size := len(line)

	var leftN, rightN int

	if s.borderLeft {
		leftN = len(leftFg) + len(s.border.Vertical) + len(reset)
		size += leftN
	}

	if s.borderRight {
		rightN = len(rightFg) + len(s.border.Vertical) + len(reset)
		size += rightN
	}

	buf := make([]byte, size)
	off := 0

	if s.borderLeft {
		off += copy(buf[off:], leftFg)
		off += copy(buf[off:], s.border.Vertical)
		off += copy(buf[off:], reset)
	}

	off += copy(buf[off:], line)

	if s.borderRight {
		off += copy(buf[off:], rightFg)
		off += copy(buf[off:], s.border.Vertical)
		copy(buf[off:], reset)
	}

	return buf
}

// buildHorizontalBorder renders a top or bottom border line: corner + fill + corner.
// The fill width is s.width minus the left and right border rune cells. Border
// glyphs are single-cell by registry invariant (see
// TestBorderPresets_SingleCellGlyphs), so the rune cell counts are exact.
func (s Style) buildHorizontalBorder(left, mid, right, borderFg, reset []byte) []byte {
	width := max(s.width, 0)

	if s.borderLeft {
		width--
	}

	if s.borderRight {
		width--
	}

	size := len(borderFg) + len(left) + len(mid)*width + len(right) + len(reset)
	buf := make([]byte, size)
	off := 0

	off += copy(buf[off:], borderFg)
	off += copy(buf[off:], left)

	for range width {
		off += copy(buf[off:], mid)
	}

	off += copy(buf[off:], right)
	copy(buf[off:], reset)

	return buf
}

// truncateToWidth truncates str to at most maxW cells, preserving ANSI escapes.
// With ellipsis, ".." replaces the last 2 cells. Widths come from scanGlyph, the
// same shared scanner CellWidth, wrapping and splicing use, so wide and measured
// glyph sequences are measured consistently.
func truncateToWidth(line []byte, maxW int, ellipsis bool) []byte {
	if maxW <= 0 {
		return nil
	}

	pos, needsTruncation := scanTruncationPoint(line, maxW)

	if !needsTruncation {
		return line
	}

	// Content exceeds maxW; truncate.
	if ellipsis && maxW > 2 {
		truncated := truncateToWidth(line, maxW-2, false)
		buf := make([]byte, len(truncated)+2)
		off := copy(buf, truncated)
		copy(buf[off:], "..")

		return buf
	}

	return line[:pos]
}

// scanTruncationPoint scans str and returns the byte position where content
// would exceed maxW, plus whether truncation is needed.
func scanTruncationPoint(line []byte, maxW int) (int, bool) {
	width := 0
	pos := 0

	for pos < len(line) {
		char := line[pos]

		if char == '\x1b' {
			pos = SkipANSI(line, pos)

			continue
		}

		size, glyphWidth, _ := scanGlyph(line, pos)

		if width+glyphWidth > maxW {
			return pos, true
		}

		width += glyphWidth
		pos += size
	}

	return pos, false
}
