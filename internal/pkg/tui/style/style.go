package style

import (
	"strings"
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
	case 4:
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

// FgPrefix returns the pre-computed ANSI foreground prefix string.
func (s Style) FgPrefix() string { return s.fgPrefix }

// BgPrefix returns the pre-computed ANSI background prefix string.
func (s Style) BgPrefix() string { return s.bgPrefix }

// String returns the ANSI escape sequence for this style's color/bold
// attributes, without applying any layout.
func (s Style) String() string {
	var b strings.Builder

	if s.fgPrefix != "" {
		b.WriteString(s.fgPrefix)
	}

	if s.bgPrefix != "" {
		b.WriteString(s.bgPrefix)
	}

	if s.bold {
		b.WriteString("\x1b[1m")
	}

	if b.Len() > 0 {
		b.WriteString(ansiReset)
	}

	return b.String()
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
//
//nolint:cyclop,funlen
func (s Style) Render(content ...string) string {
	var combined string
	if len(content) == 1 {
		combined = content[0]
	} else {
		var b strings.Builder
		for _, c := range content {
			b.WriteString(c)
		}

		combined = b.String()
	}

	if combined == "" && !s.hasBorder && s.padTop == 0 && s.padBottom == 0 && s.width == 0 {
		return ""
	}

	hasLayout := s.width > 0 || s.maxWidth > 0 || s.hasBorder ||
		s.padTop > 0 || s.padRight > 0 || s.padBottom > 0 || s.padLeft > 0

	if !hasLayout {
		return s.renderColorOnly(combined)
	}

	lines := splitLines(combined)

	// When a border is set without an explicit width, derive the width from
	// the widest content line so the border frames the content correctly.
	if s.hasBorder && s.width == 0 {
		maxW := 0
		for _, line := range lines {
			if w := CellWidth(line); w > maxW {
				maxW = w
			}
		}

		if maxW > 0 {
			bw := 0
			if s.borderLeft {
				bw++
			}

			if s.borderRight {
				bw++
			}

			s.width = maxW + s.padLeft + s.padRight + bw
		}
	}

	// When MaxWidth is set and smaller than Width, cap the effective width.
	// This ensures borders, padding, and content all honor the limit,
	// producing a properly framed block at maxWidth rather than a truncated
	// mess with missing right borders (which lipgloss v2 does by truncating
	// the final output).
	if s.maxWidth > 0 && s.width > s.maxWidth {
		s.width = s.maxWidth
	}

	if s.padLeft > 0 || s.padRight > 0 {
		pad := strings.Repeat(" ", s.padLeft) + "%s" + strings.Repeat(" ", s.padRight)

		for i, line := range lines {
			lines[i] = strings.Replace(pad, "%s", line, 1)
		}
	}

	if s.padTop > 0 {
		padLine := strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.width-s.padLeft-s.padRight) + strings.Repeat(" ", s.padRight)
		if s.width == 0 {
			padLine = strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.padRight)
		}

		top := make([]string, s.padTop)
		for i := range top {
			top[i] = padLine
		}

		lines = append(top, lines...)
	}

	if s.padBottom > 0 {
		padLine := strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.width-s.padLeft-s.padRight) + strings.Repeat(" ", s.padRight)
		if s.width == 0 {
			padLine = strings.Repeat(" ", s.padLeft) + strings.Repeat(" ", s.padRight)
		}

		bot := make([]string, s.padBottom)
		for i := range bot {
			bot[i] = padLine
		}

		lines = append(lines, bot...)
	}

	// contentWidth is the text area inside borders and padding.
	contentWidth := s.width
	borderWidth := 0

	if s.hasBorder {
		if s.borderLeft {
			borderWidth++
		}

		if s.borderRight {
			borderWidth++
		}

		contentWidth -= borderWidth
	}

	contentWidth -= s.padLeft + s.padRight
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Pad shorter lines to innerWidth, truncate longer ones.
	// innerWidth is the space inside borders (content + padding).
	// Only apply when the style has a fixed width or a maxWidth constraint.
	innerWidth := max(s.width-borderWidth, 0)

	if innerWidth > 0 || s.maxWidth > 0 {
		// maxWidthInner is used when Width is 0 but MaxWidth is set:
		// it defines the inner-space limit without a fixed block width.
		maxWidthInner := 0
		if s.maxWidth > 0 && s.width == 0 {
			maxWidthInner = max(s.maxWidth-borderWidth, 0)
		}

		for i, line := range lines {
			w := CellWidth(line)
			targetWidth := innerWidth

			if targetWidth == 0 && maxWidthInner > 0 {
				targetWidth = maxWidthInner
			}

			if w < targetWidth {
				pad := targetWidth - w

				switch {
				case s.align == Center:
					left := pad / 2
					right := pad - left
					lines[i] = strings.Repeat(" ", left) + line + strings.Repeat(" ", right)
				case s.align >= Right:
					lines[i] = strings.Repeat(" ", pad) + line
				default:
					lines[i] = line + strings.Repeat(" ", pad)
				}
			} else if w > targetWidth && targetWidth > 0 {
				lines[i] = truncateToWidth(line, targetWidth, s.ellipsis)
			}
		}
	}

	if s.hasBorder {
		lines = s.applyBorder(lines)
	}

	result := strings.Join(lines, "\n")

	return s.renderColorOnly(result)
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

	var b strings.Builder

	for i, line := range strings.Split(content, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}

		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString(reset)
	}

	return b.String()
}

