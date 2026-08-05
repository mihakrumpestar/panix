package executioner

import (
	"bytes"
	"slices"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

var (
	carriageReturn = []byte("\r")
	tabToSpaces    = []byte("    ")
	// nonPrintingSequences are ANSI escape sequences that control terminal
	// behavior (erase, cursor visibility) but produce no visible output.
	// They are stripped before processing so they don't pollute output lines.
	// SGR sequences (\x1b[...m) are NOT stripped - they carry color/style
	// information that is preserved for display.
	nonPrintingSequences = [][]byte{
		[]byte("\x1b[K"),    // erase to end of line
		[]byte("\x1b[?25h"), // show cursor (DEC private mode)
		[]byte("\x1b[?25l"), // hide cursor (DEC private mode)
	}
)

const escByte = '\x1b'

// terminalProcessor holds the cursor state for processing PTY output across
// segments and reads. It avoids threading (cursorOnNewLine, cursorAtColZero)
// as return values through a deep call chain.
type terminalProcessor struct {
	output          *buffer.LinesBufVer
	cursorOnNewLine bool
	cursorAtColZero bool
	pending         []byte
}

// process handles a single read's worth of PTY output, splitting on \r and
// applying terminal semantics:
//
//   - \r (carriage return): move cursor to column 0 - subsequent content
//     overwrites the current line
//   - \n (line feed): commit current line, start a new one
//   - \r\n: standard PTY line ending - commit current line, start new one
//   - \x1b[K: erase to end of line - stripped before processing
//   - \x1b[?25h/\x1b[?25l: show/hide cursor - stripped before processing
//
// SGR sequences (\x1b[...m) are preserved - they carry color/style info.
//
// A trailing \n means the cursor sits on a new empty line. Rather than
// storing a spurious empty line, we set PendingNewline so the next read's
// first content starts a fresh line instead of appending.
//
// A trailing \r (or \r followed by ANSI-only content with no visible chars
// and no \n) means the cursor is at column 0 of the last line. The next
// read's first content should overwrite that line. If no more output
// comes, finalizeCommandLog removes the transient progress line.
func (tp *terminalProcessor) process(buf []byte, exm *command.CommandLog) {
	buf = tp.prependPending(buf)
	buf = tp.stripPendingANSI(buf)
	buf = stripNonPrinting(buf)
	buf = expandTabs(buf)

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
		tp.processSegment(applyBackspaces(seg), segIdx, isLast, endsWithNewline)
	}

	tp.setTrailingCursorState(exm, segments, lastSeg, endsWithNewline)
	tp.trimANSIOnlyTrailingLines()
}

// prependPending combines any partial trailing bytes from the previous read
// with the current read's buffer.
func (tp *terminalProcessor) prependPending(buf []byte) []byte {
	if len(tp.pending) == 0 {
		return buf
	}

	combined := make([]byte, 0, len(tp.pending)+len(buf))
	combined = append(combined, tp.pending...)
	combined = append(combined, buf...)
	tp.pending = tp.pending[:0]

	return combined
}

// stripPendingANSI detects and saves any incomplete ANSI escape sequence at
// the end of buf (split by a PTY read boundary) so it can be prepended to the
// next read. Returns buf with the incomplete suffix removed.
func (tp *terminalProcessor) stripPendingANSI(buf []byte) []byte {
	tp.pending = trailingIncompleteANSI(buf)
	if len(tp.pending) > 0 {
		return buf[:len(buf)-len(tp.pending)]
	}

	return buf
}

// stripNonPrinting removes ANSI escape sequences that control terminal
// behavior (erase, cursor visibility) but produce no visible output.
// SGR color sequences (\x1b[...m) are preserved for display.
func stripNonPrinting(buf []byte) []byte {
	if bytes.IndexByte(buf, escByte) < 0 {
		return buf
	}

	for _, seq := range nonPrintingSequences {
		buf = bytes.ReplaceAll(buf, seq, nil)
	}

	return buf
}

// expandTabs replaces \t with 4 spaces.
func expandTabs(buf []byte) []byte {
	if bytes.IndexByte(buf, '\t') < 0 {
		return buf
	}

	return bytes.ReplaceAll(buf, []byte("\t"), tabToSpaces)
}

// setTrailingCursorState sets CarriageReturn or PendingNewline on the command
// log based on the last segment after \r splitting.
//
// CarriageReturn is set when the buffer ends with \r (or \r followed by
// ANSI-only content with no \n), meaning the cursor sits at column 0 of a
// transient progress line. The next read's first content should overwrite
// that line; if no more output comes, finalizeCommandLog removes it.
//
// We check !HasVisibleContent instead of len==0 to also catch ANSI-only
// trailing segments like \x1b[?25h (DEC private "show cursor"), which nix
// emits between the last \r and the final output. The \n exclusion ensures
// \r\n (cursor on a new line, not column 0) sets PendingNewline instead.
func (tp *terminalProcessor) setTrailingCursorState(
	exm *command.CommandLog,
	segments [][]byte,
	lastSeg int,
	endsWithNewline bool,
) {
	lastSegData := segments[lastSeg]
	if lastSeg > 0 && bytes.IndexByte(lastSegData, '\n') < 0 && !style.HasVisibleContent(lastSegData) {
		exm.CarriageReturn = true
	} else {
		exm.PendingNewline = endsWithNewline
	}
}

