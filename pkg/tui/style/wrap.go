package style

import (
	"sync"
	"unicode/utf8"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

// Wrap wraps lines of byte content to the given cell width, preserving ANSI
// escape sequences and inserting newlines at word boundaries. Output is
// written into buf.
//
// ANSI style carry-over: when a wrap-induced line break occurs, any active
// ANSI SGR sequences (colors, bold, etc.) are re-emitted at the start of the
// next wrapped line, and a reset is emitted before the break. This preserves
// styled text across line wraps.
//
// Each element of content is treated as a line.
//
// breakpoints is a string of characters considered valid word-break points
// (in addition to whitespace and hyphens). Pass "" for default behavior.
func Wrap(dst *buffer.LinesBuf, content [][]byte, limit int, breakpoints string) {
	dst.Reset()

	if limit < 1 || len(content) == 0 {
		for range content {
			dst.EmptyLine()
		}

		return
	}

	if len(content) == 1 && len(content[0]) <= limit {
		dst.WriteLine(content[0])

		return
	}

	hasBreakpoints := breakpoints != ""

	wrappedState, ok := wrapStatePool.Get().(*wrapState)
	if !ok {
		wrappedState = &wrapState{
			lineBuf:   make([]byte, 0, 128), //nolint:mnd
			carryBuf:  make([]byte, 0, 32),  //nolint:mnd
			lineStyle: make([]byte, 0, 64),  //nolint:mnd
		}
	}

	wrappedState.limit = limit
	wrappedState.breakpoints = breakpoints
	wrappedState.outBuf = dst
	wrappedState.curWidth = 0
	wrappedState.hasWord = false
	wrappedState.hasSpace = false
	wrappedState.wordWidth = 0
	wrappedState.spaceWidth = 0
	wrappedState.lineBuf = wrappedState.lineBuf[:0]
	wrappedState.lineStyle = wrappedState.lineStyle[:0]

	for _, line := range content {
		if len(line) == 0 {
			dst.EmptyLine()

			continue
		}

		if !lineExceedsLimitBytes(line, limit) {
			dst.WriteLine(line)

			continue
		}

		wrappedState.data = line
		wrappedState.wrapOneLine(line, hasBreakpoints)
	}

	wrappedState.data = nil
	wrappedState.outBuf = nil
	wrapStatePool.Put(wrappedState)
}

var wrapStatePool = sync.Pool{
	New: func() any {
		return &wrapState{
			lineBuf:   make([]byte, 0, 128), //nolint:mnd
			carryBuf:  make([]byte, 0, 32),  //nolint:mnd
			lineStyle: make([]byte, 0, 64),  //nolint:mnd
		}
	},
}

type wrapState struct {
	data        []byte
	limit       int
	breakpoints string
	curWidth    int

	wordStart int
	wordEnd   int
	wordWidth int
	hasWord   bool

	spaceStart int
	spaceEnd   int
	spaceWidth int
	hasSpace   bool

	lineBuf []byte
	outBuf  *buffer.LinesBuf

	// lineStyle is the net active ANSI style state at the END of lineBuf.
	// Updated only when content is flushed into lineBuf (not when scanned).
	// Empty means no active style (either plain text or reset was the last SGR).
	lineStyle []byte

	// carryBuf is a scratch buffer for building the style prefix to
	// carry over to the next wrapped line.
	carryBuf []byte
}

//nolint:cyclop,funlen
func (s *wrapState) wrapOneLine(data []byte, hasBreakpoints bool) {
	s.curWidth = 0
	s.hasWord = false
	s.hasSpace = false
	s.wordWidth = 0
	s.spaceWidth = 0
	s.lineBuf = s.lineBuf[:0]
	s.lineStyle = s.lineStyle[:0]

	pos := 0
	n := len(data)

	for pos < n {
		byteI := data[pos]

		if byteI == '\x1b' {
			end := skipANSI(data, pos)

			if !s.hasWord {
				s.wordStart = pos
				s.hasWord = true
			}

			s.wordEnd = end
			pos = end

			continue
		}

		// ASCII fast path: avoid utf8.DecodeRuneInString and runeWidth for
		// single-byte characters (width always 1, inline space/break checks).
		if byteI < 0x80 { //nolint:mnd
			pos++

			switch {
			case byteI == '\n':
				s.handleNewline()
			case byteI == '\t':
				s.handleTab(pos-1, pos)
			case byteI == ' ' || byteI == '\v' || byteI == '\f' || byteI == '\r':
				s.handleSpace(pos-1, pos, 1)
			case byteI == '-':
				s.handleBreakpoint(pos-1, pos, 1)
			case hasBreakpoints && s.isBreakpointRune(rune(byteI)):
				s.handleBreakpoint(pos-1, pos, 1)
			default:
				s.handleWordRune(pos-1, pos, 1)
			}

			continue
		}

		runeI, size := utf8.DecodeRune(data[pos:])
		end := pos + size
		pos = end
		runeW := RuneWidth(runeI)

		switch {
		case runeI == '\n':
			s.handleNewline()
		case runeI == '\t':
			s.handleTab(pos-size, end)
		case unicodeIsSpace(runeI):
			s.handleSpace(pos-size, end, runeW)
		case runeI == '-':
			s.handleBreakpoint(pos-size, end, runeW)
		case hasBreakpoints && s.isBreakpointRune(runeI):
			s.handleBreakpoint(pos-size, end, runeW)
		default:
			s.handleWordRune(pos-size, end, runeW)
		}
	}

	s.flushRemaining()
}

func isANSISGR(seq []byte) bool {
	return len(seq) >= 3 && seq[0] == '\x1b' && seq[1] == '[' && seq[len(seq)-1] == 'm'
}

func isResetSeq(seq []byte) bool {
	return len(seq) == 3 && seq[2] == 'm' || len(seq) == 4 && seq[2] == '0' && seq[3] == 'm'
}

// updateLineStyleAfterFlush updates lineStyle based on ANSI sequences in the
// word/space data that was just flushed into lineBuf. data[start:end] was
// appended to lineBuf.
func (s *wrapState) updateLineStyleAfterFlush(start, end int) {
	pos := start

	for pos < end {
		if s.data[pos] != '\x1b' {
			pos++

			continue
		}

		seqEnd := skipANSI(s.data, pos)
		seq := s.data[pos:seqEnd]

		if isANSISGR(seq) {
			if isResetSeq(seq) {
				s.lineStyle = s.lineStyle[:0]
			} else {
				s.lineStyle = append(s.lineStyle, seq...)
			}
		}

		pos = seqEnd
	}
}

func (s *wrapState) handleNewline() {
	if s.wordWidth == 0 && s.curWidth+s.spaceWidth > s.limit {
		s.hasSpace = false
		s.spaceWidth = 0
		s.curWidth = 0
	} else {
		s.flushWord()
	}

	s.emitLine()
	s.curWidth = 0
	s.hasSpace = false
	s.spaceWidth = 0
}

func (s *wrapState) handleTab(start, end int) {
	s.flushWord()

	if !s.hasSpace {
		s.spaceStart = start
		s.hasSpace = true
	}

	s.spaceEnd = end
	s.spaceWidth += 4
}

func (s *wrapState) handleSpace(start, end int, runeW int) {
	s.flushWord()

	if !s.hasSpace {
		s.spaceStart = start
		s.hasSpace = true
	}

	s.spaceEnd = end
	s.spaceWidth += runeW
}

func (s *wrapState) handleBreakpoint(start, end int, runeW int) {
	s.flushSpace()

	if s.curWidth+s.wordWidth+runeW > s.limit {
		if !s.hasWord {
			s.wordStart = start
			s.hasWord = true
		}

		s.wordEnd = end
		s.wordWidth += runeW
	} else {
		if s.hasWord {
			wordS, wordE := s.wordStart, s.wordEnd
			s.curWidth += s.wordWidth + runeW
			s.lineBuf = append(s.lineBuf, s.data[s.wordStart:end]...)
			s.hasWord = false
			s.wordWidth = 0
			s.updateLineStyleAfterFlush(wordS, wordE)

			// The breakpoint character itself is also in the flush.
			s.updateLineStyleAfterFlush(wordE-1, end)
		} else {
			s.lineBuf = append(s.lineBuf, s.data[start:end]...)
			s.curWidth += runeW
			s.updateLineStyleAfterFlush(start, end)
		}
	}
}

func (s *wrapState) handleWordRune(start, end int, runeW int) {
	if s.wordWidth+runeW > s.limit {
		s.flushWord()
	}

	if !s.hasWord {
		s.wordStart = start
		s.hasWord = true
	}

	s.wordEnd = end
	s.wordWidth += runeW

	if s.curWidth+s.wordWidth+s.spaceWidth > s.limit {
		s.newline()
	}

	if s.wordWidth == s.limit {
		s.flushWord()
	}
}

func (s *wrapState) flushSpace() {
	if !s.hasSpace {
		return
	}

	ss, se := s.spaceStart, s.spaceEnd
	s.curWidth += s.spaceWidth
	s.lineBuf = append(s.lineBuf, s.data[s.spaceStart:s.spaceEnd]...)
	s.hasSpace = false
	s.spaceWidth = 0
	s.updateLineStyleAfterFlush(ss, se)
}

func (s *wrapState) flushWord() {
	if !s.hasWord {
		return
	}

	s.flushSpace()

	ws, we := s.wordStart, s.wordEnd
	s.curWidth += s.wordWidth
	s.lineBuf = append(s.lineBuf, s.data[s.wordStart:s.wordEnd]...)
	s.hasWord = false
	s.wordWidth = 0
	s.updateLineStyleAfterFlush(ws, we)
}

func (s *wrapState) newline() {
	s.emitLine()
	s.curWidth = 0
	s.hasSpace = false
	s.spaceWidth = 0
}

// emitLine writes the current lineBuf content into the LinesBuf.
// If there are active ANSI styles at the end of the line, a reset is
// appended before emitting, and the styles are carried over to the next line.
func (s *wrapState) emitLine() {
	if len(s.lineBuf) == 0 {
		if len(s.lineStyle) > 0 {
			s.outBuf.WriteLine(s.lineStyle)
		} else {
			s.outBuf.EmptyLine()
		}

		s.lineBuf = s.lineBuf[:0]

		return
	}

	if len(s.lineStyle) > 0 {
		// Active style at end of line: append reset, emit, then carry style to next line.
		s.outBuf.WriteLine(s.lineBuf, ansiReset)
		s.carryBuf = append(s.carryBuf[:0], s.lineStyle...)
		s.lineBuf = append(s.lineBuf[:0], s.carryBuf...)
	} else {
		s.outBuf.WriteLine(s.lineBuf)
		s.lineBuf = s.lineBuf[:0]
	}
}

func (s *wrapState) flushRemaining() {
	if s.wordWidth == 0 && s.curWidth+s.spaceWidth > s.limit {
		s.hasSpace = false
		s.spaceWidth = 0
	} else {
		s.flushSpace()
	}

	s.flushWord()

	if len(s.lineBuf) > 0 {
		s.emitLine()
	}
}

func (s *wrapState) isBreakpointRune(rn rune) bool {
	for _, bp := range s.breakpoints {
		if rn == bp {
			return true
		}
	}

	return false
}

// lineExceedsLimitBytes scans data and reports whether any line's cell width
// exceeds the given limit. It also returns true if data contains a newline,
// since that means the line needs splitting.
//
//nolint:mnd
func lineExceedsLimitBytes(data []byte, limit int) bool {
	width := 0
	pos := 0
	n := len(data)

	for pos < n {
		byteI := data[pos]
		switch {
		case byteI == '\n':
			return true
		case byteI == '\t':
			width += 4
			pos++
		case byteI == '\x1b':
			pos = skipANSIPos(data, pos)

			continue
		case byteI < 0x80:
			width++
			pos++
		default:
			rn, size := utf8.DecodeRune(data[pos:])
			pos += size

			width += RuneWidth(rn)
		}

		if width > limit {
			return true
		}
	}

	return false
}

func skipANSIPos(data []byte, pos int) int {
	end := skipANSI(data, pos)
	if end == pos {
		return pos + 1
	}

	return end
}

// unicodeIsSpace reports whether the rune is a Unicode whitespace character
// (excluding non-breaking spaces), matching lipgloss.Wrap behavior.
func unicodeIsSpace(r rune) bool {
	return r <= 0x3000 && isSpaceTable(r)
}

// isSpaceTable covers ASCII + common Unicode whitespace up to U+3000.
// Mirrors unicode.IsSpace but excludes NBSP (U+00A0) and is inlined.
//
//nolint:mnd
func isSpaceTable(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680, 0x2000, 0x2001,
		0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200A,
		0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return r != 0xA0
	}

	return false
}
