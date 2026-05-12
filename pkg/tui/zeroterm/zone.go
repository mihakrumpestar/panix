package zeroterm

import (
	"bytes"
	"strconv"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

var zoneIDs = make(map[string]uint16)

var currentLines *buffer.LinesBufDiff

func SetCurrentLines(lines *buffer.LinesBufDiff) { currentLines = lines }
func CurrentLines() *buffer.LinesBufDiff         { return currentLines }

func EnsureZone(name string) uint16 {
	if id, ok := zoneIDs[name]; ok {
		return id
	}

	id := uint16(len(zoneIDs)) + 1
	zoneIDs[name] = id

	return id
}

func ZoneID(name string) uint16 { return zoneIDs[name] }

// formatZone appends \x1b[<id>z (open) or \x1b[/<id>z (close) to dst.
func formatZone(dst []byte, id uint16, close bool) []byte {
	if close {
		dst = append(dst, "\x1b[/"...)
	} else {
		dst = append(dst, "\x1b["...)
	}

	dst = strconv.AppendInt(dst, int64(id), 10)
	dst = append(dst, 'z')

	return dst
}

// FormatZoneOpen appends \x1b[<id>z to dst. Zero allocations.
func FormatZoneOpen(dst []byte, id uint16) []byte { return formatZone(dst, id, false) }

// FormatZoneClose appends \x1b[/<id>z to dst. Zero allocations.
func FormatZoneClose(dst []byte, id uint16) []byte { return formatZone(dst, id, true) }

// MarkBufByID wraps each line of view (split by \n) with zone open/close
// markers and appends the resulting lines to dst. Zero allocations.
// An empty view still produces one zone-marked empty line.
func MarkBufByID(id uint16, view []byte, dst *buffer.LinesBuf) {
	var openBuf, closeBuf [16]byte

	open := formatZone(openBuf[:0], id, false)
	close := formatZone(closeBuf[:0], id, true)

	if len(view) == 0 {
		dst.WriteLine3(open, nil, close)

		return
	}

	for len(view) > 0 {
		idx := bytes.IndexByte(view, '\n')
		if idx < 0 {
			dst.WriteLine3(open, view, close)

			return
		}

		dst.WriteLine3(open, view[:idx], close)
		view = view[idx+1:]
	}
}

// MarkLinesByID wraps each line from src with zone open/close markers
// and appends the resulting lines to dst. Zero allocations.
// When src is empty, a single zone-marked empty line is still produced.
func MarkLinesByID(id uint16, src *buffer.LinesBuf, dst *buffer.LinesBuf) {
	var openBuf, closeBuf [16]byte

	open := formatZone(openBuf[:0], id, false)
	close := formatZone(closeBuf[:0], id, true)

	n := src.Len()
	if n == 0 {
		dst.WriteLine3(open, nil, close)

		return
	}

	for i := range n {
		dst.WriteLine3(open, src.Line(i), close)
	}
}

func IsZoneAtLine(line []byte, col int, zoneName string) bool {
	id := ZoneID(zoneName)

	return id != 0 && ZoneIDAtCol(line, col) == id
}

func ZoneIDAtCol(line []byte, targetCol int) uint16 {
	col := 0
	activeZone := uint16(0)
	zoneStack := make([]uint16, 0, 8) //nolint:mnd

	pos := 0
	for pos < len(line) {
		b := line[pos]

		if b == '\x1b' {
			pos, activeZone, zoneStack = parseZoneMarker(line, pos, activeZone, zoneStack)

			continue
		}

		if (b >= 0x20 && b < 0x7F) || b >= 0xC0 {
			if col == targetCol {
				return activeZone
			}

			col++
		}

		pos++
	}

	if targetCol >= col {
		return activeZone
	}

	return 0
}

// parseZoneMarker skips non-zone ESC sequences and parses \x1b[<digits>z
// or \x1b[/<digits>z. Returns new pos, activeZone, zoneStack.
func parseZoneMarker(line []byte, pos int, activeZone uint16, zoneStack []uint16) (int, uint16, []uint16) {
	pos++

	if pos >= len(line) || line[pos] != '[' {
		if pos < len(line) {
			pos++
		}

		return pos, activeZone, zoneStack
	}

	pos++

	close := pos < len(line) && line[pos] == '/'
	if close {
		pos++
	}

	digitStart := pos
	for pos < len(line) && line[pos] >= '0' && line[pos] <= '9' {
		pos++
	}

	if pos < len(line) && line[pos] == 'z' {
		id, err := strconv.ParseUint(string(line[digitStart:pos]), 10, 16)
		pos++

		if err != nil {
			return pos, activeZone, zoneStack
		}

		uid := uint16(id)

		if close {
			if len(zoneStack) > 0 && zoneStack[len(zoneStack)-1] == uid {
				zoneStack = zoneStack[:len(zoneStack)-1]
			}

			if len(zoneStack) == 0 {
				return pos, 0, zoneStack
			}

			return pos, zoneStack[len(zoneStack)-1], zoneStack
		}

		return pos, uid, append(zoneStack, uid)
	}

	// Not a zone marker — skip rest of CSI sequence.
	for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x3F {
		pos++
	}

	for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x2F {
		pos++
	}

	for pos < len(line) && line[pos] >= 0x30 && line[pos] <= 0x3F {
		pos++
	}

	if pos < len(line) && line[pos] >= 0x40 && line[pos] <= 0x7E {
		pos++
	}

	return pos, activeZone, zoneStack
}
