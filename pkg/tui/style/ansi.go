package style

import (
	"strconv"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

var (
	ansiReset      = []byte("\x1b[m")
	ansiBold       = []byte("\x1b[1m")
	ansiForeground = []byte("\x1b[38;2;")
	ansiBackground = []byte("\x1b[48;2;")
)

// ANSIReset returns the ANSI reset escape sequence ("\x1b[m").
func ANSIReset() []byte {
	return ansiReset
}

// ANSIStyle is a pre-computed ANSI escape sequence pair extracted from a
// Style. It replaces Style.Render in hot paths with simple byte
// concatenation, avoiding the full rendering pipeline (border checks,
// margin calculations, grapheme clustering) for styles that only set
// foreground color or bold.
type ANSIStyle struct {
	prefix []byte
	reset  []byte
}

// NewANSIStyle extracts the ANSI prefix and reset sequences from a Style.
// Call this once at init time and reuse the result for all subsequent
// renders.
func NewANSIStyle(s Style) ANSIStyle {
	prefix := s.stylePrefix()

	return ANSIStyle{prefix: prefix, reset: ansiReset}
}

// Render writes the styled content into a pooled LinesBuf.
// Each line is individually wrapped with the ANSI prefix+reset so that
// the style persists across line breaks.
func (a ANSIStyle) Render(buf *buffer.LinesBuf, content [][]byte) {
	buf.Reset()

	if a.prefix == nil || content == nil {
		for range content {
			buf.EmptyLine()
		}

		return
	}

	for _, line := range content {
		buf.WriteLine(a.prefix, line, a.reset)
	}
}

// Prefix returns the ANSI escape sequence prefix (e.g. "\x1b[38;2;241;250;140m").
func (a ANSIStyle) Prefix() []byte {
	return a.prefix
}

// Reset returns the ANSI reset sequence ("\x1b[m").
func (a ANSIStyle) Reset() []byte {
	return a.reset
}

// ColorToPrefix extracts the ANSI escape sequence prefix for the given
// foreground color (e.g. "\x1b[38;2;241;250;140m"). Returns nil if the
// color produces no prefix.
func ColorToPrefix(c Color) []byte {
	return colorToFgPrefix(c)
}

// ColorToBgPrefix extracts the ANSI escape sequence prefix for the given
// background color (e.g. "\x1b[48;2;51;51;51m"). Returns nil if the color
// produces no prefix.
func ColorToBgPrefix(c Color) []byte {
	return colorToBgPrefix(c)
}

// RGB8ToBgPrefix builds the ANSI background escape sequence directly from
// 8-bit RGB components (e.g. "\x1b[48;2;51;51;51m"). Zero-allocation
// using the caller's buffer. Appends to buf[:0] and returns the result.
func RGB8ToBgPrefix(buf []byte, r, g, b uint8) []byte {
	buf = append(buf[:0], ansiBackground...)
	buf = strconv.AppendInt(buf, int64(r), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(g), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(b), 10)
	buf = append(buf, 'm')

	return buf
}
