package style

import (
	"bytes"
	"slices"
	"sync/atomic"
	"unicode/utf8"
)

// measuredWidthTable holds the terminal-measured glyph widths. It is nil until
// a probe runs, so by default CellWidth and runeWidth use the static table.
// Overrides are process-global and installed once before rendering; later reads
// are a lock-free atomic load of the immutable snapshot.
type measuredWidthTable struct {
	runes map[rune]int
	// seqByFirst indexes multi-rune glyphs by first byte, longest first.
	seqByFirst map[byte][]measuredSeq
}

// measuredSeq is one multi-rune glyph width override.
type measuredSeq struct {
	glyph []byte
	width int
}

var measuredGlyphWidths atomic.Pointer[measuredWidthTable]

// InstallMeasuredGlyphWidths installs terminal-measured glyph widths. A
// single-rune key overrides that rune everywhere runeWidth is used; a longer key
// overrides that exact sequence in CellWidth, so VS16 and ZWJ glyphs measure
// correctly. nil or an empty map restores the static table. Overrides are
// process-global: install once before rendering.
func InstallMeasuredGlyphWidths(measured map[string]int) {
	if len(measured) == 0 {
		measuredGlyphWidths.Store(nil)

		return
	}

	overrides := &measuredWidthTable{
		runes:      make(map[rune]int),
		seqByFirst: make(map[byte][]measuredSeq),
	}

	for glyph, width := range measured {
		if glyph == "" {
			continue
		}

		r, size := utf8.DecodeRuneInString(glyph)
		if size == len(glyph) {
			overrides.runes[r] = width

			continue
		}

		seq := measuredSeq{glyph: []byte(glyph), width: width}
		overrides.seqByFirst[seq.glyph[0]] = append(overrides.seqByFirst[seq.glyph[0]], seq)
	}

	for first, seqs := range overrides.seqByFirst {
		slices.SortFunc(seqs, func(a, b measuredSeq) int { return len(b.glyph) - len(a.glyph) })
		overrides.seqByFirst[first] = seqs
	}

	measuredGlyphWidths.Store(overrides)
}

// ResetMeasuredGlyphWidths clears installed overrides, restoring the static
// table. Tests that install overrides must call it.
func ResetMeasuredGlyphWidths() {
	measuredGlyphWidths.Store(nil)
}

// matchSequence returns the width and byte length of the longest installed
// sequence that prefixes data.
func (m *measuredWidthTable) matchSequence(data []byte) (int, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}

	for _, seq := range m.seqByFirst[data[0]] {
		if bytes.HasPrefix(data, seq.glyph) {
			return seq.width, len(seq.glyph), true
		}
	}

	return 0, 0, false
}
