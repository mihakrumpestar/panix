package zeroterm

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
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
	result := Mark("test-mark", "Hello")
	if result == "" {
		t.Error("Mark should return non-empty string")
	}

	if !contains(result, "Hello") {
		t.Errorf("Mark result should contain view content: %q", result)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkFormat(t *testing.T) {
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
	view := Mark("vp", "AAAA\nBBBB\nCCCC")

	lines := strings.Split(view, "\n")
	prefixed := "│  " + lines[0] + "\n│  " + lines[1] + "\n│  " + lines[2]

	prefixedLines := strings.Split(prefixed, "\n")
	for i, line := range prefixedLines {
		prefix := line[:3]
		if IsZoneAtLine([]byte(prefix), 0, "vp") {
			t.Errorf("prefix at line %d col 0 should not be in zone 'vp'", i)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestIsZoneAtLine(t *testing.T) {
	line := Mark("click-zone", "XY")
	if !IsZoneAtLine([]byte(line), 0, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 0")
	}

	if !IsZoneAtLine([]byte(line), 1, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 1")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneLifecycleAcrossFrames(t *testing.T) {
	// Create zone content using Mark (which creates and formats properly)
	frame1Line1 := Mark("test-zone", "line1")
	frame1Line2 := Mark("test-zone", "line2")
	frame1Line3 := Mark("test-zone", "line3")

	// Frame 1: 3 lines with zone
	frame1Buf := linesbuffer.NewPooled()
	frame1Buf.Write([]byte(frame1Line1))
	frame1Buf.Write([]byte(frame1Line2))
	frame1Buf.Write([]byte(frame1Line3))
	SetCurrentLines(frame1Buf)

	// Verify zone found in frame1 (col 2 is within "line1")
	if !IsZoneAtLine(frame1Buf.Line(0), 2, "test-zone") {
		t.Errorf("frame1 zone at col 2 should be test-zone (line content: %q)", frame1Line1)
	}

	// Frame 2: Only 2 lines (shrink)
	frame2Line1 := Mark("test-zone", "lineA")
	frame2Line2 := Mark("test-zone", "lineB")
	frame2Buf := linesbuffer.New()
	frame2Buf.Write([]byte(frame2Line1))
	frame2Buf.Write([]byte(frame2Line2))
	SetCurrentLines(frame2Buf)

	// Verify zone found in frame2 (col 3 is within "lineA")
	if !IsZoneAtLine(frame2Buf.Line(0), 3, "test-zone") {
		t.Errorf("frame2 zone at col 3 should be test-zone (line content: %q)", frame2Line1)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneIDOverflow(t *testing.T) {
	mgr := newZoneManager()

	// Simulate many zones (in real usage, this would be 65535 max)
	// For test, just verify sequential IDs work
	for idx := range 100 {
		name := "zone" + strconv.Itoa(idx)
		id := mgr.GetOrCreate(name)

		// IDs should be sequential starting from 1
		if int(id) != idx+1 {
			t.Errorf("zone %s got ID %d, want %d", name, id, idx+1)
		}
	}

	// Verify reuse returns same ID
	id := mgr.GetOrCreate("zone50")
	if id != 51 { // zone50 was the 51st zone (index 50 in loop)
		t.Errorf("reused zone50 got ID %d, want 51", id)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestIsZoneAtLineUnknownName(t *testing.T) {
	if IsZoneAtLine([]byte("some text"), 0, "nonexistent") {
		t.Error("IsZoneAtLine should return false for unknown zone name")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestCurrentLines(t *testing.T) {
	buf := linesbuffer.NewPooled()
	buf.Write([]byte("line1"))
	buf.Write([]byte("line2"))
	SetCurrentLines(buf)

	got := CurrentLines()
	if got.Len() != 2 || string(got.Line(0)) != "line1" || string(got.Line(1)) != "line2" {
		t.Errorf("CurrentLines() = %v, want %v", got, buf)
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
