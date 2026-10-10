package zeroterm

import (
	"bytes"
	"math/rand"
	"strconv"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

// ZoneID identifies a clickable zone within rendered output.
// Stores the raw uint32 for cheap equality checks and pre-rendered
// ANSI markers for zero-alloc formatting at runtime.
type ZoneID struct {
	id    uint32
	open  []byte
	close []byte
}

// NewZoneID generates a random zone ID. Call once per zone at component init.
func NewZoneID() ZoneID {
	return newZoneID(rand.Uint32()) //nolint:gosec // G404: zone IDs don't need crypto-grade randomness
}

func newZoneID(id uint32) ZoneID {
	d := strconv.AppendUint(nil, uint64(id), 10) //nolint:mnd

	return ZoneID{
		id:    id,
		open:  append(append([]byte("\x1b["), d...), 'z'),
		close: append(append([]byte("\x1b["), d...), '/', 'z'),
	}
}

// FormatOpen appends the pre-rendered \x1b[<id>z to dst.
func (id ZoneID) FormatOpen(dst []byte) []byte {
	return append(dst, id.open...)
}

// FormatClose appends the pre-rendered \x1b[<id>/z to dst.
func (id ZoneID) FormatClose(dst []byte) []byte {
	return append(dst, id.close...)
}

// Equal reports whether two zone IDs are the same.
func (id ZoneID) Equal(other ZoneID) bool { return id.id == other.id }

// MarkBuf wraps each line of view (split by \n) with zone open/close
// markers and appends the resulting lines to dst.
// An empty view still produces one zone-marked empty line.
func (id ZoneID) MarkBuf(view []byte, dst *buffer.LinesBuf) {
	if len(view) == 0 {
		dst.WriteLine3(id.open, nil, id.close)

		return
	}

	for len(view) > 0 {
		idx := bytes.IndexByte(view, '\n')
		if idx < 0 {
			dst.WriteLine3(id.open, view, id.close)

			return
		}

		dst.WriteLine3(id.open, view[:idx], id.close)
		view = view[idx+1:]
	}
}

// MarkLines wraps each line from src with zone open/close markers
// and appends the resulting lines to dst.
// When src is empty, a single zone-marked empty line is still produced.
func (id ZoneID) MarkLines(src *buffer.LinesBuf, dst *buffer.LinesBuf) {
	length := src.Len()
	if length == 0 {
		dst.WriteLine3(id.open, nil, id.close)

		return
	}

	for i := range length {
		dst.WriteLine3(id.open, src.Line(i), id.close)
	}
}

// ZoneIDAtCol returns the zone ID active at the given column in line,
// or (zero-value, false) if no zone marker covers that column.
func ZoneIDAtCol(line []byte, targetCol int) (ZoneID, bool) {
	col := 0
	zoneStack := make([]ZoneID, 0, 8) //nolint:mnd

	for pos := 0; pos < len(line); {
		byteI := line[pos]

		if byteI == '\x1b' {
			pos, zoneStack = parseZoneMarker(line, pos, zoneStack)

			continue
		}

		if (byteI >= 0x20 && byteI < 0x7F) || byteI >= 0xC0 {
			if col == targetCol {
				if len(zoneStack) > 0 {
					return zoneStack[len(zoneStack)-1], true
				}

				return ZoneID{}, false
			}

			col++
		}

		pos++
	}

	if targetCol >= col && len(zoneStack) > 0 {
		return zoneStack[len(zoneStack)-1], true
	}

	return ZoneID{}, false
}

func skipNonZoneCSI(line []byte, pos int) int {
	for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x3F {
		pos++
	}

	if pos < len(line) && line[pos] >= 0x40 && line[pos] <= 0x7E {
		pos++
	}

	return pos
}

// parseZoneBody parses a zone marker body directly after the '[' byte:
// <digits>z (open) or <digits>/z (close; parameters precede the
// intermediate byte per ECMA-48). Returns (end, digits, isClose, ok);
// ok is false when the bytes do not form a marker, and end is then the
// position where non-marker scanning should resume.
func parseZoneBody(line []byte, pos int) (int, []byte, bool, bool) {
	digitEnd := scanDigits(line, pos)
	if digitEnd == pos {
		return pos, nil, false, false
	}

	digits := line[pos:digitEnd]

	switch {
	case digitEnd < len(line) && line[digitEnd] == 'z': // open \x1b[<id>z
		return digitEnd + 1, digits, false, true
	case digitEnd+1 < len(line) && line[digitEnd] == '/' && line[digitEnd+1] == 'z': // close \x1b[<id>/z
		return digitEnd + 2, digits, true, true
	default:
		return digitEnd, nil, false, false
	}
}

// scanDigits returns the position just past a run of ASCII digits
// starting at pos (which may equal pos when there are no digits).
func scanDigits(line []byte, pos int) int {
	for pos < len(line) && line[pos] >= '0' && line[pos] <= '9' {
		pos++
	}

	return pos
}

// parseZoneMarker consumes one escape sequence at pos (pointing at
// \x1b) during the ZoneIDAtCol walk: zone markers push/pop zoneStack,
// anything else is skipped as a non-zone CSI or two-byte escape.
func parseZoneMarker(line []byte, pos int, zoneStack []ZoneID) (int, []ZoneID) {
	pos++ // skip \x1b

	if pos >= len(line) {
		return pos, zoneStack
	}

	if line[pos] != '[' {
		return pos + 1, zoneStack // consume ESC + following byte
	}

	end, digits, isClose, ok := parseZoneBody(line, pos+1)
	if !ok {
		return skipNonZoneCSI(line, end), zoneStack
	}

	uid := zoneIDFromDigits(digits)

	if isClose {
		if len(zoneStack) > 0 && zoneStack[len(zoneStack)-1].id == uid.id {
			zoneStack = zoneStack[:len(zoneStack)-1]
		}

		return end, zoneStack
	}

	return end, append(zoneStack, uid)
}

// zoneIDFromDigits builds a ZoneID from raw decimal digit bytes.
func zoneIDFromDigits(digits []byte) ZoneID {
	var id uint32
	for _, d := range digits {
		id = id*10 + uint32(d-'0') //nolint:mnd
	}

	return newZoneID(id)
}
