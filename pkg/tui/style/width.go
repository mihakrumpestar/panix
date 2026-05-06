// Based on charm.land/lipgloss/v2 — Copyright (c) 2021-2026 Charmbracelet, Inc.
// Licensed under the MIT License. See pkg/LICENSE for details.

package style

import (
	"strings"
	"unsafe"

	"github.com/rivo/uniseg"
)

// CellWidth returns the maximum terminal cell width across all lines in a
// string, accounting for ANSI escape sequences (zero width) and grapheme
// clusters with proper width (emoji ZWJ sequences, skin tone modifiers, CJK
// wide chars, etc.). For single-line strings this is equivalent to the total
// visible width. For multi-line strings it returns the widest line, matching
// lipgloss.Width behavior.
//
// Fast paths:
//   - ASCII printable chars (0x20-0x7E): counted as width 1 without uniseg
//   - Zero-copy []byte via unsafe.Slice (avoids O(n²) []byte(str[pos:]) copies)
func CellWidth(str string) int {
	width := 0
	maxWidth := 0
	pos := 0
	graphemeState := -1

	// Zero-copy string→[]byte for uniseg calls. Safe because:
	// 1. uniseg.FirstGraphemeCluster doesn't modify the input
	// 2. str is alive for the duration of this function
	//nolint:gosec // G103: audited — zero-copy byte view of str, str outlives byteSlice
	byteSlice := unsafe.Slice(unsafe.StringData(str), len(str))

	for pos < len(byteSlice) {
		char := byteSlice[pos]

		// Fast ASCII path for printable characters (0x20-0x7E).
		// Handles >90% of terminal content without calling uniseg.
		if char >= 0x20 && char < 0x7F {
			graphemeState = -1
			width++
			pos++

			continue
		}

		switch char {
		case '\x1b':
			graphemeState = -1
			pos = skipANSI(str, pos)
		case '\n', '\r':
			graphemeState = -1

			if width > maxWidth {
				maxWidth = width
			}

			width = 0
			pos++
		default:
			cluster, rest, charWidth, newState := uniseg.FirstGraphemeCluster(byteSlice[pos:], graphemeState)
			graphemeState = newState
			pos = len(byteSlice) - len(rest)
			width += charWidth
			_ = cluster
		}
	}

	if width > maxWidth {
		maxWidth = width
	}

	return maxWidth
}

// skipANSI skips a complete ANSI escape sequence starting at pos (which points
// to '\x1b') and returns the position immediately after the sequence. Handles:
//
//   - CSI sequences: \x1b[<params><final>  (final byte 0x40-0x7E)
//     This includes SGR (\x1b[...m) and bubblezone markers (\x1b[...z).
//   - OSC sequences: \x1b]...<BEL> or \x1b]...\x1b\\
//   - Bare ESC sequences: \x1b<0x40-0x5F> (2-byte C1 codes like \x1b[)
//
//nolint:cyclop,mnd
func skipANSI(str string, pos int) int {
	if pos >= len(str) || str[pos] != '\x1b' {
		return pos
	}

	pos++

	if pos >= len(str) {
		return pos
	}

	next := str[pos]

	switch {
	case next == '[':
		// CSI: ESC [ <param bytes 0x30-0x3F> <intermediate bytes 0x20-0x2F>
		// <final byte 0x40-0x7E>
		pos++

		for pos < len(str) && str[pos] >= 0x20 && str[pos] <= 0x3F {
			pos++
		}

		for pos < len(str) && str[pos] >= 0x20 && str[pos] <= 0x2F {
			pos++
		}

		if pos < len(str) && str[pos] >= 0x40 && str[pos] <= 0x7E {
			pos++
		}
	case next == ']':
		// OSC: ESC ] ... <BEL (0x07)> or <ST (ESC \)>
		pos++

		for pos < len(str) {
			if str[pos] == 0x07 {
				pos++

				break
			}

			if str[pos] == '\x1b' && pos+1 < len(str) && str[pos+1] == '\\' {
				pos += 2

				break
			}

			pos++
		}
	case next >= 0x40 && next <= 0x5F:
		// 2-byte C1 escape code (e.g. ESC [ is CSI, ESC ] is OSC,
		// but we handle those above; this catches ESC O, ESC P, etc.)
		pos++
	default:
		// Unknown: just skip the ESC byte, leave pos at next byte.
	}

	return pos
}

// CountLines returns the number of lines in a string (1 for a string with no
// newlines, including empty strings). It replaces lipgloss.Height in hot
// paths with a direct newline count.
func CountLines(str string) int {
	if str == "" {
		return 1
	}

	return 1 + strings.Count(str, "\n")
}

// RuneWidth returns the display width of a rune (1 for ASCII, 2 for East Asian Wide).
//
//nolint:cyclop
func RuneWidth(runeVal rune) int {
	if runeVal >= 0x1100 &&
		(runeVal <= 0x115F ||
			runeVal == 0x2329 ||
			runeVal == 0x27C2 ||
			(runeVal >= 0x2E80 && runeVal <= 0xA4CF && runeVal != 0x303F) ||
			(runeVal >= 0xAC00 && runeVal <= 0xD7A3) ||
			(runeVal >= 0xF900 && runeVal <= 0xFAFF) ||
			(runeVal >= 0xFE10 && runeVal <= 0xFE19) ||
			(runeVal >= 0xFE30 && runeVal <= 0xFE6F) ||
			(runeVal >= 0xFF01 && runeVal <= 0xFF60) ||
			(runeVal >= 0xFFE0 && runeVal <= 0xFFE6) ||
			(runeVal >= 0x1F300 && runeVal <= 0x1F9FF)) {
		return 2 //nolint:mnd
	}

	return 1
}
