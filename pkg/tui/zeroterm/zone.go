package zeroterm

import (
	"strconv"
	"strings"
	"sync"

	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
)

type ZoneManager struct {
	mu     sync.RWMutex
	names  []string
	byName map[string]uint16
	nextID uint16
	active map[uint16]int
}

var globalZones = newZoneManager()

var currentLines *linesbuffer.LinesBuffer

func SetCurrentLines(lines *linesbuffer.LinesBuffer) { currentLines = lines }

func CurrentLines() *linesbuffer.LinesBuffer { return currentLines }

func newZoneManager() *ZoneManager {
	return &ZoneManager{
		byName: make(map[string]uint16),
		active: make(map[uint16]int),
	}
}

func (z *ZoneManager) GetOrCreate(name string) uint16 {
	z.mu.Lock()
	defer z.mu.Unlock()

	if id, ok := z.byName[name]; ok {
		return id
	}

	z.nextID++
	zoneID := z.nextID

	z.byName[name] = zoneID
	for int(zoneID) >= len(z.names) {
		z.names = append(z.names, "")
	}

	z.names[zoneID] = name

	return zoneID
}

func (z *ZoneManager) Name(id uint16) string {
	z.mu.RLock()
	defer z.mu.RUnlock()

	if int(id) < len(z.names) {
		return z.names[id]
	}

	return ""
}

func (z *ZoneManager) ID(name string) uint16 {
	z.mu.RLock()
	defer z.mu.RUnlock()

	return z.byName[name]
}

func (z *ZoneManager) Reset() {
	z.mu.Lock()
	for k := range z.active {
		delete(z.active, k)
	}
	z.mu.Unlock()
}

func (z *ZoneManager) acquire(id uint16) {
	z.mu.Lock()
	z.active[id]++
	z.mu.Unlock()
}

func (z *ZoneManager) release(id uint16) {
	z.mu.Lock()
	if z.active[id] > 0 {
		z.active[id]--
	}
	z.mu.Unlock()
}

func Mark(name string, view string) string {
	id := globalZones.GetOrCreate(name)
	start := "\x1b[" + strconv.Itoa(int(id)) + "z"
	end := "\x1b[/" + strconv.Itoa(int(id)) + "z"

	estimatedLen := len(view) + (len(start)+len(end))*max(1, strings.Count(view, "\n")+1)

	var builder strings.Builder
	builder.Grow(estimatedLen)

	first := true
	for line := range strings.SplitSeq(view, "\n") {
		if !first {
			builder.WriteByte('\n')
		}

		builder.WriteString(start)
		builder.WriteString(line)
		builder.WriteString(end)

		first = false
	}

	return builder.String()
}

// EnsureZone creates the zone if it doesn't exist and returns its ID.
// Use this to pre-compute zone IDs at row-setup time instead of per-render.
func EnsureZone(name string) uint16 {
	return globalZones.GetOrCreate(name)
}

func IsZoneAtLine(line []byte, col int, zoneName string) bool {
	id := globalZones.ID(zoneName)
	if id == 0 {
		return false
	}

	return zoneIDAtCol(line, col) == id
}

func zoneIDAtCol(line []byte, targetCol int) uint16 {
	col := 0
	pos := 0
	activeZone := uint16(0)
	zoneStack := make([]uint16, 0, 8) //nolint:mnd

	for pos < len(line) {
		char := line[pos]

		if char == '\x1b' {
			newPos, newActive, newStack := parseZoneEscape(line, pos, activeZone, zoneStack)
			pos = newPos
			activeZone = newActive
			zoneStack = newStack

			continue
		}

		if advanceCol(char) {
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

// advanceCol reports whether the byte at the current position occupies a visible
// cell and should increment the column counter.
func advanceCol(char byte) bool {
	return (char >= 0x20 && char < 0x7F) || char >= 0xC0
}

// parseZoneEscape parses an ANSI escape sequence starting at pos (the ESC byte).
// It returns the new position, updated activeZone, and updated zoneStack.
func parseZoneEscape(line []byte, pos int, activeZone uint16, zoneStack []uint16) (int, uint16, []uint16) {
	pos++

	if pos >= len(line) {
		return pos, activeZone, zoneStack
	}

	next := line[pos]
	pos++

	if next != '[' {
		return pos, activeZone, zoneStack
	}

	paramStart := pos
	pos = scanCSIParams(line, pos)

	intermediateStart := pos
	for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x2F {
		pos++
	}

	trailingParamStart := pos
	pos = scanCSITrailingParams(line, pos)

	if pos >= len(line) || line[pos] < 0x40 || line[pos] > 0x7E {
		return pos, activeZone, zoneStack
	}

	finalByte := line[pos]
	params := line[paramStart:intermediateStart]
	intermediates := line[intermediateStart:trailingParamStart]
	trailingParams := line[trailingParamStart:pos]
	pos++

	if finalByte == 'z' {
		activeZone, zoneStack = applyZoneMarker(params, intermediates, trailingParams, activeZone, zoneStack)
	}

	return pos, activeZone, zoneStack
}

// scanCSIParams advances pos past CSI parameter bytes (0x30-0x3F).
func scanCSIParams(line []byte, pos int) int {
	for pos < len(line) && line[pos] >= 0x30 && line[pos] <= 0x3F {
		pos++
	}

	return pos
}

// scanCSITrailingParams advances pos past trailing CSI parameter bytes (0x30-0x3F).
func scanCSITrailingParams(line []byte, pos int) int {
	for pos < len(line) && line[pos] >= 0x30 && line[pos] <= 0x3F {
		pos++
	}

	return pos
}

// applyZoneMarker updates the active zone and stack based on an open or close
// zone marker that was already identified by its final byte 'z'.
func applyZoneMarker(params, intermediates, trailingParams []byte, activeZone uint16, zoneStack []uint16) (uint16, []uint16) {
	if len(intermediates) > 0 && intermediates[0] == '/' {
		// Close marker: \x1b[/<id>z
		id, err := strconv.ParseUint(string(trailingParams), 10, 16)
		if err != nil {
			return activeZone, zoneStack
		}

		if len(zoneStack) > 0 && zoneStack[len(zoneStack)-1] == uint16(id) {
			zoneStack = zoneStack[:len(zoneStack)-1]
		}

		if len(zoneStack) == 0 {
			return 0, zoneStack
		}

		return zoneStack[len(zoneStack)-1], zoneStack
	}

	// Open marker: \x1b[<id>z
	id, err := strconv.ParseUint(string(params), 10, 16)
	if err != nil {
		return activeZone, zoneStack
	}

	return uint16(id), append(zoneStack, uint16(id))
}
