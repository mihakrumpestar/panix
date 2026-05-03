package render

import (
	"fmt"
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

var currentBuf *CellBuf

func SetCurrentBuf(buf *CellBuf) { currentBuf = buf }

func CurrentBuf() *CellBuf { return currentBuf }

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
	// Pre-compute zone marker strings to avoid repeated concatenation.
	start := "\x1b[" + strconv.Itoa(int(id)) + "z"
	end := "\x1b[/" + strconv.Itoa(int(id)) + "z"

	// Estimate output size: original + markers per line.
	// Each line gets start+len(end) extra bytes. Estimate ~1.5 newlines.
	estimatedLen := len(view) + (len(start)+len(end))*max(1, strings.Count(view, "\n")+1)
	var b strings.Builder
	b.Grow(estimatedLen)

	// Wrap each line individually so that zone markers don't leak into
	// tree prefixes or other content prepended to intermediate lines.
	// A single pair wrapping the whole multi-line string would cause the
	// zone ID to persist across newlines, assigning it to tree prefixes
	// on subsequent lines.
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

func IsZoneAt(buf *CellBuf, x, y int, zoneName string) bool {
	id := globalZones.ID(zoneName)
	if id == 0 {
		return false
	}

	cell := buf.CellAt(x, y)

	return cell.ZoneID == id
}

func ZoneAt(buf *CellBuf, x, y int) string {
	cell := buf.CellAt(x, y)
	if cell.ZoneID == 0 {
		return ""
	}

	return globalZones.Name(cell.ZoneID)
}

//nolint:unused
func fmtMark(id uint16) string {
	return fmt.Sprintf("\x1b[%dz", id)
}
