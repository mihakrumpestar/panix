package zeroterm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func TestNewZoneID(t *testing.T) {
	t.Parallel()

	_ = NewZoneID()
}

func TestZoneID_MarkBuf(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("Hello"), buf)

	if buf.Len() != 1 {
		t.Fatalf("MarkBuf: got %d lines, want 1", buf.Len())
	}

	line := buf.Line(0)
	if !bytes.Contains(line, []byte("Hello")) {
		t.Errorf("MarkBuf line should contain 'Hello': %q", line)
	}

	open := id.FormatOpen(nil)
	closeMarker := id.FormatClose(nil)

	if !bytes.HasPrefix(line, open) {
		t.Errorf("MarkBuf line should start with open marker: %q", line)
	}

	if !bytes.HasSuffix(line, closeMarker) {
		t.Errorf("MarkBuf line should end with close marker: %q", line)
	}
}

func TestZoneID_MarkBufMultiline(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("line1\nline2\nline3"), buf)

	if buf.Len() != 3 {
		t.Fatalf("MarkBuf multiline: got %d lines, want 3", buf.Len())
	}

	for i, want := range []string{"line1", "line2", "line3"} {
		if !bytes.Contains(buf.Line(i), []byte(want)) {
			t.Errorf("line %d should contain %q: %q", i, want, buf.Line(i))
		}
	}
}

func TestZoneID_MarkBufEmpty(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf(nil, buf)

	if buf.Len() != 1 {
		t.Fatalf("MarkBuf empty: got %d lines, want 1", buf.Len())
	}
}

func TestZoneID_MarkBufMultilineExact(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("line1\nline2\nline3"), buf)

	if buf.Len() != 3 {
		t.Fatalf("got %d lines, want 3", buf.Len())
	}

	open := string(id.FormatOpen(nil))
	closeMarker := string(id.FormatClose(nil))

	for i, want := range []string{"line1", "line2", "line3"} {
		expected := open + want + closeMarker
		if string(buf.Line(i)) != expected {
			t.Errorf("line %d = %q, want %q", i, buf.Line(i), expected)
		}
	}
}

func TestZoneID_MarkBufNoZoneLeakWithPrefix(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("AAAA\nBBBB\nCCCC"), buf)

	lines := make([]string, buf.Len())
	for i := range buf.Len() {
		lines[i] = string(buf.Line(i))
	}

	prefixed := "│  " + lines[0] + "\n│  " + lines[1] + "\n│  " + lines[2]

	prefixedLines := strings.Split(prefixed, "\n")
	for i, line := range prefixedLines {
		prefix := line[:3]

		found, ok := ZoneIDAtCol([]byte(prefix), 0)
		if ok && found.Equal(id) {
			t.Errorf("prefix at line %d col 0 should not be in zone", i)
		}
	}
}

func TestZoneIDAtCol(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	lb := buffer.NewLinesBuf()
	defer lb.Release()

	id.MarkBuf([]byte("XY"), lb)

	line := lb.Line(0)

	found, ok := ZoneIDAtCol(line, 0)
	if !ok || !found.Equal(id) {
		t.Errorf("ZoneIDAtCol col 0 = (%v, %v), want match", found, ok)
	}

	found, ok = ZoneIDAtCol(line, 1)
	if !ok || !found.Equal(id) {
		t.Errorf("ZoneIDAtCol col 1 = (%v, %v), want match", found, ok)
	}
}

func TestZoneIDAtColNoZone(t *testing.T) {
	t.Parallel()

	_, ok := ZoneIDAtCol([]byte("no zone here"), 0)
	if ok {
		t.Error("ZoneIDAtCol should return false for no zone")
	}
}

func TestZoneLifecycleAcrossFrames(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	frame1Buf := buffer.NewLinesBufDiff()
	buf1 := buffer.NewLinesBuf()
	id.MarkBuf([]byte("line1"), buf1)
	frame1Buf.Write(buf1.Line(0))
	buf1.Reset()
	id.MarkBuf([]byte("line2"), buf1)
	frame1Buf.Write(buf1.Line(0))
	buf1.Reset()
	id.MarkBuf([]byte("line3"), buf1)
	frame1Buf.Write(buf1.Line(0))
	buf1.Release()

	found, ok := ZoneIDAtCol(frame1Buf.Line(0), 2)
	if !ok || !found.Equal(id) {
		t.Errorf("frame1 zone at col 2 should match id")
	}

	frame2Buf := buffer.NewLinesBufDiff()
	buf2 := buffer.NewLinesBuf()
	id.MarkBuf([]byte("lineA"), buf2)
	frame2Buf.Write(buf2.Line(0))
	buf2.Reset()
	id.MarkBuf([]byte("lineB"), buf2)
	frame2Buf.Write(buf2.Line(0))
	buf2.Release()

	found, ok = ZoneIDAtCol(frame2Buf.Line(0), 3)
	if !ok || !found.Equal(id) {
		t.Errorf("frame2 zone at col 3 should match id")
	}
}

func TestZoneID_MarkLines(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	src := buffer.NewLinesBuf()
	defer src.Release()

	src.WriteLine([]byte("A"))
	src.WriteLine([]byte("B"))

	dst := buffer.NewLinesBuf()
	defer dst.Release()

	id.MarkLines(src, dst)

	if dst.Len() != 2 {
		t.Fatalf("MarkLines: got %d lines, want 2", dst.Len())
	}
}

func TestZoneID_MarkLinesEmpty(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	src := buffer.NewLinesBuf()
	defer src.Release()

	dst := buffer.NewLinesBuf()
	defer dst.Release()

	id.MarkLines(src, dst)

	if dst.Len() != 1 {
		t.Fatalf("MarkLines empty: got %d lines, want 1", dst.Len())
	}
}

func TestZoneID_FormatOpenClose(t *testing.T) {
	t.Parallel()

	id := zoneIDFromDigits([]byte("42"))
	open := id.FormatOpen(nil)
	closeMarker := id.FormatClose(nil)

	if string(open) != "\x1b[42z" {
		t.Errorf("FormatOpen(42) = %q, want \\x1b[42z", open)
	}

	if string(closeMarker) != "\x1b[/42z" {
		t.Errorf("FormatClose(42) = %q, want \\x1b[/42z", closeMarker)
	}
}
