package style

// SpliceCells replaces the cells of line in [start, end) with replacement,
// preserving the content and styling of cells outside the range and the line's
// ending style state. Wide runes straddling a boundary are dropped and padded;
// cells beyond the line count as blanks; an empty range is a plain insert.
func SpliceCells(line []byte, start, end int, replacement []byte) []byte {
	if start < 0 {
		start = 0
	}

	if end < start {
		end = start
	}

	scan := scanSpliceCells(line, start, end)
	replacement = fitCells(replacement, end-start)

	tailEmpty := len(scan.tail) == 0 && scan.tailPad == 0
	// Splice separators only matter on styled input; plain rows stay plain.
	styled := len(scan.replay) > 0 || hasEscape(replacement)

	out := make([]byte, 0, len(scan.head)+len(replacement)+len(scan.tail)+len(scan.replay))
	out = append(out, scan.head...)
	out = append(out, PaddingBytes(scan.headPad)...)

	if hasEscape(scan.head) {
		out = append(out, ansiReset...)
	}

	out = append(out, replacement...)

	if tailEmpty {
		// Re-emitting the state keeps the row's end style identical to line's.
		if styled {
			out = append(out, ansiReset...)
			out = append(out, scan.replay...)
		}
	} else {
		if styled {
			out = append(out, ansiReset...)
			out = append(out, scan.replay...)
		}

		out = append(out, PaddingBytes(scan.tailPad)...)
		out = append(out, scan.tail...)
	}

	return out
}

// spliceScan holds the line segments of one SpliceCells call.
type spliceScan struct {
	head     []byte // line bytes before start
	tail     []byte // line bytes from end on
	replay   []byte // escape sequences folded from the tail's style state
	headPad  int    // blank cells for a wide rune split at start
	tailPad  int    // blank cells for a wide rune split at end
	tailOpen bool   // the tail segment has begun
}

// scanSpliceCells segments line at the start and end cells. Escape sequences
// are zero width: before the tail begins they land in head and replay, after in
// the tail.
func scanSpliceCells(line []byte, start, end int) spliceScan {
	var scan spliceScan

	pos, cell := 0, 0

	for pos < len(line) {
		if line[pos] == '\x1b' {
			next := SkipANSI(line, pos)

			if scan.tailOpen {
				scan.tail = append(scan.tail, line[pos:next]...)
			} else {
				scan.head = append(scan.head, line[pos:next]...)
				scan.replay = append(scan.replay, line[pos:next]...)
			}

			pos = next

			continue
		}

		size, width, _ := scanGlyph(line, pos)
		next := pos + size

		pos, cell = scanSpliceSegment(&scan, line, pos, next, cell, width, start, end)
	}

	if cell < start {
		// The row ends before the range: blanks fill the gap.
		scan.headPad += start - cell
	}

	return scan
}

// scanSpliceSegment files the glyph [pos, next) into its segment and returns
// the advanced byte and cell positions.
func scanSpliceSegment(
	scan *spliceScan,
	line []byte,
	pos, next, cell, width int,
	start, end int,
) (int, int) {
	cellEnd := cell + width

	switch {
	case cellEnd <= start:
		scan.head = append(scan.head, line[pos:next]...)
	case cell >= end:
		scan.tailOpen = true
		scan.tail = append(scan.tail, line[pos:next]...)
	default:
		// The rune intersects the range and is dropped: pad its cells.
		if cell < start {
			scan.headPad += start - cell
		}

		if cellEnd > end {
			scan.tailOpen = true
			scan.tailPad += cellEnd - end
		}
	}

	return next, cellEnd
}

// fitCells truncates or pads content to exactly width cells; a zero width
// range is a plain insert.
func fitCells(content []byte, width int) []byte {
	if width <= 0 {
		return content
	}

	current := CellWidth(content)

	switch {
	case current > width:
		return TruncateToWidth(content, width, false)
	case current < width:
		out := make([]byte, 0, len(content)+width-current)
		out = append(out, content...)
		out = append(out, PaddingBytes(width-current)...)

		return out
	default:
		return content
	}
}

// hasEscape reports whether b contains an ANSI escape sequence.
func hasEscape(b []byte) bool {
	return indexByte(b, '\x1b') >= 0
}
