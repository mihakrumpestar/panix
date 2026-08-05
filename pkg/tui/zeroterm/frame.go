package zeroterm

import (
	"bytes"

	"github.com/mihakrumpestar/panix/pkg/buffer"
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
func RenderLines(buf []byte, diffs []int, cur *buffer.LinesBufDiff, prevLineCount int, terminalHeight int) []byte {
	lineCount := cur.Len()

	for _, lineIdx := range diffs {
		if lineIdx >= terminalHeight {
			break
		}

		if lineIdx < 0 || lineIdx >= lineCount {
			continue
		}

	buf = append(buf, "\x1b["...)
	buf = buffer.AppendInt(buf, lineIdx+1)
	buf = append(buf, ";1H"...)

	// Strip \r from line content inline; avoids strings.ReplaceAll
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
			buf = buffer.AppendInt(buf, clearFrom+1)
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
