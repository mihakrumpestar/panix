package buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimFrontBasic(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))
	buf.WriteLine([]byte("b"))
	buf.WriteLine([]byte("c"))
	buf.WriteLine([]byte("d"))
	buf.WriteLine([]byte("e"))

	buf.TrimFront(2)

	assert.Equal(t, 3, buf.Len(), "should have 3 lines after trimming 2 from 5")
	assert.Equal(t, "c", string(buf.Line(0)))
	assert.Equal(t, "d", string(buf.Line(1)))
	assert.Equal(t, "e", string(buf.Line(2)))
}

func TestTrimFrontOneLine(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("first"))
	buf.WriteLine([]byte("second"))

	buf.TrimFront(1)

	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, "second", string(buf.Line(0)))
}

func TestTrimFrontAll(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))
	buf.WriteLine([]byte("b"))
	buf.WriteLine([]byte("c"))

	buf.TrimFront(3)

	assert.Equal(t, 0, buf.Len(), "should be empty after trimming all")
}

func TestTrimFrontMoreThanLen(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))
	buf.WriteLine([]byte("b"))

	buf.TrimFront(10)

	assert.Equal(t, 0, buf.Len(), "should be empty when trimming more than len")
}

func TestTrimFrontZero(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))
	buf.WriteLine([]byte("b"))

	buf.TrimFront(0)

	assert.Equal(t, 2, buf.Len(), "trim 0 should be a no-op")
	assert.Equal(t, "a", string(buf.Line(0)))
	assert.Equal(t, "b", string(buf.Line(1)))
}

func TestTrimFrontNegative(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))

	buf.TrimFront(-1)

	assert.Equal(t, 1, buf.Len(), "negative trim should be a no-op")
}

func TestTrimFrontEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.TrimFront(5)

	assert.Equal(t, 0, buf.Len())
}

func TestTrimFrontAppendAfter(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("a"))
	buf.WriteLine([]byte("b"))
	buf.WriteLine([]byte("c"))

	buf.TrimFront(1)
	buf.WriteLine([]byte("d"))

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "b", string(buf.Line(0)))
	assert.Equal(t, "c", string(buf.Line(1)))
	assert.Equal(t, "d", string(buf.Line(2)))
}

func TestTrimFrontByteContent(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteLine([]byte("hello"))
	buf.WriteLine([]byte("world"))

	buf.TrimFront(1)

	// Verify buf is compacted (no leftover bytes from trimmed line)
	assert.Equal(t, "world", string(buf.Line(0)))
	assert.Len(t, buf.buf, len("world"), "buf should only contain remaining line bytes")
}

func TestTrimFrontWithANSI(t *testing.T) {
	t.Parallel()

	line1 := []byte("\x1b[31mred\x1b[0m")
	line2 := []byte("\x1b[32mgreen\x1b[0m")
	line3 := []byte("\x1b[33myellow\x1b[0m")

	buf := NewLinesBuf()
	buf.WriteLine(line1)
	buf.WriteLine(line2)
	buf.WriteLine(line3)

	buf.TrimFront(1)

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, string(line2), string(buf.Line(0)))
	assert.Equal(t, string(line3), string(buf.Line(1)))
}

func TestLineOffsetDefault(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	assert.Zero(t, buf.LineOffset(), "new LinesBuf should have lineOffset 0")
}

func TestLineOffsetCopyFrom(t *testing.T) {
	t.Parallel()

	src := NewLinesBuf()
	src.WriteLine([]byte("data"))
	src.lineOffset = 42

	dst := NewLinesBuf()
	dst.CopyFrom(src)

	assert.Equal(t, uint64(42), dst.LineOffset(), "CopyFrom should carry lineOffset")
}

func TestLineOffsetReset(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.lineOffset = 100
	buf.Reset()

	assert.Zero(t, buf.LineOffset(), "Reset should zero lineOffset")
}

// setLineCase describes one SetLine expectation.
type setLineCase struct {
	name    string
	lines   []string
	index   int
	replace string
	want    []string
}

// runSetLineCases builds a LinesBuf from the case lines, replaces one line and
// asserts the resulting lines.
func runSetLineCases(t *testing.T, cases []setLineCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := NewLinesBuf()

			for _, line := range tt.lines {
				buf.WriteString(line)
			}

			buf.SetLine(tt.index, []byte(tt.replace))

			assert.Equal(t, tt.want, linesAsStrings(buf), "lines after SetLine")
		})
	}
}

func TestLinesBufSetLine(t *testing.T) {
	t.Parallel()

	runSetLineCases(t, []setLineCase{
		{
			name:    "replace with equal length",
			lines:   []string{"a", "bb", "ccc"},
			index:   1,
			replace: "xx",
			want:    []string{"a", "xx", "ccc"},
		},
		{
			name:    "replace with longer line",
			lines:   []string{"a", "bb", "ccc"},
			index:   1,
			replace: "four",
			want:    []string{"a", "four", "ccc"},
		},
		{
			name:    "replace with shorter line",
			lines:   []string{"a", "bbbb", "ccc"},
			index:   1,
			replace: "x",
			want:    []string{"a", "x", "ccc"},
		},
		{
			name:    "replace first line",
			lines:   []string{"aa", "bb", "cc"},
			index:   0,
			replace: "zzz",
			want:    []string{"zzz", "bb", "cc"},
		},
		{
			name:    "replace last line",
			lines:   []string{"aa", "bb", "cc"},
			index:   2,
			replace: "z",
			want:    []string{"aa", "bb", "z"},
		},
		{
			name:    "replace with empty line",
			lines:   []string{"aa", "bb", "cc"},
			index:   1,
			replace: "",
			want:    []string{"aa", "", "cc"},
		},
	})
}

func TestLinesBufSetLineCopiesInput(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteString("aa")
	buf.WriteString("bb")

	line := []byte("XY")
	buf.SetLine(1, line)
	line[0] = 'Z'

	assert.Equal(t, "XY", string(buf.Line(1)), "SetLine must copy the input, not alias it")

	buf.SetLine(0, []byte("QQ"))

	assert.Equal(t, "QQ", string(buf.Line(0)))
	assert.Equal(t, "XY", string(buf.Line(1)), "other lines must stay intact")
}

func TestLinesBufSetLineOutOfBoundsPanics(t *testing.T) {
	t.Parallel()

	buf := NewLinesBuf()
	buf.WriteString("aa")

	assert.Panics(t, func() { buf.SetLine(1, []byte("x")) }, "out of range must panic like Line")
	assert.Panics(t, func() { buf.SetLine(-1, []byte("x")) }, "negative index must panic like Line")
}

func TestLinesBufDiffSetLine(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufDiff()
	buf.WriteString("aa")

	buf.SetLine(5, []byte("x"))
	buf.SetLine(-1, []byte("x"))

	assert.Equal(t, []string{"aa"}, linesAsStrings(buf.LinesBuf), "out of bounds must be a no-op")

	buf.SetLine(0, []byte("bb"))

	assert.Equal(t, []string{"bb"}, linesAsStrings(buf.LinesBuf), "in bounds must replace")
}

// linesAsStrings returns all lines of buf as strings.
func linesAsStrings(buf *LinesBuf) []string {
	out := make([]string, buf.Len())

	for i := range buf.Len() {
		out[i] = string(buf.Line(i))
	}

	return out
}
