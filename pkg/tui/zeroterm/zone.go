package zeroterm

import (
	"strconv"
	"strings"
	"sync"
)

type ZoneManager struct {
	mu     sync.RWMutex
	names  []string
	byName map[string]uint16
	nextID uint16
	active map[uint16]int
}

var globalZones = newZoneManager()

var currentLines []string

func SetCurrentLines(lines []string) { currentLines = lines }

func CurrentLines() []string { return currentLines }

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
	id := z.nextID

	z.byName[name] = id
	for int(id) >= len(z.names) {
		z.names = append(z.names, "")
	}

	z.names[id] = name

	return id
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

func (z *ZoneManager) acquire(id uint16) uint16 {
	z.mu.Lock()
	z.active[id]++
	z.mu.Unlock()

	return id
}

func (z *ZoneManager) release(id uint16) uint16 {
	z.mu.Lock()
	if z.active[id] > 0 {
		z.active[id]--
	}
	z.mu.Unlock()

	return id
}

func (z *ZoneManager) Reset() {
	z.mu.Lock()
	z.active = make(map[uint16]int)
	z.mu.Unlock()
}

func Mark(name string, view string) string {
	id := globalZones.GetOrCreate(name)
	start := "\x1b[" + strconv.Itoa(int(id)) + "z"
	end := "\x1b[/" + strconv.Itoa(int(id)) + "z"

	estimatedLen := len(view) + (len(start)+len(end))*max(1, strings.Count(view, "\n")+1)

	var b strings.Builder
	b.Grow(estimatedLen)

	first := true
	for line := range strings.SplitSeq(view, "\n") {
		if !first {
			b.WriteByte('\n')
		}

		b.WriteString(start)
		b.WriteString(line)
		b.WriteString(end)

		first = false
	}

	return b.String()
}

func GetZoneName(id uint16) string {
	return globalZones.Name(id)
}

// EnsureZone creates the zone if it doesn't exist and returns its ID.
// Use this to pre-compute zone IDs at row-setup time instead of per-render.
func EnsureZone(name string) uint16 {
	return globalZones.GetOrCreate(name)
}

func GetZoneID(name string) uint16 {
	return globalZones.ID(name)
}

func ResetZones() {
	globalZones.Reset()
}

func ZoneNames() []string {
	globalZones.mu.RLock()
	defer globalZones.mu.RUnlock()

	names := make([]string, 0, len(globalZones.byName))
	for name := range globalZones.byName {
		names = append(names, name)
	}

	return names
}

func IsZoneAtLine(line string, x int, zoneName string) bool {
	id := globalZones.ID(zoneName)
	if id == 0 {
		return false
	}

	return zoneIDAtCol(line, x) == id
}

func ZoneAtLine(line string, x int) string {
	id := zoneIDAtCol(line, x)
	if id == 0 {
		return ""
	}

	return globalZones.Name(id)
}

//nolint:gocognit
func zoneIDAtCol(line string, targetCol int) uint16 {
	col := 0
	pos := 0
	activeZone := uint16(0)
	zoneStack := make([]uint16, 0, 8)

	for pos < len(line) {
		ch := line[pos]

		if ch == '\x1b' {
			pos++

			if pos >= len(line) {
				break
			}

			next := line[pos]
			pos++

			if next != '[' {
				continue
			}

			paramStart := pos
			for pos < len(line) && line[pos] >= 0x30 && line[pos] <= 0x3F {
				pos++
			}

			intermediateStart := pos
			for pos < len(line) && line[pos] >= 0x20 && line[pos] <= 0x2F {
				pos++
			}

			trailingParamStart := pos
			for pos < len(line) && line[pos] >= 0x30 && line[pos] <= 0x3F {
				pos++
			}

			if pos >= len(line) || line[pos] < 0x40 || line[pos] > 0x7E {
				continue
			}

			finalByte := line[pos]
			params := line[paramStart:intermediateStart]
			intermediates := line[intermediateStart:trailingParamStart]
			trailingParams := line[trailingParamStart:pos]
			pos++

			// Zone open marker: \x1b[<id>z
			// Zone close marker: \x1b[/<id>z  ('/' is an ANSI intermediate byte)
			if finalByte == 'z' {
				if len(intermediates) > 0 && intermediates[0] == '/' {
					id, err := strconv.ParseUint(trailingParams, 10, 16)
					if err == nil {
						if len(zoneStack) > 0 && zoneStack[len(zoneStack)-1] == uint16(id) {
							zoneStack = zoneStack[:len(zoneStack)-1]
						}

						if len(zoneStack) == 0 {
							activeZone = 0
						} else {
							activeZone = zoneStack[len(zoneStack)-1]
						}
					}
				} else {
					id, err := strconv.ParseUint(params, 10, 16)
					if err == nil {
						activeZone = uint16(id)
						zoneStack = append(zoneStack, uint16(id))
					}
				}
			}

			continue
		}

		if ch >= 0x20 && ch < 0x7F {
			if col == targetCol {
				return activeZone
			}

			col++
		} else if ch >= 0xC0 {
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
