package zeroterm

import (
	"bytes"
	"strconv"

	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
)

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
func RenderLines(buf []byte, diffs []int, cur *linesbuffer.LinesBuffer, prevLineCount int, terminalHeight int) []byte {
	lineCount := cur.Len()

	for _, lineIdx := range diffs {
		if lineIdx >= terminalHeight {
			break
		}

		if lineIdx < 0 || lineIdx >= lineCount {
			continue
		}

		buf = append(buf, "\x1b["...)
		buf = appendInt(buf, lineIdx+1)
		buf = append(buf, ";1H"...)

		// Strip \r from line content inline — avoids strings.ReplaceAll
		// allocation. lipgloss and ANSI renderers may emit \r within a
		// "line" (e.g. for cursor repositioning within a styled region).
		line := cur.Line(lineIdx)

		found := bytes.Contains(line, []byte{'\r'})
		if found {
			buf = appendStripCR(buf, line)
		} else {
			buf = append(buf, line...)
		}

		buf = append(buf, "\x1b[0m\x1b[K"...)
	}

	contentEnd := min(lineCount, terminalHeight)
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

// appendStripCR appends line to buf with all \r bytes removed.
func appendStripCR(buf []byte, line []byte) []byte {
	for _, b := range line {
		if b != '\r' {
			buf = append(buf, b)
		}
	}

	return buf
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
