package zeroterm

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
	commonLen := min(len(newLines), len(oldLines))

	var diffs []ChangedLine

	for y := range commonLen {
		if newLines[y] == oldLines[y] {
			continue
		}

		diffs = append(diffs, ChangedLine{Y: y})
	}

	for y := commonLen; y < len(newLines); y++ {
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
		rowIdx := d.Y

		if rowIdx >= terminalHeight {
			break
		}

		// Always position explicitly — avoids cursor drift from wrapped
		// lines or \r characters in content.
		buf = append(buf, "\x1b["...)
		buf = appendInt(buf, rowIdx+1)
		buf = append(buf, ";1H"...)

		// Strip \r from line content. lipgloss and ANSI renderers may
		// emit \r within a "line" (e.g. for cursor repositioning within
		// a styled region). In the old cell-based pipeline, \r was
		// handled by resetting curX. In the line-based pipeline, \r
		// would cause the terminal to jump to column 0 mid-line,
		// overwriting the beginning of the content.
		line := lines[rowIdx]
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

//nolint:mnd
func appendInt(buf []byte, val int) []byte {
	if val < 0 {
		buf = append(buf, '-')
		val = -val
	}

	if val < 10 {
		//nolint:gosec // G115: safe — val<10, so '0'+val is in ['0','9']
		return append(buf, byte('0'+val))
	}

	if val < 100 {
		return append(buf, byte('0'+val/10), byte('0'+val%10))
	}

	if val < 1000 {
		buf = append(buf, byte('0'+val/100))
		buf = append(buf, byte('0'+(val/10)%10))

		return append(buf, byte('0'+val%10))
	}

	return strconv.AppendInt(buf, int64(val), 10)
}
