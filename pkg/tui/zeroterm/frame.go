package zeroterm

import (
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
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

		// Strip \r and internal zone hit-test markers inline; avoids
		// intermediate allocations. See appendSanitizedLine.
		buf = appendSanitizedLine(buf, cur.Line(lineIdx))

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

// appendSanitizedLine appends line to buf after stripping \r bytes, internal
// zone hit-test markers, and every terminal control sequence except SGR.
//
// lipgloss and ANSI renderers may emit \r within a "line" (e.g. for
// cursor repositioning within a styled region). Zone markers are
// panix-internal state for mouse hit-testing (ZoneIDAtCol) and must
// never reach the physical terminal: terminal parsers disagree on
// unknown or malformed CSI sequences, and some print them as literal
// text, corrupting the layout. They are detected and skipped here (the
// shared sanitizer would treat them as non-SGR CSI) while every text run
// is passed through style.SanitizeTerminalOutput, the same filter used at
// the target-output boundary.
func appendSanitizedLine(buf []byte, line []byte) []byte {
	start := 0

	for pos := 0; pos < len(line); {
		byteI := line[pos]

		if byteI == '\r' {
			buf = style.SanitizeTerminalOutput(buf, line[start:pos])
			start = pos + 1
			pos++

			continue
		}

		if byteI == '\x1b' && pos+1 < len(line) && line[pos+1] == '[' {
			if end, _, _, ok := parseZoneBody(line, pos+2); ok {
				buf = style.SanitizeTerminalOutput(buf, line[start:pos])
				start = end
				pos = end

				continue
			}
		}

		pos++
	}

	return style.SanitizeTerminalOutput(buf, line[start:])
}
