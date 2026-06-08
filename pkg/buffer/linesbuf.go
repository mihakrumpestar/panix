package buffer

import (
	"strconv"
	"sync"
)

const (
	DefaultLinesBufLen = 5
)

// LinesBuf is a pooled buffer for building output lines in a single
// contiguous byte slice. Line boundaries are recorded as start offsets.
//
// Acquire with NewLineBuf, write lines with WriteLine, read with Line.
// Call Release when done — do not retain references after Release.
type LinesBuf struct {
	buf     []byte
	indexes []int
}

var linesBufPool = sync.Pool{
	New: func() any {
		return &LinesBuf{
			buf:     make([]byte, 0, DefaultLinesBufLen*DefaultLineBufLen),
			indexes: make([]int, 0, DefaultLinesBufLen),
		}
	},
}

func NewLinesBuf() *LinesBuf {
	return linesBufPool.Get().(*LinesBuf) //nolint:forcetypeassert
}

func (b *LinesBuf) Reset() {
	b.buf = b.buf[:0]
	b.indexes = b.indexes[:0]
}

func (b *LinesBuf) Release() {
	b.Reset()
	linesBufPool.Put(b)
}

// WriteLine starts a new line and appends the given byte slices into it.
func (b *LinesBuf) WriteLine(parts ...[]byte) {
	b.indexes = append(b.indexes, len(b.buf))
	for _, p := range parts {
		b.buf = append(b.buf, p...)
	}
}

// WriteString starts a new line from a string.
func (b *LinesBuf) WriteString(line string) {
	b.WriteLine([]byte(line))
}

// WriteLines appends multiple lines.
func (b *LinesBuf) WriteLines(lines [][]byte) {
	for _, line := range lines {
		b.WriteLine(line)
	}
}

// AppendToLine appends byte slices to the current (last) line without
// starting a new one. Panics if no line has been started yet.
func (b *LinesBuf) AppendToLine(parts ...[]byte) {
	if len(b.indexes) == 0 {
		panic("AppendToLine called with no active line")
	}

	for _, p := range parts {
		b.buf = append(b.buf, p...)
	}
}

// EmptyLine appends an empty line.
func (b *LinesBuf) EmptyLine() {
	b.indexes = append(b.indexes, len(b.buf))
}

// Line returns the i-th line as a byte slice.
func (b *LinesBuf) Line(i int) []byte {
	start := b.indexes[i]

	end := len(b.buf)
	if i+1 < len(b.indexes) {
		end = b.indexes[i+1]
	}

	return b.buf[start:end]
}

// Lines reconstructs [][]byte. Allocates — prefer Line(i).
func (b *LinesBuf) Lines() [][]byte {
	length := len(b.indexes)
	if length == 0 {
		return nil
	}

	out := make([][]byte, length)
	for i := range length {
		out[i] = b.Line(i)
	}

	return out
}

// OverrideLastLine replaces the content of the last line.
// If no lines exist, the content is written as the first line.
func (b *LinesBuf) OverrideLastLine(line []byte) {
	if len(b.indexes) == 0 {
		b.WriteLine(line)

		return
	}

	startIdx := b.indexes[len(b.indexes)-1]
	b.buf = b.buf[:startIdx]
	b.buf = append(b.buf, line...)
}

// RemoveLastLine removes the last line. No-op if the buffer is empty.
func (b *LinesBuf) RemoveLastLine() {
	if len(b.indexes) == 0 {
		return
	}

	startIdx := b.indexes[len(b.indexes)-1]
	b.buf = b.buf[:startIdx]
	b.indexes = b.indexes[:len(b.indexes)-1]
}

// LastLine returns the content of the last line, or nil if empty.
func (b *LinesBuf) LastLine() []byte {
	if len(b.indexes) == 0 {
		return nil
	}

	return b.Line(len(b.indexes) - 1)
}

// Len returns the number of lines written.
func (b *LinesBuf) Len() int {
	if b == nil {
		return 0
	}

	return len(b.indexes)
}

// Append appends bytes to the current (last) line. If no lines exist yet,
// a new line is started.
func (b *LinesBuf) Append(buf []byte) {
	if b.Len() == 0 {
		b.WriteLine(buf)
	} else {
		b.AppendToLine(buf)
	}
}

