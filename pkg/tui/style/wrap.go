// Based on charm.land/lipgloss/v2 — Copyright (c) 2021-2026 Charmbracelet, Inc.
// Licensed under the MIT License. See pkg/LICENSE for details.

package style

import (
	"strings"
	"unicode/utf8"
)

// Wrap wraps a string to the given cell width, preserving ANSI escape sequences
// and inserting newlines at word boundaries. It replaces lipgloss.Wrap in hot
// paths, using CellWidth instead of the heavy uax29 grapheme clustering.
//
// breakpoints is a string of characters considered valid word-break points
// (in addition to whitespace and hyphens). Pass "" for default behavior.
//
//nolint:cyclop,funlen
func Wrap(str string, limit int, breakpoints string) string {
	if limit < 1 || str == "" {
		return str
	}

	// Ultra-fast path: byte length <= limit and no newlines means no wrapping
	// needed. Each visible rune occupies at least 1 byte, so cell width <= byte
	// length <= limit.
	if len(str) <= limit && strings.IndexByte(str, '\n') < 0 {
		return str
	}

	// Fast path: scan to check if any line exceeds the width limit.
	// If no line exceeds the limit, return the input as-is.
	if !lineExceedsLimit(str, limit) {
		return str
	}

	s := wrapState{ //nolint:varnamelen
		str:         str,
		limit:       limit,
		breakpoints: breakpoints,
	}
	s.buf.Grow(len(str) + len(str)/limit + 1)

	pos := 0
	n := len(str)
	hasBreakpoints := breakpoints != ""

	for pos < n {
		b := str[pos] //nolint:varnamelen

		if b == '\x1b' {
			end := skipANSI(str, pos)

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
		if b < 0x80 { //nolint:mnd
			pos++

			switch {
			case b == '\n':
				s.handleNewline()
			case b == '\t':
				s.handleTab(pos-1, pos)
			case b == ' ' || b == '\v' || b == '\f' || b == '\r':
				s.handleSpace(pos-1, pos, 1)
			case b == '-':
				s.handleBreakpoint(pos-1, pos, 1)
			case hasBreakpoints && s.isBreakpointRune(rune(b)):
				s.handleBreakpoint(pos-1, pos, 1)
			default:
				s.handleWordRune(pos-1, pos, 1)
			}

			continue
		}

		rn, size := utf8.DecodeRuneInString(str[pos:]) //nolint:varnamelen
		end := pos + size
		pos = end
		rw := RuneWidth(rn) //nolint:varnamelen

		switch {
		case rn == '\n':
			s.handleNewline()
		case rn == '\t':
			s.handleTab(pos-size, end)
		case unicodeIsSpace(rn):
			s.handleSpace(pos-size, end, rw)
		case rn == '-':
			s.handleBreakpoint(pos-size, end, rw)
		case hasBreakpoints && s.isBreakpointRune(rn):
			s.handleBreakpoint(pos-size, end, rw)
		default:
			s.handleWordRune(pos-size, end, rw)
		}
	}

	s.flushRemaining()

	return s.buf.String()
}

// wrapState tracks word-wrapping state using byte ranges into the original
// input string instead of intermediate buffers. This eliminates per-rune
// WriteString calls and two buffer allocations (word, space).
type wrapState struct {
	buf         strings.Builder
	str         string
	limit       int
	breakpoints string
	curWidth    int

	// Word: str[wordStart:wordEnd] is the current pending word (may include
	// ANSI sequences, which contribute zero to wordWidth).
	wordStart int
	wordEnd   int
	wordWidth int
	hasWord   bool

	// Space: str[spaceStart:spaceEnd] is the pending whitespace.
	spaceStart int
	spaceEnd   int
	spaceWidth int
	hasSpace   bool
}

func (s *wrapState) handleNewline() {
	if s.wordWidth == 0 && s.curWidth+s.spaceWidth > s.limit {
		s.hasSpace = false
		s.spaceWidth = 0
		s.curWidth = 0
	} else {
		s.flushWord()
	}

	s.newline()
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

//nolint:varnamelen
func (s *wrapState) handleSpace(start, end int, rw int) {
	s.flushWord()

	if !s.hasSpace {
		s.spaceStart = start
		s.hasSpace = true
	}

	s.spaceEnd = end
	s.spaceWidth += rw
}

//nolint:varnamelen
func (s *wrapState) handleBreakpoint(start, end int, rw int) {
	s.flushSpace()

	if s.curWidth+s.wordWidth+rw > s.limit {
		if !s.hasWord {
			s.wordStart = start
			s.hasWord = true
		}

		s.wordEnd = end
		s.wordWidth += rw
	} else {
		// Merge word + breakpoint into a single WriteString call.
		if s.hasWord {
			s.curWidth += s.wordWidth + rw
			s.buf.WriteString(s.str[s.wordStart:end])
			s.hasWord = false
			s.wordWidth = 0
		} else {
			s.buf.WriteString(s.str[start:end])
			s.curWidth += rw
		}
	}
}

//nolint:varnamelen
func (s *wrapState) handleWordRune(start, end int, rw int) {
	if s.wordWidth+rw > s.limit {
		s.flushWord()
	}

	if !s.hasWord {
		s.wordStart = start
		s.hasWord = true
	}

	s.wordEnd = end
	s.wordWidth += rw

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

	s.curWidth += s.spaceWidth
	s.buf.WriteString(s.str[s.spaceStart:s.spaceEnd])
	s.hasSpace = false
	s.spaceWidth = 0
}

func (s *wrapState) flushWord() {
	if !s.hasWord {
		return
	}

	s.flushSpace()

	s.curWidth += s.wordWidth
	s.buf.WriteString(s.str[s.wordStart:s.wordEnd])
	s.hasWord = false
	s.wordWidth = 0
}

func (s *wrapState) newline() {
	s.buf.WriteByte('\n')
	s.curWidth = 0
	s.hasSpace = false
	s.spaceWidth = 0
}

func (s *wrapState) flushRemaining() {
	if s.wordWidth == 0 && s.curWidth+s.spaceWidth > s.limit {
		s.hasSpace = false
		s.spaceWidth = 0
	} else {
		s.flushSpace()
	}

	s.flushWord()
}

func (s *wrapState) isBreakpointRune(rn rune) bool {
	for _, bp := range s.breakpoints {
		if rn == bp {
			return true
		}
	}

	return false
}

// lineExceedsLimit scans str and reports whether any line's cell width exceeds
// the given limit. Used as a pre-check before the full wrapping algorithm to
// avoid builder allocation when no wrapping is needed.
//
//nolint:mnd
func lineExceedsLimit(str string, limit int) bool {
	width := 0
	pos := 0
	n := len(str)

	for pos < n {
		switch {
		case str[pos] == '\x1b':
			pos = skipANSI(str, pos)
		case str[pos] == '\n':
			width = 0
			pos++
		case str[pos] == '\t':
			width += 4
			if width > limit {
				return true
			}

			pos++
		case str[pos] < 0x80:
			width++
			if width > limit {
				return true
			}

			pos++
		default:
			rn, size := utf8.DecodeRuneInString(str[pos:])
			pos += size

			width += RuneWidth(rn)
			if width > limit {
				return true
			}
		}
	}

	return false
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
