package buffer

import "bytes"

// LinesBufDiff is a diff-capable LinesBuf for frame diffing in zeroterm.
// The Diff method compares two buffers line-by-line and copies
// b (current frame) into src (previous frame) so src becomes the
// new "previous" frame for the next call.
type LinesBufDiff struct {
	*LinesBuf

	diffs []int
}

func NewLinesBufDiff() *LinesBufDiff {
	return &LinesBufDiff{
		LinesBuf: NewLinesBuf(),
	}
}

func (b *LinesBufDiff) Write(line []byte) {
	b.WriteLine(line)
}

// AppendString is a string version of Append.
func (b *LinesBufDiff) AppendString(s string) {
	b.Append([]byte(s))
}

// OverrideLastLine rewrites the last line in-place.
func (b *LinesBufDiff) OverrideLastLine(line []byte) {
	b.LinesBuf.OverrideLastLine(line)
}

// Line returns the content of line i, or nil if out of bounds.
func (b *LinesBufDiff) Line(i int) []byte {
	if i < 0 || i >= b.Len() {
		return nil
	}

	return b.LinesBuf.Line(i)
}

func (b *LinesBufDiff) LastLine() []byte {
	return b.Line(b.Len() - 1)
}

func (b *LinesBufDiff) String() string {
	if b.Len() == 0 {
		return ""
	}

	size := len(b.LinesBuf.buf) + b.Len() - 1
	out := make([]byte, 0, size)

	for i := range b.Len() {
		if i > 0 {
			out = append(out, '\n')
		}

		out = append(out, b.Line(i)...)
	}

	return string(out)
}

func (b *LinesBufDiff) Bytes() []byte {
	if b.Len() == 0 {
		return nil
	}

	size := len(b.LinesBuf.buf) + b.Len() - 1
	result := make([]byte, 0, size)

	for i := range b.Len() {
		if i > 0 {
			result = append(result, '\n')
		}

		result = append(result, b.Line(i)...)
	}

	return result
}

// Diff compares b (new/current frame) with src (previous frame) line-by-line,
// returns indices of changed lines, and copies b into src so src becomes
// the new "previous" frame for the next diff call.
func (b *LinesBufDiff) Diff(src *LinesBufDiff) []int {
	b.diffs = b.diffs[:0]

	dstN := b.Len()
	srcN := src.Len()
	common := min(dstN, srcN)

	for idx := range common {
		if bytes.Equal(b.LinesBuf.Line(idx), src.LinesBuf.Line(idx)) {
			continue
		}

		b.diffs = append(b.diffs, idx)
	}

	for idx := common; idx < dstN; idx++ {
		b.diffs = append(b.diffs, idx)
	}

	src.CopyFrom(b.LinesBuf)

	return b.diffs
}

// Reset clears all content while preserving underlying capacity.
func (b *LinesBufDiff) Reset() {
	b.LinesBuf.Reset()
}
