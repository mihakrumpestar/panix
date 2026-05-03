package style

import (
	"image/color"
	"strings"
)

const ansiReset = "\x1b[m"

// ANSIReset returns the ANSI reset escape sequence ("\x1b[m").
func ANSIReset() string { return ansiReset }

// ANSIStyle is a pre-computed ANSI escape sequence pair extracted from a
// Style. It replaces Style.Render in hot paths with simple string
// concatenation, avoiding the full rendering pipeline (border checks,
// margin calculations, grapheme clustering) for styles that only set
// foreground color or bold.
type ANSIStyle struct {
	prefix string
	reset  string
}

// NewANSIStyle extracts the ANSI prefix and reset sequences from a Style.
// Call this once at init time and reuse the result for all subsequent
// renders.
func NewANSIStyle(s Style) ANSIStyle {
	prefix := s.stylePrefix()

	return ANSIStyle{prefix: prefix, reset: ansiReset}
}

// Render wraps content in the pre-computed ANSI escape sequences.
// For single-line content this is a simple prefix+content+reset concatenation.
// For multi-line content, each line is individually wrapped so that the style
// persists across line breaks (matching lipgloss.Style.Render behavior for
// foreground-only styles).
func (a ANSIStyle) Render(content string) string {
	if a.prefix == "" || content == "" {
		return content
	}

	if !strings.Contains(content, "\n") {
		return a.prefix + content + a.reset
	}

	var builder strings.Builder

	first := true

	for {
		line := content

		idx := strings.IndexByte(content, '\n')
		if idx >= 0 {
			line = content[:idx]
			content = content[idx+1:]
		}

		if !first {
			builder.WriteByte('\n')
		}

		builder.WriteString(a.prefix)
		builder.WriteString(line)
		builder.WriteString(a.reset)

		first = false

		if idx < 0 {
			break
		}
	}

	return builder.String()
}

// Prefix returns the ANSI escape sequence prefix (e.g. "\x1b[38;2;241;250;140m").
func (a ANSIStyle) Prefix() string { return a.prefix }

// Reset returns the ANSI reset sequence ("\x1b[m").
func (a ANSIStyle) Reset() string { return a.reset }

// ColorToPrefix extracts the ANSI escape sequence prefix for the given
// foreground color (e.g. "\x1b[38;2;241;250;140m"). Returns "" if the
// color produces no prefix.
func ColorToPrefix(c color.Color) string {
	return colorToFgPrefix(c)
}

// ColorToBgPrefix extracts the ANSI escape sequence prefix for the given
// background color (e.g. "\x1b[48;2;51;51;51m"). Returns "" if the
// color produces no prefix.
func ColorToBgPrefix(c color.Color) string {
	return colorToBgPrefix(c)
}