// trailingIncompleteANSI detects whether buf ends with an incomplete ANSI
// escape sequence that was split by a PTY read boundary. If so, returns the
// partial bytes to be prepended to the next read. This prevents partial
// escape sequences from being processed as visible output.
//
// Handles CSI (\x1b[...<final>), OSC (\x1b]...<BEL/ST>), and bare ESC
// (\x1b<0x40-0x5F>) sequences. SGR color sequences (\x1b[...m) are also
// caught here when split across reads.
func trailingIncompleteANSI(buf []byte) []byte {
	escIdx := bytes.LastIndexByte(buf, escByte)
	if escIdx < 0 {
		return nil
	}

	end := style.SkipANSI(buf, escIdx)
	if end < len(buf) {
		return nil // complete sequence with content after it
	}

	// SkipANSI consumed to end of buffer. Determine if the sequence is
	// actually complete or just ran off the end.
	if escIdx == len(buf)-1 {
		return buf[escIdx:] // lone ESC, incomplete
	}

	return trailingIncompleteByType(buf, escIdx)
}

// trailingIncompleteByType checks whether the ANSI escape starting at escIdx
// is complete based on its type (CSI, OSC, or bare ESC). Returns the partial
// bytes if incomplete, nil if complete.
func trailingIncompleteByType(buf []byte, escIdx int) []byte {
	switch buf[escIdx+1] {
	case '[':
		return trailingIncompleteCSI(buf, escIdx)
	case ']':
		return trailingIncompleteOSC(buf, escIdx)
	default:
		// Bare ESC (2-byte C1 code like \x1bM): always complete with 2 bytes
		return nil
	}
}

// trailingIncompleteCSI checks if a CSI sequence (\x1b[...<final>) starting at
// escIdx is complete. A CSI is complete iff SkipANSI found a final byte
// (0x40-0x7E) that is not the '[' introducer itself.
func trailingIncompleteCSI(buf []byte, escIdx int) []byte {
	end := style.SkipANSI(buf, escIdx)
	if end-1 > escIdx+1 && buf[end-1] >= 0x40 && buf[end-1] <= 0x7E {
		return nil // complete
	}

	return buf[escIdx:] // incomplete
}

// trailingIncompleteOSC checks if an OSC sequence (\x1b]...<BEL/ST>) starting
// at escIdx is complete. An OSC is complete iff it ends with BEL (0x07) or
// ST (\x1b\\).
func trailingIncompleteOSC(buf []byte, escIdx int) []byte {
	last := buf[len(buf)-1]
	if last == 0x07 { //nolint:mnd
		return nil // complete (BEL terminator)
	}

	if len(buf) >= 2 && buf[len(buf)-2] == escByte && last == '\\' {
		return nil // complete (ST terminator)
	}

	return buf[escIdx:] // incomplete
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
// segIdx>0:  content after a \r, cursor at column 0, overwrite.
func (tp *terminalProcessor) processNoNewlineSegment(seg []byte, segIdx int) {
	if segIdx == 0 {
		tp.writeFirstSegmentContent(seg)

		return
	}

	if len(seg) == 0 {
		return
	}

	if !style.HasVisibleContent(seg) {
		tp.resetCursorFlags()

		return
	}

	tp.output.OverrideLastLine(seg)
	tp.resetCursorFlags()
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
	case tp.cursorAtColZero:
		if len(data) > 0 {
			tp.output.OverrideLastLine(data)
		}

		tp.resetCursorFlags()
	case tp.cursorOnNewLine:
		if len(data) == 0 {
			tp.output.Write(nil)
		} else {
			tp.output.Write(data)
		}

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
		tp.output.Write(nil)
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

		if style.HasVisibleContent(last) {
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
// line, meaning the line was a transient progress indicator (like nix copy's
// "[1/614/615 copied]...") that would have been overwritten by the next
// update. Since no more output is coming, remove it.
func finalizeCommandLog(commandLog *command.CommandLog) {
	if commandLog.CarriageReturn && commandLog.Output.Len() > 0 {
		commandLog.Output.RemoveLastLine()
	}

	commandLog.CarriageReturn = false
	commandLog.PendingNewline = false
}

// applyBackspaces simulates terminal backspace (\b) processing on a single
// line segment. For each \b it removes the preceding visible character.
// Returns data unchanged (zero-copy) when no \b is present.
func applyBackspaces(data []byte) []byte {
	idx := bytes.IndexByte(data, '\b')
	if idx < 0 {
		return data
	}

	out := make([]byte, 0, len(data))
	out = append(out, data[:idx]...)
	lastVisible := findLastVisible(out)

	for pos := idx; pos < len(data); pos++ {
		char := data[pos]

		if char == '\b' {
			if lastVisible >= 0 {
				out = out[:lastVisible]
				lastVisible = findLastVisible(out)
			}

			continue
		}

		if char == escByte {
			end := style.SkipANSI(data, pos)
			out = append(out, data[pos:end]...)
			pos = end - 1

			continue
		}

		out = append(out, char)

		if char == '\n' || char == '\r' || byteWidth(char) > 0 {
			lastVisible = len(out) - 1
		}
	}

	return out
}

// findLastVisible returns the byte index of the last visible character in
// data, or -1 if none is found.
func findLastVisible(data []byte) int {
	for i, v := range slices.Backward(data) {
		if byteWidth(v) > 0 || v == '\n' || v == '\r' {
			return i
		}
	}

	return -1
}

// byteWidth returns the terminal cell width of a single byte.
func byteWidth(b byte) int {
	switch {
	case b >= 0x20 && b < 0x7F:
		return 1
	case b == '\t':
		return 4 //nolint:mnd
	default:
		return 0
	}
}