// stylePrefix builds the ANSI escape sequence for fg/bg/bold.
func (s Style) stylePrefix() string {
	var b strings.Builder

	if s.fgPrefix != "" {
		b.WriteString(s.fgPrefix)
	}

	if s.bgPrefix != "" {
		b.WriteString(s.bgPrefix)
	}

	if s.bold {
		b.WriteString("\x1b[1m")
	}

	return b.String()
}

// applyBorder wraps each line with vertical border runes and adds
// horizontal border lines at the top and/or bottom.
func (s Style) applyBorder(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	b := s.border

	fg := s.borderFg
	if fg == "" {
		fg = s.fgPrefix
	}

	topFg := b.topFg
	if topFg == "" {
		topFg = fg
	}

	rightFg := b.rightFg
	if rightFg == "" {
		rightFg = fg
	}

	bottomFg := b.bottomFg
	if bottomFg == "" {
		bottomFg = fg
	}

	leftFg := b.leftFg
	if leftFg == "" {
		leftFg = fg
	}

	reset := ""
	if topFg != "" || rightFg != "" || bottomFg != "" || leftFg != "" {
		reset = ansiReset
	}

	var result []string

	if s.borderTop {
		topBorder := s.buildHorizontalBorder(b.TopLeft, b.Horizontal, b.TopRight, topFg, reset)
		result = append(result, topBorder)
	}

	for _, line := range lines {
		var sb strings.Builder

		if s.borderLeft {
			sb.WriteString(leftFg)
			sb.WriteString(b.Vertical)
			sb.WriteString(reset)
		}

		sb.WriteString(line)

		if s.borderRight {
			sb.WriteString(rightFg)
			sb.WriteString(b.Vertical)
			sb.WriteString(reset)
		}

		result = append(result, sb.String())
	}

	if s.borderBottom {
		bottomBorder := s.buildHorizontalBorder(b.BottomLeft, b.Horizontal, b.BottomRight, bottomFg, reset)
		result = append(result, bottomBorder)
	}

	return result
}

// buildHorizontalBorder renders a top or bottom border line: corner + fill + corner.
// The fill width is s.width minus the left and right border rune cells.
func (s Style) buildHorizontalBorder(left, mid, right, fg, reset string) string {
	var sb strings.Builder

	sb.WriteString(fg)
	sb.WriteString(left)

	width := max(s.width, 0)

	if s.borderLeft {
		width--
	}

	if s.borderRight {
		width--
	}

	if width > 0 {
		sb.WriteString(strings.Repeat(mid, width))
	}

	sb.WriteString(right)
	sb.WriteString(reset)

	return sb.String()
}

// truncateToWidth truncates str to at most maxW visible cell width, preserving
// any ANSI escape sequences. When ellipsis is true and the string is actually
// truncated, ".." replaces the last 2 cells so overflowing content is visually
// distinguishable.
func truncateToWidth(str string, maxW int, ellipsis bool) string {
	if maxW <= 0 {
		return ""
	}

	// First pass: determine if truncation is needed and find the cut position
	// at maxW cells.
	width := 0
	pos := 0
	needsTruncation := false

	for pos < len(str) {
		ch := str[pos]

		if ch == '\x1b' {
			pos = skipANSI(str, pos)

			continue
		}

		if ch >= 0x20 && ch < 0x7F {
			if width+1 > maxW {
				needsTruncation = true

				break
			}

			width++
			pos++

			continue
		}

		if ch < 0x80 {
			pos++

			continue
		}

		_, size := decodeUTF8(str, pos)
		rw := RuneWidth(rune(str[pos]))

		if width+rw > maxW {
			needsTruncation = true

			break
		}

		width += rw
		pos += size
	}

	if !needsTruncation {
		return str
	}

	// Content exceeds maxW — truncate.
	if ellipsis && maxW > 2 {
		// Reserve 2 cells for ".." and truncate content to maxW-2.
		return truncateToWidth(str, maxW-2, false) + ".."
	}

	return str[:pos]
}
