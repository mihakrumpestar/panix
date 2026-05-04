package render

import (
	"os"
	"strconv"
	"strings"
)

// ChangedLine represents a changed line index in a frame diff.
type ChangedLine struct {
	Y int
}

// DiffLines compares two line slices and returns indices of changed lines.
// Uses direct string comparison — O(n) per line but extremely fast in practice
// because Go's memequal short-circuits on the first differing byte, and
// most lines are identical between frames.
func DiffLines(newLines, oldLines []string) []ChangedLine {
	n := min(len(newLines), len(oldLines))

	var diffs []ChangedLine

	for y := range n {
		if newLines[y] == oldLines[y] {
			continue
		}

		diffs = append(diffs, ChangedLine{Y: y})
	}

	for y := n; y < len(newLines); y++ {
		diffs = append(diffs, ChangedLine{Y: y})
	}

	return diffs
}

// RenderLines emits terminal bytes for changed lines. Returns the output
// buffer (reuses the provided buf for zero allocation).
//
// For each changed line:
//
//	\x1b[y;1H  <line content with \r stripped>  \x1b[0m\x1b[K
//
// Always uses explicit cursor positioning to avoid misalignment from
// line-wrapping or cursor tracking drift. After all changed lines,
// clears below if the frame shrank.
func RenderLines(buf []byte, diffs []ChangedLine, lines []string, prevLineCount int, terminalHeight int) []byte {
	for _, d := range diffs {
		y := d.Y

		if y >= terminalHeight {
			break
		}

		// Always position explicitly — avoids cursor drift from wrapped
		// lines or \r characters in content.
		buf = append(buf, "\x1b["...)
		buf = appendInt(buf, y+1)
		buf = append(buf, ";1H"...)

		// Strip \r from line content. lipgloss and ANSI renderers may
		// emit \r within a "line" (e.g. for cursor repositioning within
		// a styled region). In the old cell-based pipeline, \r was
		// handled by resetting curX. In the line-based pipeline, \r
		// would cause the terminal to jump to column 0 mid-line,
		// overwriting the beginning of the content.
		line := lines[y]
		if strings.ContainsRune(line, '\r') {
			line = strings.ReplaceAll(line, "\r", "")
		}

		buf = append(buf, line...)
		buf = append(buf, "\x1b[0m\x1b[K"...)
	}

	// Clear below content if the frame shrank, or if the terminal is
	// taller than the content (stale lines from previous frames).
	contentEnd := min(len(lines), terminalHeight)
	if contentEnd < prevLineCount || contentEnd < terminalHeight {
		clearFrom := contentEnd
		if clearFrom < terminalHeight {
			buf = append(buf, "\x1b["...)
			buf = appendInt(buf, clearFrom+1)
			buf = append(buf, ";1H\x1b[J"...)
		}
	}

	return buf
}

// RenderLinesTo writes changed lines directly to the terminal.
func RenderLinesTo(out *os.File, diffs []ChangedLine, lines []string, prevLineCount int, terminalHeight int) {
	var buf []byte

	buf = RenderLines(buf[:0], diffs, lines, prevLineCount, terminalHeight)

	if len(buf) > 0 {
		_, _ = out.Write(buf)
	}
}

func appendInt(b []byte, n int) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}

	if n < 10 {
		return append(b, byte('0'+n))
	}

	if n < 100 {
		return append(b, byte('0'+n/10), byte('0'+n%10))
	}

	if n < 1000 {
		b = append(b, byte('0'+n/100))
		b = append(b, byte('0'+(n/10)%10))

		return append(b, byte('0'+n%10))
	}

	return strconv.AppendInt(b, int64(n), 10)
}
