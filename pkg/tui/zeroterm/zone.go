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

func (id ZoneID) Equal(other ZoneID) bool { return id.id == other.id }

// NewZoneID generates a random zone ID. Call once per zone at component init.
func NewZoneID() ZoneID {
	return newZoneID(rand.Uint32())
}

func newZoneID(id uint32) ZoneID {
	d := strconv.AppendUint(nil, uint64(id), 10)

	return ZoneID{
		id:    id,
		open:  append(append([]byte("\x1b["), d...), 'z'),
		close: append(append([]byte("\x1b[/"), d...), 'z'),
	}
}

// FormatOpen appends the pre-rendered \x1b[<id>z to dst.
func (id ZoneID) FormatOpen(dst []byte) []byte {
	return append(dst, id.open...)
}

// FormatClose appends the pre-rendered \x1b[/<id>z to dst.
func (id ZoneID) FormatClose(dst []byte) []byte {
	return append(dst, id.close...)
}

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
	n := src.Len()
	if n == 0 {
		dst.WriteLine3(id.open, nil, id.close)

		return
	}

	for i := range n {
		dst.WriteLine3(id.open, src.Line(i), id.close)
	}
}

// ZoneIDAtCol returns the zone ID active at the given column in line,
// or (zero-value, false) if no zone marker covers that column.
func ZoneIDAtCol(line []byte, targetCol int) (ZoneID, bool) {
	col := 0
	zoneStack := make([]ZoneID, 0, 8) //nolint:mnd

	for pos := 0; pos < len(line); {
		b := line[pos]

		if b == '\x1b' {
			pos, zoneStack = parseZoneMarker(line, pos, zoneStack)

			continue
		}

		if (b >= 0x20 && b < 0x7F) || b >= 0xC0 {
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

// parseZoneMarker skips non-zone ESC sequences and handles \x1b[<digits>z
// (open) and \x1b[/<digits>z (close). Returns the new position and updated stack.
func parseZoneMarker(line []byte, pos int, zoneStack []ZoneID) (int, []ZoneID) {
	pos++ // skip ESC

	if pos >= len(line) || line[pos] != '[' {
		if pos < len(line) {
			pos++
		}

		return pos, zoneStack
	}

	pos++ // skip '['

	isClose := pos < len(line) && line[pos] == '/'
	if isClose {
		pos++
	}

	digitStart := pos
	for pos < len(line) && line[pos] >= '0' && line[pos] <= '9' {
		pos++
	}

	if pos < len(line) && line[pos] == 'z' {
		uid := zoneIDFromDigits(line[digitStart:pos])
		pos++

		if isClose {
			if len(zoneStack) > 0 && zoneStack[len(zoneStack)-1].id == uid.id {
				zoneStack = zoneStack[:len(zoneStack)-1]
			}

			return pos, zoneStack
		}

		return pos, append(zoneStack, uid)
	}

	// Not a zone marker — skip CSI parameter/intermediate bytes then final byte.
	for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x3F {
		pos++
	}

	if pos < len(line) && line[pos] >= 0x40 && line[pos] <= 0x7E {
		pos++
	}

	return pos, zoneStack
}

// zoneIDFromDigits builds a ZoneID from raw decimal digit bytes.
func zoneIDFromDigits(digits []byte) ZoneID {
	var id uint32
	for _, d := range digits {
		id = id*10 + uint32(d-'0')
	}

	return newZoneID(id)
}
