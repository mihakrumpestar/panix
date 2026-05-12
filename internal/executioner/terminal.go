package executioner

import (
	"bytes"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var (
	carriageReturn   = []byte("\r")
	eraseToEndOfLine = []byte("\x1b[K")
)

// terminalProcessor holds the cursor state for processing PTY output across
// segments and reads. It avoids threading (cursorOnNewLine, cursorAtColZero)
// as return values through a deep call chain.
type terminalProcessor struct {
	output          *buffer.LinesBufVer
	cursorOnNewLine bool
	cursorAtColZero bool
}

// process handles a single read's worth of PTY output, splitting on \r and
// applying terminal semantics:
//
//   - \r (carriage return): move cursor to column 0 — subsequent content
//     overwrites the current line
//   - \n (line feed): commit current line, start a new one
//   - \r\n: standard PTY line ending — commit current line, start new one
//   - \x1b[K: erase to end of line — stripped before processing
//
// A trailing \n means the cursor sits on a new empty line. Rather than
// storing a spurious empty line, we set PendingNewline so the next read's
// first content starts a fresh line instead of appending.
//
// A trailing \r means the cursor is at column 0 of the last line. The
// next read's first content should overwrite that line. If no more output
// comes, finalizeCommandLog removes the transient progress line.
func (tp *terminalProcessor) process(buf []byte, exm *command.CommandLog) {
	buf = bytes.ReplaceAll(buf, eraseToEndOfLine, nil)
	if len(buf) == 0 {
		return
	}

	endsWithNewline := buf[len(buf)-1] == '\n'
	tp.cursorOnNewLine = exm.PendingNewline
	tp.cursorAtColZero = exm.CarriageReturn
	exm.PendingNewline = false
	exm.CarriageReturn = false

	segments := bytes.Split(buf, carriageReturn)
	lastSeg := len(segments) - 1

	for segIdx, seg := range segments {
		isLast := segIdx == lastSeg
		tp.processSegment(seg, segIdx, isLast, endsWithNewline)
	}

	if lastSeg > 0 && len(segments[lastSeg]) == 0 {
		exm.CarriageReturn = true
	} else {
		exm.PendingNewline = endsWithNewline
	}

	tp.trimANSIOnlyTrailingLines()
}

func (tp *terminalProcessor) processSegment(seg []byte, segIdx int, isLast, endsWithNewline bool) {
	nlIdx := bytes.IndexByte(seg, '\n')
	if nlIdx < 0 {
		tp.processNoNewlineSegment(seg, segIdx)

		return
	}

	tp.processPreNewlineContent(seg[:nlIdx], segIdx, nlIdx)
	tp.processPostNewlines(seg[nlIdx+1:], isLast, endsWithNewline)
}

// processNoNewlineSegment handles a segment with no \n.
// segIdx==0: first segment (no preceding \r in this read).
// segIdx>0:  content after a \r — cursor at column 0, overwrite.
func (tp *terminalProcessor) processNoNewlineSegment(seg []byte, segIdx int) {
	if segIdx == 0 {
		tp.writeFirstSegmentContent(seg)

		return
	}

	if len(seg) > 0 {
		tp.output.OverrideLastLine(seg)
		tp.resetCursorFlags()
	}
}

// processPreNewlineContent handles content before the first \n in a segment.
// segIdx==0: first segment (no preceding \r).
// segIdx>0:  after a \r, cursor at column 0.
func (tp *terminalProcessor) processPreNewlineContent(preChunk []byte, segIdx, nlIdx int) {
	if segIdx == 0 {
		tp.writeFirstSegmentContent(preChunk)

		return
	}

	tp.writeOverridePreChunk(preChunk, nlIdx)
}

// writeFirstSegmentContent writes content that is NOT preceded by \r in this
// read. The cursor position from the previous read determines behavior:
//   - cursorAtColZero: previous read ended with \r → overwrite last line
//   - cursorOnNewLine: previous read ended with \n → start new line
//   - neither:         cursor mid-line → append to current line
func (tp *terminalProcessor) writeFirstSegmentContent(data []byte) {
	switch {
	case tp.cursorAtColZero && len(data) > 0:
		tp.output.OverrideLastLine(data)
		tp.resetCursorFlags()
	case tp.cursorOnNewLine && len(data) > 0:
		tp.output.Write(data)
		tp.resetCursorFlags()
	case !tp.cursorOnNewLine && !tp.cursorAtColZero:
		appendLineContent(tp.output, data)
	}
}

// writeOverridePreChunk handles pre-\n content after a \r.
// \r returns cursor to column 0, so this overwrites the current line.
func (tp *terminalProcessor) writeOverridePreChunk(preChunk []byte, nlIdx int) {
	switch {
	case nlIdx > 0:
		tp.output.OverrideLastLine(preChunk)
		tp.resetCursorFlags()
	case nlIdx == 0 && (tp.cursorOnNewLine || tp.cursorAtColZero):
		tp.output.Write([]byte{})
		tp.resetCursorFlags()
	}
}

// processPostNewlines processes data after the first \n in a segment,
// writing complete lines. The trailing newline artifact is handled by
// skipping an empty final chunk on the last segment when endsWithNewline.
func (tp *terminalProcessor) processPostNewlines(data []byte, isLast, endsWithNewline bool) {
	for {
		nlIdx := bytes.IndexByte(data, '\n')
		if nlIdx < 0 {
			if !isLast || len(data) != 0 || !endsWithNewline {
				tp.output.Write(data)
			}

			return
		}

		tp.output.Write(data[:nlIdx])
		data = data[nlIdx+1:]
	}
}

func (tp *terminalProcessor) resetCursorFlags() {
	tp.cursorOnNewLine = false
	tp.cursorAtColZero = false
}

// trimANSIOnlyTrailingLines removes trailing lines that contain only ANSI
// escape sequences with no visible content. Intentional empty lines
// (from \n\n) are preserved.
func (tp *terminalProcessor) trimANSIOnlyTrailingLines() {
	for tp.output.Len() > 0 {
		last := tp.output.LastLine()
		if len(last) == 0 {
			break
		}

		if len(style.StripANSI(last)) > 0 {
			break
		}

		tp.output.RemoveLastLine()
	}
}

// appendLineContent appends data to the last line in the output buffer.
// If the buffer is empty, a new line is created.
func appendLineContent(buf *buffer.LinesBufVer, data []byte) {
	if len(data) == 0 {
		return
	}

	if buf.Len() == 0 {
		buf.Write(data)

		return
	}

	last := buf.LastLine()
	combined := make([]byte, 0, len(last)+len(data))
	combined = append(combined, last...)
	combined = append(combined, data...)
	buf.OverrideLastLine(combined)
}

// finalizeCommandLog cleans up the command log after all PTY output has been
// read. When CarriageReturn is true, the cursor was at column 0 of the last
// line — meaning the line was a transient progress indicator (like nix copy's
// "[1/614/615 copied]...") that would have been overwritten by the next
// update. Since no more output is coming, remove it.
func finalizeCommandLog(commandLog *command.CommandLog) {
	if commandLog.CarriageReturn && commandLog.Output.Len() > 0 {
		commandLog.Output.RemoveLastLine()
	}

	commandLog.CarriageReturn = false
	commandLog.PendingNewline = false
}
