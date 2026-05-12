package style

import (
	"slices"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/rivo/uniseg"
)

// Style defines terminal styling properties: colors, bold, width, padding,
// borders, and alignment. Like lipgloss v2, Width includes borders and
// padding (the total rendered block width). All setter methods return a copy
// and do not mutate the receiver.
type Style struct {
	fgPrefix  []byte
	bgPrefix  []byte
	bold      bool
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
// included in the block Width. Returns a copy.
func (s Style) Padding(vertical, horizontal int) Style {
	s.padTop = vertical
	s.padBottom = vertical
	s.padLeft = horizontal
	s.padRight = horizontal

	return s
}

// PaddingTop sets top padding. Returns a copy.
func (s Style) PaddingTop(v int) Style {
	s.padTop = v

	return s
}

// PaddingRight sets right padding. Returns a copy.
func (s Style) PaddingRight(v int) Style {
	s.padRight = v

	return s
}

// PaddingBottom sets bottom padding. Returns a copy.
func (s Style) PaddingBottom(v int) Style {
	s.padBottom = v

	return s
}

// PaddingLeft sets left padding. Returns a copy.
func (s Style) PaddingLeft(v int) Style {
	s.padLeft = v

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

// GetWidth returns the explicit width set on the style, or 0 if none.
func (s Style) GetWidth() int { return s.width }

// GetAlign returns the text alignment set on the style.
func (s Style) GetAlign() Position { return s.align }

// TruncateToWidth truncates str to at most maxW visible cell width, preserving
// any ANSI escape sequences. When ellipsis is true and the string is actually
// truncated, ".." replaces the last 2 cells so overflowing content is visually
// distinguishable. This is the exported version of truncateToWidth for use by
// packages that need zero-allocation cell rendering.
func TruncateToWidth(str []byte, maxW int, ellipsis bool) []byte {
	return truncateToWidth(str, maxW, ellipsis)
}

// FgPrefix returns the pre-computed ANSI foreground prefix string.
func (s Style) FgPrefix() []byte {
	return s.fgPrefix
}

// BgPrefix returns the pre-computed ANSI background prefix string.
func (s Style) BgPrefix() []byte {
	return s.bgPrefix
}

// StylePrefix returns the cached ANSI escape sequence for this style's
// color/bold attributes (fg + bg + bold combined). Nil if no styling.
func (s Style) StylePrefix() []byte {
	return s.prefix
}

// HasLayoutProperties reports whether this style has layout properties
// (width, padding, borders) that require the full rendering pipeline.
func (s Style) HasLayoutProperties() bool {
	return s.hasLayoutProperties()
}

// String returns the ANSI escape sequence for this style's color/bold
// attributes, without applying any layout.
func (s Style) Bytes() []byte {
	buf := s.stylePrefix()

	if len(buf) > 0 {
		buf = append(buf, ansiReset...)
	}

	return buf
}

// Render applies the style to the given content and returns the result.
//
// Layout pipeline (matches lipgloss v2 semantics where Width includes borders):
//
//  1. Auto-derive width from content when border is set without explicit Width
//  2. Cap effective width to MaxWidth when MaxWidth < Width
//  3. Apply horizontal padding (padLeft, padRight)
//  4. Apply vertical padding (padTop, padBottom)
//  5. Compute contentWidth = width - borders - padding
//  6. Pad or truncate each line to contentWidth (using alignment)
//  7. Apply borders around the content
//  8. Apply ANSI color/bold sequences to every line
func (s Style) Render(content [][]byte) [][]byte {
	if content == nil && !s.hasBorder && s.padTop == 0 && s.padBottom == 0 && s.width == 0 {
		return nil
	}

	if !s.hasLayoutProperties() {
		return s.renderColorOnly(content)
	}

	result := s.applyLayout(content)

	return s.renderColorOnly(result)
}

// RenderInto renders content through the full style pipeline and writes
// the result lines into dst. Equivalent to dst.WriteLines(s.Render(content))
// but avoids the intermediate [][]byte allocation for the color pass.
// Call Reset() on dst before if you want to overwrite it.
func (s Style) RenderInto(dst *buffer.LinesBuf, content [][]byte) {
	if content == nil && !s.hasBorder && s.padTop == 0 && s.padBottom == 0 && s.width == 0 {
		return
	}

	if !s.hasLayoutProperties() {
		s.renderColorOnlyInto(dst, content)

		return
	}

	result := s.applyLayout(content)
	s.renderColorOnlyInto(dst, result)
}

// RenderLine renders a single line through the full style pipeline
// (padding, alignment, borders, color) and returns the first result line.
// Equivalent to Render([][]byte{line})[0] but avoids the outer slice allocation.
func (s Style) RenderLine(line []byte) []byte {
	result := s.Render([][]byte{line})
	if len(result) == 0 {
		return nil
	}

	return result[0]
}

// RenderLineInto renders a single line through the full style pipeline and
// writes the result into dst. For the common no-layout case (color-only),
// this is zero-allocation: it writes prefix+line+reset directly.
func (s Style) RenderLineInto(dst *buffer.LinesBuf, line []byte) {
	if !s.hasLayoutProperties() {
		prefix := s.stylePrefix()
		if len(prefix) == 0 && len(ansiReset) == 0 {
			dst.WriteLine(line)
		} else {
			dst.WriteLine3(prefix, line, ansiReset)
		}

		return
	}

	result := s.applyLayout([][]byte{line})
	s.renderColorOnlyInto(dst, result)
}

// renderColorOnlyInto applies ANSI foreground/background/bold sequences to
// every line of content, writing directly into dst. This avoids the contiguous
// buffer + [][]byte slice allocation of renderColorOnly.
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

	for _, line := range content {
		dst.WriteLine3(prefix, line, reset)
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

var prerenderedPadding = slices.Repeat([]byte(" "), 1000)

// PaddingBytes returns a slice of n space bytes for padding.
// The returned slice shares a static buffer; do not modify it.
func PaddingBytes(n int) []byte {
	if n > len(prerenderedPadding) {
		n = len(prerenderedPadding)
	}

	return prerenderedPadding[:n]
}

// applyHorizontalPadding adds left/right padding to each line in-place.
func (s Style) applyHorizontalPadding(content [][]byte) [][]byte {
	if s.padLeft <= 0 && s.padRight <= 0 {
		return content
	}

	for idx := range content {
		line := content[idx]
		size := s.padLeft + len(line) + s.padRight
		buf := make([]byte, size)
		off := copy(buf, prerenderedPadding[:s.padLeft])
		off += copy(buf[off:], line)
		copy(buf[off:], prerenderedPadding[:s.padRight])
		content[idx] = buf
	}

	return content
}

// applyVerticalPadding prepends top and appends bottom padding lines.
func (s Style) applyVerticalPadding(content [][]byte) [][]byte {
	if s.padTop > 0 {
		padLine := s.padLine()

		result := make([][]byte, s.padTop+len(content))
		for i := range s.padTop {
			result[i] = padLine
		}

		copy(result[s.padTop:], content)
		content = result
	}

	if s.padBottom > 0 {
		padLine := s.padLine()
		oldLen := len(content)

		content = append(content, make([][]byte, s.padBottom)...)
		for i := range s.padBottom {
			content[oldLen+i] = padLine
		}
	}

	return content
}

// padLine returns a blank line matching the style's width and horizontal padding.
func (s Style) padLine() []byte {
	toPad := s.padLeft + s.padRight

	// Content width
	if s.width != 0 {
		toPad = s.width
	}

	return prerenderedPadding[:toPad]
}

// borderCharWidth returns the number of border character columns (left + right).
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
		left := pad / 2 //nolint:mnd
		right := pad - left
		off := copy(buf, prerenderedPadding[:left])
		off += copy(buf[off:], line)
		copy(buf[off:], prerenderedPadding[:right])
	case align >= Right:
		off := copy(buf, prerenderedPadding[:pad])
		copy(buf[off:], line)
	default:
		off := copy(buf, line)
		copy(buf[off:], prerenderedPadding[:pad])
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

	if s.fgPrefix == nil && s.bgPrefix == nil && !s.bold {
		return content
	}

	prefix := s.stylePrefix()
	reset := ansiReset
	prefixLen := len(prefix)
	resetLen := len(reset)
	perLine := prefixLen + resetLen

	// Single contiguous buffer for all line data — 1 malloc instead of N.
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
	leftN := 0
	rightN := 0

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
// The fill width is s.width minus the left and right border rune cells.
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

// truncateToWidth truncates str to at most maxW visible cell width, preserving
// any ANSI escape sequences. When ellipsis is true and the string is actually
// truncated, ".." replaces the last 2 cells so overflowing content is visually
// distinguishable.
//
// Uses uniseg for grapheme cluster width (consistent with CellWidth) so emoji
// and other wide characters are measured correctly.
func truncateToWidth(line []byte, maxW int, ellipsis bool) []byte {
	if maxW <= 0 {
		return nil
	}

	pos, needsTruncation := scanTruncationPoint(line, maxW)

	if !needsTruncation {
		return line
	}

	// Content exceeds maxW — truncate.
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
	graphemeState := -1

	for pos < len(line) {
		char := line[pos]

		if char == '\x1b' {
			graphemeState = -1
			pos = skipANSI(line, pos)

			continue
		}

		if char >= 0x20 && char < 0x7F {
			graphemeState = -1

			if width+1 > maxW {
				return pos, true
			}

			width++
			pos++

			continue
		}

		if char < 0x80 { //nolint:mnd
			pos++
			graphemeState = -1

			continue
		}

		// Non-ASCII: use uniseg for proper grapheme cluster width
		// (emoji ZWJ, skin tone modifiers, CJK wide chars, etc.)
		_, rest, clusterWidth, newState := uniseg.FirstGraphemeCluster(line[pos:], graphemeState)
		graphemeState = newState

		if width+clusterWidth > maxW {
			return pos, true
		}

		width += clusterWidth
		pos = len(line) - len(rest)
	}

	return pos, false
}
