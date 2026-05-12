package zeroterm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestEnsureZone(t *testing.T) {
	id1 := EnsureZone("zone-a")

	id2 := EnsureZone("zone-a")
	if id1 != id2 {
		t.Errorf("same name should return same ID: %d != %d", id1, id2)
	}

	id3 := EnsureZone("zone-b")
	if id3 == id1 {
		t.Error("different names should return different IDs")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneID(t *testing.T) {
	zoneID := EnsureZone("test")

	found := ZoneID("test")
	if found != zoneID {
		t.Errorf("ZoneID(test) = %d, want %d", found, zoneID)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneIDNotFound(t *testing.T) {
	if ZoneID("nonexistent") != 0 {
		t.Error("ZoneID(nonexistent) = should be 0")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkBufByID(t *testing.T) {
	id := EnsureZone("markbuf-test")

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(id, []byte("Hello"), lb)

	if lb.Len() != 1 {
		t.Fatalf("MarkBufByID: got %d lines, want 1", lb.Len())
	}

	line := lb.Line(0)
	if !bytes.Contains(line, []byte("Hello")) {
		t.Errorf("MarkBufByID line should contain 'Hello': %q", line)
	}

	var openBuf, closeBuf [16]byte

	open := FormatZoneOpen(openBuf[:0], id)
	close := FormatZoneClose(closeBuf[:0], id)

	if !bytes.HasPrefix(line, open) {
		t.Errorf("MarkBufByID line should start with open marker: %q", line)
	}

	if !bytes.HasSuffix(line, close) {
		t.Errorf("MarkBufByID line should end with close marker: %q", line)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkBufByIDMultiline(t *testing.T) {
	id := EnsureZone("markbuf-ml")

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(id, []byte("line1\nline2\nline3"), lb)

	if lb.Len() != 3 {
		t.Fatalf("MarkBufByID multiline: got %d lines, want 3", lb.Len())
	}

	for i, want := range []string{"line1", "line2", "line3"} {
		if !bytes.Contains(lb.Line(i), []byte(want)) {
			t.Errorf("line %d should contain %q: %q", i, want, lb.Line(i))
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkBufByIDEmpty(t *testing.T) {
	id := EnsureZone("markbuf-empty")

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(id, nil, lb)

	if lb.Len() != 1 {
		t.Fatalf("MarkBufByID empty: got %d lines, want 1", lb.Len())
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkBufByIDMultilineExact(t *testing.T) {
	id := EnsureZone("markbuf-exact")

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(id, []byte("line1\nline2\nline3"), lb)

	if lb.Len() != 3 {
		t.Fatalf("got %d lines, want 3", lb.Len())
	}

	var openBuf, closeBuf [16]byte

	open := string(FormatZoneOpen(openBuf[:0], id))
	close := string(FormatZoneClose(closeBuf[:0], id))

	for i, want := range []string{"line1", "line2", "line3"} {
		expected := open + want + close
		if string(lb.Line(i)) != expected {
			t.Errorf("line %d = %q, want %q", i, lb.Line(i), expected)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkBufByIDNoZoneLeakWithPrefix(t *testing.T) {
	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(EnsureZone("vp"), []byte("AAAA\nBBBB\nCCCC"), lb)

	lines := make([]string, lb.Len())
	for i := range lb.Len() {
		lines[i] = string(lb.Line(i))
	}

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
	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(EnsureZone("click-zone"), []byte("XY"), lb)

	line := lb.Line(0)
	if !IsZoneAtLine(line, 0, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 0")
	}

	if !IsZoneAtLine(line, 1, "click-zone") {
		t.Error("IsZoneAtLine should find zone at col 1")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneIDAtCol(t *testing.T) {
	id := EnsureZone("col-test")

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	MarkBufByID(id, []byte("AB"), lb)

	if ZoneIDAtCol(lb.Line(0), 0) != id {
		t.Error("ZoneIDAtCol should find zone at col 0")
	}

	if ZoneIDAtCol(lb.Line(0), 1) != id {
		t.Error("ZoneIDAtCol should find zone at col 1")
	}

	if ZoneIDAtCol([]byte("no zone here"), 0) != 0 {
		t.Error("ZoneIDAtCol should return 0 for no zone")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneLifecycleAcrossFrames(t *testing.T) {
	id := EnsureZone("test-zone")

	frame1Buf := buffer.NewLinesBufDiff()
	f1 := buffer.NewLinesBuf()
	MarkBufByID(id, []byte("line1"), f1)
	frame1Buf.Write(f1.Line(0))
	f1.Reset()
	MarkBufByID(id, []byte("line2"), f1)
	frame1Buf.Write(f1.Line(0))
	f1.Reset()
	MarkBufByID(id, []byte("line3"), f1)
	frame1Buf.Write(f1.Line(0))
	f1.Release()

	SetCurrentLines(frame1Buf)

	if !IsZoneAtLine(frame1Buf.Line(0), 2, "test-zone") {
		t.Errorf("frame1 zone at col 2 should be test-zone")
	}

	frame2Buf := buffer.NewLinesBufDiff()
	f2 := buffer.NewLinesBuf()
	MarkBufByID(id, []byte("lineA"), f2)
	frame2Buf.Write(f2.Line(0))
	f2.Reset()
	MarkBufByID(id, []byte("lineB"), f2)
	frame2Buf.Write(f2.Line(0))
	f2.Release()

	SetCurrentLines(frame2Buf)

	if !IsZoneAtLine(frame2Buf.Line(0), 3, "test-zone") {
		t.Errorf("frame2 zone at col 3 should be test-zone")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestZoneIDSequential(t *testing.T) {
	first := EnsureZone("seq-0")

	second := EnsureZone("seq-1")
	if second != first+1 {
		t.Errorf("IDs should be sequential: %d, %d", first, second)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestIsZoneAtLineUnknownName(t *testing.T) {
	if IsZoneAtLine([]byte("some text"), 0, "definitely-nonexistent-zone-xyz") {
		t.Error("IsZoneAtLine should return false for unknown zone name")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkLinesByID(t *testing.T) {
	id := EnsureZone("marklines-test")

	src := buffer.NewLinesBuf()
	defer src.Release()

	src.WriteLine([]byte("A"))
	src.WriteLine([]byte("B"))

	dst := buffer.NewLinesBuf()
	defer dst.Release()

	MarkLinesByID(id, src, dst)

	if dst.Len() != 2 {
		t.Fatalf("MarkLinesByID: got %d lines, want 2", dst.Len())
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMarkLinesByIDEmpty(t *testing.T) {
	id := EnsureZone("marklines-empty")

	src := buffer.NewLinesBuf()
	defer src.Release()

	dst := buffer.NewLinesBuf()
	defer dst.Release()

	MarkLinesByID(id, src, dst)

	if dst.Len() != 1 {
		t.Fatalf("MarkLinesByID empty: got %d lines, want 1", dst.Len())
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestFormatZoneOpenClose(t *testing.T) {
	var openBuf, closeBuf [16]byte

	open := FormatZoneOpen(openBuf[:0], 42)
	close := FormatZoneClose(closeBuf[:0], 42)

	if string(open) != "\x1b[42z" {
		t.Errorf("FormatZoneOpen(42) = %q, want \\x1b[42z", open)
	}

	if string(close) != "\x1b[/42z" {
		t.Errorf("FormatZoneClose(42) = %q, want \\x1b[/42z", close)
	}
}

func TestCurrentLines(t *testing.T) {
	buf := buffer.NewLinesBufDiff()
	buf.Write([]byte("line1"))
	buf.Write([]byte("line2"))
	SetCurrentLines(buf)

	got := CurrentLines()
	if got.Len() != 2 || string(got.Line(0)) != "line1" || string(got.Line(1)) != "line2" {
		t.Errorf("CurrentLines() = %v, want %v", got, buf)
	}
}
