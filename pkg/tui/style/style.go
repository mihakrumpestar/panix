// Based on charm.land/lipgloss/v2 — Copyright (c) 2021-2026 Charmbracelet, Inc.
// Licensed under the MIT License. See pkg/LICENSE for details.

package style

import (
	"strings"
	"unsafe"

	"github.com/rivo/uniseg"
)

// Style defines terminal styling properties: colors, bold, width, padding,
// borders, and alignment. Like lipgloss v2, Width includes borders and
// padding (the total rendered block width). All setter methods return a copy
// and do not mutate the receiver.
type Style struct {
	fgPrefix  string
	bgPrefix  string
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
	borderFg     string
	hasBorder    bool
	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool

	fgColor Color
	bgColor Color
}

func NewStyle() Style {
	return Style{}
}

// Foreground sets the text foreground color. Returns a copy.
func (s Style) Foreground(c Color) Style {
	s.fgPrefix = colorToFgPrefix(c)
	s.fgColor = c

	return s
}

// Background sets the text background color. Returns a copy.
func (s Style) Background(c Color) Style {
	s.bgPrefix = colorToBgPrefix(c)
	s.bgColor = c

	return s
}

// Bold sets bold text rendering. Returns a copy.
func (s Style) Bold(v bool) Style {
	s.bold = v

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
func TruncateToWidth(str string, maxW int, ellipsis bool) string {
	return truncateToWidth(str, maxW, ellipsis)
}

// FgPrefix returns the pre-computed ANSI foreground prefix string.
func (s Style) FgPrefix() string { return s.fgPrefix }

// BgPrefix returns the pre-computed ANSI background prefix string.
func (s Style) BgPrefix() string { return s.bgPrefix }

// String returns the ANSI escape sequence for this style's color/bold
// attributes, without applying any layout.
func (s Style) String() string {
	var builder strings.Builder

	if s.fgPrefix != "" {
		builder.WriteString(s.fgPrefix)
	}

	if s.bgPrefix != "" {
		builder.WriteString(s.bgPrefix)
	}

	if s.bold {
		builder.WriteString("\x1b[1m")
	}

	if builder.Len() > 0 {
		builder.WriteString(ansiReset)
	}

	return builder.String()
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
func (s Style) Render(content ...string) string {
	combined := joinContent(content)

	if combined == "" && !s.hasBorder && s.padTop == 0 && s.padBottom == 0 && s.width == 0 {
		return ""
	}

	if !s.hasLayoutProperties() {
		return s.renderColorOnly(combined)
	}

	result := s.applyLayout(combined)

	return s.renderColorOnly(result)
}

// applyLayout applies padding, alignment, borders, and width constraints.
func (s Style) applyLayout(combined string) string {
	lines := splitLines(combined)

	if derived := s.deriveBorderWidth(lines); derived > 0 {
		s.width = derived
	}

	if s.maxWidth > 0 && s.width > s.maxWidth {
		s.width = s.maxWidth
	}

	s.applyHorizontalPadding(lines)
	lines = s.applyVerticalPadding(lines)

	borderWidth := s.borderCharWidth()
	innerWidth := max(s.width-borderWidth, 0)
	alignLines(lines, innerWidth, s.maxWidth, borderWidth, s.align, s.ellipsis)

	if s.hasBorder {
		lines = s.applyBorder(lines)
	}

	return strings.Join(lines, "\n")
}

// hasLayoutProperties reports whether any layout properties are set.
func (s Style) hasLayoutProperties() bool {
	return s.width > 0 || s.maxWidth > 0 || s.hasBorder ||
		s.padTop > 0 || s.padRight > 0 || s.padBottom > 0 || s.padLeft > 0
}

// joinContent concatenates content strings.
func joinContent(content []string) string {
	if len(content) == 1 {
		return content[0]
	}

	var builder strings.Builder
	for _, c := range content {
		builder.WriteString(c)
	}

	return builder.String()
}

// applyHorizontalPadding adds left/right padding to each line in-place.
func (s Style) applyHorizontalPadding(lines []string) {
	if s.padLeft <= 0 && s.padRight <= 0 {
		return
	}

	pad := strings.Repeat(" ", s.padLeft) + "%s" + strings.Repeat(" ", s.padRight)

	for i, line := range lines {
		lines[i] = strings.Replace(pad, "%s", line, 1)
	}
}

// applyVerticalPadding prepends top and appends bottom padding lines.
func (s Style) applyVerticalPadding(lines []string) []string {
	if s.padTop > 0 {
		padLine := s.padLine()

		padded := make([]string, 0, s.padTop+len(lines))
		for range s.padTop {
			padded = append(padded, padLine)
		}

		lines = append(padded, lines...)
	}

	if s.padBottom > 0 {
		padLine := s.padLine()

		bot := make([]string, s.padBottom)
		for i := range bot {
			bot[i] = padLine
		}

		lines = append(lines, bot...)
	}

	return lines
}

// padLine returns a blank line matching the style's width and horizontal padding.
func (s Style) padLine() string {
	if s.width == 0 {
		return strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.padRight)
	}

	return strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.width-s.padLeft-s.padRight) + strings.Repeat(" ", s.padRight)
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
func alignLines(lines []string, innerWidth, maxWidth, borderWidth int, align Position, ellipsis bool) {
	if innerWidth <= 0 && maxWidth <= 0 {
		return
	}

	maxWidthInner := computeMaxWidthInner(maxWidth, innerWidth, borderWidth)

	for lineIdx, line := range lines {
		targetWidth := resolveTargetWidth(innerWidth, maxWidthInner)
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

func alignLine(lines []string, lineIdx int, line string, targetWidth int, align Position, ellipsis bool) {
	lineWidth := CellWidth(line)

	if lineWidth < targetWidth {
		lines[lineIdx] = padLineAlignment(line, targetWidth-lineWidth, align)
	} else if lineWidth > targetWidth && targetWidth > 0 {
		lines[lineIdx] = truncateToWidth(line, targetWidth, ellipsis)
	}
}

// padLineAlignment pads a line to the given alignment.
func padLineAlignment(line string, pad int, align Position) string {
	switch {
	case align == Center:
		left := pad / 2 //nolint:mnd
		right := pad - left

		return strings.Repeat(" ", left) + line + strings.Repeat(" ", right)
	case align >= Right:
		return strings.Repeat(" ", pad) + line
	default:
		return line + strings.Repeat(" ", pad)
	}
}

// deriveBorderWidth returns the width derived from the widest content line when
// a border is present but no explicit width has been set. Returns 0 if the
// width should not be overridden.
func (s Style) deriveBorderWidth(lines []string) int {
	if !s.hasBorder || s.width != 0 {
		return 0
	}

	maxW := 0
	for _, line := range lines {
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
func (s Style) renderColorOnly(content string) string {
	if content == "" {
		return ""
	}

	if s.fgPrefix == "" && s.bgPrefix == "" && !s.bold {
		return content
	}

	prefix := s.stylePrefix()
	reset := ansiReset

	if !strings.Contains(content, "\n") {
		return prefix + content + reset
	}

	var builder strings.Builder

	for lineIdx, line := range strings.Split(content, "\n") {
		if lineIdx > 0 {
			builder.WriteByte('\n')
		}

		builder.WriteString(prefix)
		builder.WriteString(line)
		builder.WriteString(reset)
	}

	return builder.String()
}

// stylePrefix builds the ANSI escape sequence for fg/bg/bold.
func (s Style) stylePrefix() string {
	var builder strings.Builder

	if s.fgPrefix != "" {
		builder.WriteString(s.fgPrefix)
	}

	if s.bgPrefix != "" {
		builder.WriteString(s.bgPrefix)
	}

	if s.bold {
		builder.WriteString("\x1b[1m")
	}

	return builder.String()
}

// applyBorder wraps each line with vertical border runes and adds
// horizontal border lines at the top and/or bottom.
func (s Style) applyBorder(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	topFg, rightFg, bottomFg, leftFg, reset := s.borderColors()

	var result []string

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
func (s Style) borderColors() (string, string, string, string, string) {
	borderFg := s.borderFg
	if borderFg == "" {
		borderFg = s.fgPrefix
	}

	topFg := s.border.topFg
	if topFg == "" {
		topFg = borderFg
	}

	rightFg := s.border.rightFg
	if rightFg == "" {
		rightFg = borderFg
	}

	bottomFg := s.border.bottomFg
	if bottomFg == "" {
		bottomFg = borderFg
	}

	leftFg := s.border.leftFg
	if leftFg == "" {
		leftFg = borderFg
	}

	reset := ""
	if topFg != "" || rightFg != "" || bottomFg != "" || leftFg != "" {
		reset = ansiReset
	}

	return topFg, rightFg, bottomFg, leftFg, reset
}

// wrapBorderLine wraps a content line with left/right vertical borders.
func (s Style) wrapBorderLine(line, leftFg, rightFg, reset string) string {
	var lineBuilder strings.Builder

	if s.borderLeft {
		lineBuilder.WriteString(leftFg)
		lineBuilder.WriteString(s.border.Vertical)
		lineBuilder.WriteString(reset)
	}

	lineBuilder.WriteString(line)

	if s.borderRight {
		lineBuilder.WriteString(rightFg)
		lineBuilder.WriteString(s.border.Vertical)
		lineBuilder.WriteString(reset)
	}

	return lineBuilder.String()
}

// buildHorizontalBorder renders a top or bottom border line: corner + fill + corner.
// The fill width is s.width minus the left and right border rune cells.
func (s Style) buildHorizontalBorder(left, mid, right, borderFg, reset string) string {
	var lineBuilder strings.Builder

	lineBuilder.WriteString(borderFg)
	lineBuilder.WriteString(left)

	width := max(s.width, 0)

	if s.borderLeft {
		width--
	}

	if s.borderRight {
		width--
	}

	if width > 0 {
		lineBuilder.WriteString(strings.Repeat(mid, width))
	}

	lineBuilder.WriteString(right)
	lineBuilder.WriteString(reset)

	return lineBuilder.String()
}

// truncateToWidth truncates str to at most maxW visible cell width, preserving
// any ANSI escape sequences. When ellipsis is true and the string is actually
// truncated, ".." replaces the last 2 cells so overflowing content is visually
// distinguishable.
//
// Uses uniseg for grapheme cluster width (consistent with CellWidth) so emoji
// and other wide characters are measured correctly.
func truncateToWidth(str string, maxW int, ellipsis bool) string {
	if maxW <= 0 {
		return ""
	}

	pos, needsTruncation := scanTruncationPoint(str, maxW)

	if !needsTruncation {
		return str
	}

	// Content exceeds maxW — truncate.
	if ellipsis && maxW > 2 {
		// Reserve 2 cells for ".." and truncate content to maxW-2.
		return truncateToWidth(str, maxW-2, false) + ".." //nolint:mnd
	}

	return str[:pos]
}

// scanTruncationPoint scans str and returns the byte position where content
// would exceed maxW, plus whether truncation is needed.
func scanTruncationPoint(str string, maxW int) (int, bool) {
	width := 0
	pos := 0
	graphemeState := -1

	//nolint:gosec // G103: audited — zero-copy byte view of str, str outlives byteSlice
	byteSlice := unsafe.Slice(unsafe.StringData(str), len(str))

	for pos < len(byteSlice) {
		char := byteSlice[pos]

		if char == '\x1b' {
			graphemeState = -1
			pos = skipANSI(str, pos)

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
		_, rest, clusterWidth, newState := uniseg.FirstGraphemeCluster(byteSlice[pos:], graphemeState)
		graphemeState = newState

		if width+clusterWidth > maxW {
			return pos, true
		}

		width += clusterWidth
		pos = len(byteSlice) - len(rest)
	}

	return pos, false
}
