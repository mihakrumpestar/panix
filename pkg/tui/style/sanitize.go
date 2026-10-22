package style

import (
	"unicode/utf8"
)

const (
	// asciiLimit is the first byte outside the 7-bit ASCII range.
	asciiLimit = 0x80
	// c1ControlEnd is the last Unicode C1 control (8-bit terminal control).
	c1ControlEnd = 0x9F
	// bel terminates OSC sequences.
	bel = 0x07
)

// SanitizeTerminalOutput appends the safe subset of src to dst and returns the
// extended slice. Safe output is:
//
//   - printable ASCII and valid non-control UTF-8
//   - '\n', '\t', '\r' and '\b' (callers implement their own cursor semantics)
//   - SGR sequences: CSI ... m (colour, bold, reset, and similar styling)
//
// Every other escape sequence is removed: cursor movement, erase, DEC private
// modes, OSC (including clipboard OSC 52 and hyperlink OSC 8), DCS, SOS, PM,
// APC, and bare two-byte escapes. Incomplete escape sequences are also dropped
// so a truncated sequence can never splice into following terminal input.
//
// The function is the single sanitizer shared by the target-output boundary
// (internal/executioner) and the final TUI frame guard (pkg/tui/zeroterm).
func SanitizeTerminalOutput(dst, src []byte) []byte {
	for pos := 0; pos < len(src); {
		byteI := src[pos]

		if byteI == '\x1b' {
			end, keep := scanEscape(src, pos)
			if keep {
				dst = append(dst, src[pos:end]...)
			}

			pos = end

			continue
		}

		var size int

		dst, size = appendSafeByte(dst, src[pos:])
		pos += size
	}

	return dst
}

// appendSafeByte appends one safe byte or decoded UTF-8 rune from src and
// returns the number of input bytes consumed. Unsafe input is dropped.
func appendSafeByte(dst, src []byte) ([]byte, int) {
	byteI := src[0]

	if isAllowedControl(byteI) {
		return append(dst, byteI), 1
	}

	if byteI < 0x20 || byteI == 0x7F {
		return dst, 1
	}

	if byteI < asciiLimit {
		return append(dst, byteI), 1
	}

	decodedRune, size := utf8.DecodeRune(src)
	if decodedRune == utf8.RuneError && size == 1 {
		return dst, 1 // invalid UTF-8 byte
	}

	if decodedRune >= asciiLimit && decodedRune <= c1ControlEnd {
		return dst, size // Unicode C1 control (8-bit terminal control range)
	}

	return append(dst, src[:size]...), size
}

// isAllowedControl reports whether b is a control byte whose effect is handled
// by panix rather than by the physical terminal.
func isAllowedControl(b byte) bool {
	switch b {
	case '\n', '\t', '\r', '\b':
		return true
	default:
		return false
	}
}

// scanEscape inspects the escape sequence starting at pos (which points at
// ESC) and returns the position just past it plus whether it is an SGR
// sequence to keep.
func scanEscape(src []byte, pos int) (int, bool) {
	if pos+1 >= len(src) {
		return len(src), false // lone trailing ESC
	}

	switch src[pos+1] {
	case '[':
		return scanCSI(src, pos)
	case ']':
		return scanStringSequence(src, pos, isOSCTerminator), false
	case 'P', 'X', '^', '_':
		// DCS, SOS, PM, APC: all terminated by ST (ESC \).
		return scanStringSequence(src, pos, isSTTerminator), false

	default:
		// Bare two-byte escape (for example ESC c or ESC M). Drop both bytes
		// only for the 0x40-0x5F final-byte range; any other following byte
		// is stray printable data, so drop just the ESC and keep the byte.
		if src[pos+1] >= 0x40 && src[pos+1] <= 0x5F {
			return pos + 2, false
		}

		return pos + 1, false
	}
}

// scanCSI parses a CSI sequence and keeps it only when its final byte is 'm'
// (SGR). A CSI without a final byte is incomplete and is dropped to the end of
// the buffer.
func scanCSI(src []byte, pos int) (int, bool) {
	next := pos + 2

	for next < len(src) && src[next] >= 0x20 && src[next] <= 0x3F {
		next++
	}

	if next < len(src) && src[next] >= 0x40 && src[next] <= 0x7E {
		final := src[next]

		return next + 1, final == 'm'
	}

	return len(src), false
}

// scanStringSequence skips an OSC or DCS-style sequence. It stops at the first
// terminator accepted by done; an unterminated sequence consumes the rest of
// the buffer.
func scanStringSequence(src []byte, pos int, done func([]byte, int) (int, bool)) int {
	for next := pos + 2; next < len(src); next++ {
		if end, ok := done(src, next); ok {
			return end
		}
	}

	return len(src)
}

// isOSCTerminator matches BEL or ST at pos.
func isOSCTerminator(src []byte, pos int) (int, bool) {
	if src[pos] == bel {
		return pos + 1, true
	}

	return isSTTerminator(src, pos)
}

// isSTTerminator matches ST (ESC \) at pos.
func isSTTerminator(src []byte, pos int) (int, bool) {
	if src[pos] == '\x1b' && pos+1 < len(src) && src[pos+1] == '\\' {
		return pos + 2, true
	}

	return 0, false
}