// AppendFrom bulk-copies all lines from src into b. Zero allocation when
// destination has enough capacity; single allocation on growth. No temp slices.
func (b *LinesBuf) AppendFrom(src *LinesBuf) {
	if src == nil {
		return
	}

	length := len(src.indexes)
	if length == 0 {
		return
	}

	baseIdx := len(b.buf)
	b.buf = append(b.buf, src.buf...)

	oldLen := len(b.indexes)
	newLen := oldLen + length

	if newLen <= cap(b.indexes) {
		b.indexes = b.indexes[:newLen]
	} else {
		newCap := cap(b.indexes) * 2
		for newCap < newLen {
			newCap *= 2
		}

		grown := make([]int, newLen, newCap)
		copy(grown, b.indexes[:oldLen])
		b.indexes = grown
	}

	dst := b.indexes[oldLen:]
	for i, idx := range src.indexes {
		dst[i] = baseIdx + idx
	}
}

// WriteLine1 starts a new line from 1 part. Non-variadic, inlinable.
func (b *LinesBuf) WriteLine1(p []byte) {
	b.indexes = append(b.indexes, len(b.buf))
	b.buf = append(b.buf, p...)
}

// WriteLine2 starts a new line from 2 parts. Faster than WriteLine(parts...).
func (b *LinesBuf) WriteLine2(p1, p2 []byte) {
	b.indexes = append(b.indexes, len(b.buf))
	b.buf = append(b.buf, p1...)
	b.buf = append(b.buf, p2...)
}

// WriteLine3 starts a new line from 3 parts. Faster than WriteLine(parts...).
func (b *LinesBuf) WriteLine3(p1, p2, p3 []byte) {
	b.indexes = append(b.indexes, len(b.buf))
	b.buf = append(b.buf, p1...)
	b.buf = append(b.buf, p2...)
	b.buf = append(b.buf, p3...)
}

// WritePaddedView bulk-copies a contiguous padded-line view into b.
// data contains all padded lines concatenated (no \n separators).
// offsets[i] is the byte offset of line i's start; offsets[end] is the
// exclusive end of the last line. Start and end are the visible line range.
// One bulk append for data, one slice extension for indexes — zero per-line
// overhead.
func (b *LinesBuf) WritePaddedView(data []byte, offsets []int, start, end int) {
	base := len(b.buf)
	b.buf = append(b.buf, data[offsets[start]:offsets[end]]...)

	length := end - start
	oldLen := len(b.indexes)
	newLen := oldLen + length

	if newLen <= cap(b.indexes) {
		b.indexes = b.indexes[:newLen]
	} else {
		newCap := cap(b.indexes) * 2
		for newCap < newLen {
			newCap *= 2
		}

		grown := make([]int, newLen, newCap)
		copy(grown, b.indexes[:oldLen])
		b.indexes = grown
	}

	baseOffset := base - offsets[start]
	for i := range length {
		b.indexes[oldLen+i] = baseOffset + offsets[start+i]
	}
}

// CopyFrom copies all data from src into b, reusing b's capacity when possible.
func (b *LinesBuf) CopyFrom(src *LinesBuf) {
	b.buf = append(b.buf[:0], src.buf...)
	b.indexes = append(b.indexes[:0], src.indexes...)
}

// AppendInt appends the decimal representation of val to buf and returns the
// extended buffer. Fast-path for 1–3 digit numbers; falls back to
// strconv.AppendInt for larger values. Zero allocations.
//
//nolint:mnd
func AppendInt(buf []byte, val int) []byte {
	if val < 0 {
		buf = append(buf, '-')
		val = -val
	}

	if val < 10 {
		//nolint:gosec // G115: safe — val<10, so '0'+val is in ['0','9']
		return append(buf, byte('0'+val))
	}

	if val < 100 {
		return append(buf, byte('0'+val/10), byte('0'+val%10))
	}

	if val < 1000 {
		buf = append(buf, byte('0'+val/100))
		buf = append(buf, byte('0'+(val/10)%10))

		return append(buf, byte('0'+val%10))
	}

	return strconv.AppendInt(buf, int64(val), 10)
}

// WriteLineInt starts a new line composed of prefix + decimal val + suffix,
// written directly into the buffer with zero allocations. Equivalent to
// fmt.Appendf(nil, "%s%d%s", prefix, val, suffix) but without interface{}
// boxing or fmt reflection overhead.
func (b *LinesBuf) WriteLineInt(prefix []byte, val int, suffix []byte) {
	b.indexes = append(b.indexes, len(b.buf))
	b.buf = append(b.buf, prefix...)
	b.buf = AppendInt(b.buf, val)
	b.buf = append(b.buf, suffix...)
}
