package style

import (
	"image/color"
	"strings"
)

type Style struct {
	fgPrefix  string
	bgPrefix  string
	bold      bool
	width     int
	maxWidth  int
	align     Position
	padTop    int
	padRight  int
	padBottom int
	padLeft   int

	border        Border
	borderFg      string
	hasBorder     bool
	borderTop     bool
	borderRight   bool
	borderBottom  bool
	borderLeft    bool

	fgColor color.Color
	bgColor color.Color
}

func NewStyle() Style {
	return Style{}
}

func (s Style) Foreground(c color.Color) Style {
	s.fgPrefix = colorToFgPrefix(c)
	s.fgColor = c

	return s
}

func (s Style) Background(c color.Color) Style {
	s.bgPrefix = colorToBgPrefix(c)
	s.bgColor = c

	return s
}

func (s Style) Bold(v bool) Style {
	s.bold = v

	return s
}

func (s Style) Width(w int) Style {
	s.width = w

	return s
}

func (s Style) MaxWidth(w int) Style {
	s.maxWidth = w

	return s
}

func (s Style) Align(p Position) Style {
	s.align = p

	return s
}

func (s Style) Padding(vertical, horizontal int) Style {
	s.padTop = vertical
	s.padBottom = vertical
	s.padLeft = horizontal
	s.padRight = horizontal

	return s
}

func (s Style) PaddingTop(v int) Style   { s.padTop = v; return s }
func (s Style) PaddingRight(v int) Style  { s.padRight = v; return s }
func (s Style) PaddingBottom(v int) Style { s.padBottom = v; return s }
func (s Style) PaddingLeft(v int) Style   { s.padLeft = v; return s }

//nolint:cyclop
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

func (s Style) BorderForeground(c color.Color) Style {
	s.borderFg = colorToFgPrefix(c)

	return s
}

func (s Style) BorderTopForeground(c color.Color) Style {
	s.border.topFg = colorToFgPrefix(c)

	return s
}

func (s Style) BorderRightForeground(c color.Color) Style {
	s.border.rightFg = colorToFgPrefix(c)

	return s
}

func (s Style) BorderBottomForeground(c color.Color) Style {
	s.border.bottomFg = colorToFgPrefix(c)

	return s
}

func (s Style) BorderLeftForeground(c color.Color) Style {
	s.border.leftFg = colorToFgPrefix(c)

	return s
}

func (s Style) GetForeground() color.Color { return s.fgColor }
func (s Style) GetBackground() color.Color { return s.bgColor }

// GetWidth returns the explicit width set on the style, or 0 if none.
func (s Style) GetWidth() int { return s.width }

// FgPrefix returns the pre-computed ANSI foreground prefix string.
// Used by packages that need direct access to the style's color encoding.
func (s Style) FgPrefix() string { return s.fgPrefix }

// BgPrefix returns the pre-computed ANSI background prefix string.
// Used by packages that need direct access to the style's color encoding.
func (s Style) BgPrefix() string { return s.bgPrefix }

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

	if contentWidth > 0 || s.maxWidth > 0 {
		for i, line := range lines {
			w := CellWidth(line)
			targetWidth := contentWidth

			if targetWidth == 0 && s.maxWidth > 0 {
				targetWidth = s.maxWidth
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
			} else if s.maxWidth > 0 && w > s.maxWidth {
				lines[i] = truncateToWidth(line, s.maxWidth)
			}
		}
	}

	if s.hasBorder {
		lines = s.applyBorder(lines)
	}

	result := strings.Join(lines, "\n")

	return s.renderColorOnly(result)
}

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

func (s Style) buildHorizontalBorder(left, mid, right, fg, reset string) string {
	var sb strings.Builder

	sb.WriteString(fg)
	sb.WriteString(left)

	width := s.width
	if width <= 0 {
		width = 0
	}

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

func truncateToWidth(str string, maxW int) string {
	width := 0
	pos := 0

	for pos < len(str) {
		ch := str[pos]

		if ch == '\x1b' {
			pos = skipANSI(str, pos)

			continue
		}

		if ch >= 0x20 && ch < 0x7F {
			if width+1 > maxW {
				return str[:pos]
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
			return str[:pos]
		}

		width += rw
		pos += size
	}

	return str
}
