package render

import (
	"strconv"
	"strings"
	"testing"
)

func TestZoneManagerGetOrCreate(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()

	id1 := zm.GetOrCreate("zone-a")

	id2 := zm.GetOrCreate("zone-a")
	if id1 != id2 {
		t.Errorf("same name should return same ID: %d != %d", id1, id2)
	}

	id3 := zm.GetOrCreate("zone-b")
	if id3 == id1 {
		t.Error("different names should return different IDs")
	}
}

func TestZoneManagerName(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()
	id := zm.GetOrCreate("my-zone")

	name := zm.Name(id)
	if name != "my-zone" {
		t.Errorf("Name(%d) = %q, want %q", id, name, "my-zone")
	}
}

func TestZoneManagerNameInvalidID(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()

	name := zm.Name(999)
	if name != "" {
		t.Errorf("Name(999) = %q, want empty string", name)
	}
}

func TestZoneManagerID(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()
	id := zm.GetOrCreate("test")

	found := zm.ID("test")
	if found != id {
		t.Errorf("ID(test) = %d, want %d", found, id)
	}
}

func TestZoneManagerIDNotFound(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()

	id := zm.ID("nonexistent")
	if id != 0 {
		t.Errorf("ID(nonexistent) = %d, want 0", id)
	}
}

func TestZoneManagerAcquireRelease(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()
	id := zm.GetOrCreate("zone1")

	zm.acquire(id)
	zm.acquire(id)
	zm.acquire(id)

	zm.release(id)
	zm.release(id)
	// active[id] should now be 1
}

func TestZoneManagerReset(t *testing.T) {
	t.Parallel()

	zm := newZoneManager()
	id := zm.GetOrCreate("zone1")
	zm.acquire(id)
	zm.acquire(id)

	zm.Reset()
	// After reset, active map should be empty (no panic on further ops)
	zm.acquire(id)
}

func TestMark(t *testing.T) {
	ResetZones()

	result := Mark("test-mark", "Hello")
	if result == "" {
		t.Error("Mark should return non-empty string")
	}
	// Should contain zone start marker, content, and zone end marker
	if !contains(result, "Hello") {
		t.Errorf("Mark result should contain view content: %q", result)
	}
}

func TestIsZoneAt(t *testing.T) {
	ResetZones()

	buf := NewCellBuf(20, 3)

	zoneID := globalZones.GetOrCreate("click-zone")
	cell := Cell{Content: "X", Width: 1, ZoneID: zoneID}
	buf.SetCell(5, 1, cell)

	if !IsZoneAt(buf, 5, 1, "click-zone") {
		t.Error("IsZoneAt should find zone at (5,1)")
	}

	if IsZoneAt(buf, 0, 0, "click-zone") {
		t.Error("IsZoneAt should not find zone at (0,0)")
	}
}

func TestIsZoneAtUnknownName(t *testing.T) {
	buf := NewCellBuf(20, 3)
	if IsZoneAt(buf, 0, 0, "nonexistent") {
		t.Error("IsZoneAt should return false for unknown zone name")
	}
}

func TestZoneAt(t *testing.T) {
	ResetZones()

	buf := NewCellBuf(20, 3)

	zoneID := globalZones.GetOrCreate("my-zone")
	cell := Cell{Content: "X", Width: 1, ZoneID: zoneID}
	buf.SetCell(0, 0, cell)

	name := ZoneAt(buf, 0, 0)
	if name != "my-zone" {
		t.Errorf("ZoneAt(0,0) = %q, want %q", name, "my-zone")
	}

	// Empty zone
	name = ZoneAt(buf, 1, 0)
	if name != "" {
		t.Errorf("ZoneAt(1,0) = %q, want empty", name)
	}
}

func TestZoneNames(t *testing.T) {
	ResetZones()
	globalZones.GetOrCreate("alpha")
	globalZones.GetOrCreate("beta")

	names := ZoneNames()
	if len(names) < 2 {
		t.Errorf("ZoneNames() = %v, expected at least 2", names)
	}
}

func TestGetZoneIDAndName(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("test-zone-unique")

	name := globalZones.Name(id)
	if name != "test-zone-unique" {
		t.Errorf("Name(%d) = %q, want %q", id, name, "test-zone-unique")
	}
}

func TestGetZoneNameFunc(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("named-zone")

	name := GetZoneName(id)
	if name != "named-zone" {
		t.Errorf("GetZoneName(%d) = %q, want %q", id, name, "named-zone")
	}
}

func TestGetZoneIDFunc(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("id-zone")

	found := GetZoneID("id-zone")
	if found != id {
		t.Errorf("GetZoneID(%q) = %d, want %d", "id-zone", found, id)
	}
}

func TestCurrentBufFunc(t *testing.T) {
	buf := NewCellBuf(10, 5)
	SetCurrentBuf(buf)

	got := CurrentBuf()
	if got != buf {
		t.Error("CurrentBuf should return the buffer set by SetCurrentBuf")
	}
}

func TestMarkFormat(t *testing.T) {
	ResetZones()

	result := Mark("fmt-test", "XY")
	if len(result) == 0 {
		t.Fatal("Mark should return non-empty string")
	}

	if result[0] != '\x1b' {
		t.Error("Mark should start with ESC")
	}
}

func TestMarkMultiline(t *testing.T) {
	ResetZones()

	// Multi-line Mark should wrap each line individually so zone markers
	// don't leak into tree prefixes prepended to intermediate lines.
	view := "line1\nline2\nline3"
	result := Mark("ml-test", view)

	id := globalZones.GetOrCreate("ml-test")
	start := "\x1b[" + strconv.Itoa(int(id)) + "z"
	end := "\x1b[/" + strconv.Itoa(int(id)) + "z"

	expected := start + "line1" + end + "\n" + start + "line2" + end + "\n" + start + "line3" + end
	if result != expected {
		t.Errorf("Mark multiline = %q, want %q", result, expected)
	}
}

func TestMarkMultilineNoZoneLeakWithPrefix(t *testing.T) {
	ResetZones()

	// Simulate tree rendering: viewport zone at column 3 with
	// tree prefix "│  " prepended to each line.
	buf := NewCellBuf(30, 3)
	view := Mark("vp", "AAAA\nBBBB\nCCCC")

	// Tree renderer adds prefix before each line
	lines := strings.Split(view, "\n")
	prefixed := "│  " + lines[0] + "\n│  " + lines[1] + "\n│  " + lines[2]
	buf.WriteANSIString(0, 0, prefixed)

	// Prefix columns (0-2) should NOT have the zone ID
	for x := range 3 {
		if IsZoneAt(buf, x, 1, "vp") {
			t.Errorf("prefix at (%d,1) should not be in zone 'vp'", x)
		}
	}

	// Content columns (3-6) SHOULD have the zone ID
	for x := 3; x <= 6; x++ {
		if !IsZoneAt(buf, x, 1, "vp") {
			t.Errorf("content at (%d,1) should be in zone 'vp'", x)
		}
	}

	// Right of content (7+) should NOT have the zone ID
	if IsZoneAt(buf, 7, 1, "vp") {
		t.Errorf("cell at (7,1) should not be in zone 'vp'")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
