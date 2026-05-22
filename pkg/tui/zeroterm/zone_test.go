package zeroterm

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assert.Equal(t, 1, buf.Len(), "MarkBuf: expected 1 line")

	lineContent := string(buf.Line(0))
	assert.Contains(t, lineContent, "Hello", "MarkBuf line should contain 'Hello': %q", lineContent)

	line := buf.Line(0)
	assert.True(t, bytes.Contains(line, []byte("Hello")), "MarkBuf line should contain 'Hello': %q", line)

	open := id.FormatOpen(nil)
	closeMarker := id.FormatClose(nil)

	assert.True(t, bytes.HasPrefix(line, open), "MarkBuf line should start with open marker: %q", line)

	assert.True(t, bytes.HasSuffix(line, closeMarker), "MarkBuf line should end with close marker: %q", line)
}

func TestZoneID_MarkBufMultiline(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("line1\nline2\nline3"), buf)

	require.Equal(t, 3, buf.Len(), "MarkBuf multiline: got %d lines, want 3")

	for i, want := range []string{"line1", "line2", "line3"} {
		assert.True(t, bytes.Contains(buf.Line(i), []byte(want)), "line %d should contain %q: %q", i, want, buf.Line(i))
	}
}

func TestZoneID_MarkBufEmpty(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf(nil, buf)

	require.Equal(t, 1, buf.Len(), "MarkBuf empty: got %d lines, want 1")
}

func TestZoneID_MarkBufMultilineExact(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	buf := buffer.NewLinesBuf()
	defer buf.Release()

	id.MarkBuf([]byte("line1\nline2\nline3"), buf)

	require.Equal(t, 3, buf.Len(), "got %d lines, want 3")

	open := string(id.FormatOpen(nil))
	closeMarker := string(id.FormatClose(nil))

	for i, want := range []string{"line1", "line2", "line3"} {
		expected := open + want + closeMarker
		assert.Equal(t, expected, string(buf.Line(i)), "line %d = %q, want %q", i, buf.Line(i), expected)
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
		assert.False(t, ok && found.Equal(id), "prefix at line %d col 0 should not be in zone", i)
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
	assert.True(t, ok && found.Equal(id), "ZoneIDAtCol col 0 = (%v, %v), want match", found, ok)

	found, ok = ZoneIDAtCol(line, 1)
	assert.True(t, ok && found.Equal(id), "ZoneIDAtCol col 1 = (%v, %v), want match", found, ok)
}

func TestZoneIDAtColNoZone(t *testing.T) {
	t.Parallel()

	_, ok := ZoneIDAtCol([]byte("no zone here"), 0)
	assert.False(t, ok, "ZoneIDAtCol should return false for no zone")
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
	assert.True(t, ok && found.Equal(id), "frame1 zone at col 2 should match id")

	frame2Buf := buffer.NewLinesBufDiff()
	buf2 := buffer.NewLinesBuf()
	id.MarkBuf([]byte("lineA"), buf2)
	frame2Buf.Write(buf2.Line(0))
	buf2.Reset()
	id.MarkBuf([]byte("lineB"), buf2)
	frame2Buf.Write(buf2.Line(0))
	buf2.Release()

	found, ok = ZoneIDAtCol(frame2Buf.Line(0), 3)
	assert.True(t, ok && found.Equal(id), "frame2 zone at col 3 should match id")
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

	require.Equal(t, 2, dst.Len(), "MarkLines: got %d lines, want 2")
}

func TestZoneID_MarkLinesEmpty(t *testing.T) {
	t.Parallel()

	id := NewZoneID()

	src := buffer.NewLinesBuf()
	defer src.Release()

	dst := buffer.NewLinesBuf()
	defer dst.Release()

	id.MarkLines(src, dst)

	require.Equal(t, 1, dst.Len(), "MarkLines empty: got %d lines, want 1")
}

func TestZoneID_FormatOpenClose(t *testing.T) {
	t.Parallel()

	id := zoneIDFromDigits([]byte("42"))
	open := id.FormatOpen(nil)
	closeMarker := id.FormatClose(nil)

	assert.Equal(t, "\x1b[42z", string(open), "FormatOpen(42) = %q, want \\x1b[42z", open)
	assert.Equal(t, "\x1b[/42z", string(closeMarker), "FormatClose(42) = %q, want \\x1b[/42z", closeMarker)
}
