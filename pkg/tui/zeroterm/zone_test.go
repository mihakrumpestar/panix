package zeroterm

import (
	"strconv"
	"strings"
	"testing"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerGetOrCreate(t *testing.T) {
	mgr := newZoneManager()

	id1 := mgr.GetOrCreate("zone-a")

	id2 := mgr.GetOrCreate("zone-a")
	if id1 != id2 {
		t.Errorf("same name should return same ID: %d != %d", id1, id2)
	}

	id3 := mgr.GetOrCreate("zone-b")
	if id3 == id1 {
		t.Error("different names should return different IDs")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerName(t *testing.T) {
	mgr := newZoneManager()
	zoneID := mgr.GetOrCreate("my-zone")

	name := mgr.Name(zoneID)
	if name != "my-zone" {
		t.Errorf("Name(%d) = %q, want %q", zoneID, name, "my-zone")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerNameInvalidID(t *testing.T) {
	mgr := newZoneManager()

	name := mgr.Name(999)
	if name != "" {
		t.Errorf("Name(999) = %q, want empty string", name)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerID(t *testing.T) {
	mgr := newZoneManager()
	zoneID := mgr.GetOrCreate("test")

	found := mgr.ID("test")
	if found != zoneID {
		t.Errorf("ID(test) = %d, want %d", found, zoneID)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerIDNotFound(t *testing.T) {
	mgr := newZoneManager()

	zoneID := mgr.ID("nonexistent")
	if zoneID != 0 {
		t.Errorf("ID(nonexistent) = %d, want 0", zoneID)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerAcquireRelease(t *testing.T) {
	mgr := newZoneManager()
	zoneID := mgr.GetOrCreate("zone1")

	mgr.acquire(zoneID)
	mgr.acquire(zoneID)
	mgr.acquire(zoneID)

	mgr.release(zoneID)
	mgr.release(zoneID)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneManagerReset(t *testing.T) {
	mgr := newZoneManager()
	zoneID := mgr.GetOrCreate("zone1")
	mgr.acquire(zoneID)
	mgr.acquire(zoneID)

	mgr.Reset()
	mgr.acquire(zoneID)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMark(t *testing.T) {
	ResetZones()

	result := Mark("test-mark", "Hello")
	if result == "" {
		t.Error("Mark should return non-empty string")
	}

	if !contains(result, "Hello") {
		t.Errorf("Mark result should contain view content: %q", result)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneNames(t *testing.T) {
	ResetZones()
	globalZones.GetOrCreate("alpha")
	globalZones.GetOrCreate("beta")

	names := ZoneNames()
	if len(names) < 2 {
		t.Errorf("ZoneNames() = %v, expected at least 2", names)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestGetZoneIDAndName(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("test-zone-unique")

	name := globalZones.Name(id)
	if name != "test-zone-unique" {
		t.Errorf("Name(%d) = %q, want %q", id, name, "test-zone-unique")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestGetZoneNameFunc(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("named-zone")

	name := GetZoneName(id)
	if name != "named-zone" {
		t.Errorf("GetZoneName(%d) = %q, want %q", id, name, "named-zone")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestGetZoneIDFunc(t *testing.T) {
	ResetZones()

	id := globalZones.GetOrCreate("id-zone")

	found := GetZoneID("id-zone")
	if found != id {
		t.Errorf("GetZoneID(%q) = %d, want %d", "id-zone", found, id)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
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

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkMultiline(t *testing.T) {
	ResetZones()

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

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkMultilineNoZoneLeakWithPrefix(t *testing.T) {
	ResetZones()

	view := Mark("vp", "AAAA\nBBBB\nCCCC")

	lines := strings.Split(view, "\n")
	prefixed := "│  " + lines[0] + "\n│  " + lines[1] + "\n│  " + lines[2]

	prefixedLines := strings.Split(prefixed, "\n")
	for i, line := range prefixedLines {
		prefix := line[:3]
		if IsZoneAtLine(prefix, 0, "vp") {
			t.Errorf("prefix at line %d col 0 should not be in zone 'vp'", i)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestIsZoneAtLine(t *testing.T) {
	ResetZones()

	line := Mark("click-zone", "XY")
	if !IsZoneAtLine(line, 0, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 0")
	}

	if !IsZoneAtLine(line, 1, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 1")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestIsZoneAtLineUnknownName(t *testing.T) {
	if IsZoneAtLine("some text", 0, "nonexistent") {
		t.Error("IsZoneAtLine should return false for unknown zone name")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneAtLine(t *testing.T) {
	ResetZones()

	line := Mark("my-zone", "AB")

	name := ZoneAtLine(line, 0)
	if name != "my-zone" {
		t.Errorf("ZoneAtLine(0) = %q, want %q", name, "my-zone")
	}

	afterZone := line[strings.LastIndex(line, "z")+1:]
	if afterZone != "" {
		name = ZoneAtLine(afterZone, 0)
		if name != "" {
			t.Errorf("ZoneAtLine outside zone = %q, want empty", name)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestCurrentLines(t *testing.T) {
	lines := []string{"line1", "line2"}
	SetCurrentLines(lines)

	got := CurrentLines()
	if len(got) != 2 || got[0] != "line1" || got[1] != "line2" {
		t.Errorf("CurrentLines() = %v, want %v", got, lines)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneAtLineOutside(t *testing.T) {
	ResetZones()

	// Mark a short zone, pad it to simulate centering in a wider column.
	marked := Mark("padded", "AB")
	padded := "   " + marked + "   "

	// Zone should be active only at cols 3-4 (the "AB" content)
	if name := ZoneAtLine(padded, 3); name != "padded" {
		t.Errorf("ZoneAtLine(3) = %q, want %q", name, "padded")
	}

	if name := ZoneAtLine(padded, 0); name != "" {
		t.Errorf("ZoneAtLine(0) = %q, want empty (before zone)", name)
	}

	if name := ZoneAtLine(padded, 6); name != "" {
		t.Errorf("ZoneAtLine(6) = %q, want empty (after zone)", name)
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
