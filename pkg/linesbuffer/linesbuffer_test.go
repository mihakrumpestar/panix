package linesbuffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	buf := New()
	assert.Zero(t, buf.Len(), "new buffer should have 0 lines")
	assert.Nil(t, buf.Line(0), "out-of-bounds Line should return nil")
}

func TestWriteAndLine(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "hello", string(buf.Line(0)))
	assert.Equal(t, "world", string(buf.Line(1)))
}

func TestWriteLines(t *testing.T) {
	t.Parallel()

	buf := New()
	lines := [][]byte{[]byte("a"), []byte("bb"), []byte("ccc")}
	buf.WriteLines(lines)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "a", string(buf.Line(0)))
	assert.Equal(t, "ccc", string(buf.Line(2)))
}

func TestReset(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("line1"))
	buf.Write([]byte("line2"))
	buf.Reset()

	assert.Zero(t, buf.Len(), "buffer should be empty after reset")

	buf.Write([]byte("new line"))
	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, "new line", string(buf.Line(0)))
}

func TestOverrideLastLine(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("first"))
	buf.Write([]byte("second"))
	buf.OverrideLastLine([]byte("replaced"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "first", string(buf.Line(0)))
	assert.Equal(t, "replaced", string(buf.Line(1)))
}

func TestOverrideLastLineEmpty(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.OverrideLastLine([]byte("only"))

	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, "only", string(buf.Line(0)))
}

func TestDiffIdentical(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("line1"))
	first.Write([]byte("line2"))

	second := New()
	second.Write([]byte("line1"))
	second.Write([]byte("line2"))

	assert.Empty(t, first.Diff(second))
}

func TestDiffChanged(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("line1"))
	first.Write([]byte("line2"))
	first.Write([]byte("line3"))

	second := New()
	second.Write([]byte("line1"))
	second.Write([]byte("changed"))
	second.Write([]byte("line3"))

	assert.Equal(t, []int{1}, first.Diff(second))
}

func TestDiffGrew(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("line1"))
	first.Write([]byte("line2"))
	first.Write([]byte("line3"))

	second := New()
	second.Write([]byte("line1"))
	second.Write([]byte("line2"))

	assert.Equal(t, []int{2}, first.Diff(second))
}

func TestDiffShrunk(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("line1"))

	second := New()
	second.Write([]byte("line1"))
	second.Write([]byte("line2"))

	assert.Empty(t, first.Diff(second), "fewer lines in new should produce no diffs")
}

func TestDiffLengthMismatch(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("same"))

	second := New()
	second.Write([]byte("different"))

	assert.Equal(t, []int{0}, first.Diff(second))
}

func TestDiffReuse(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("a"))
	first.Write([]byte("b"))

	second := New()
	second.Write([]byte("a"))
	second.Write([]byte("c"))

	diffs1 := first.Diff(second)
	assert.Len(t, diffs1, 1)

	first.Write([]byte("d"))

	diffs2 := first.Diff(second)
	assert.Len(t, diffs2, 2)

	_ = diffs1
}

func TestMultipleResetCycles(t *testing.T) {
	t.Parallel()

	buf := New()
	for cycle := range 100 {
		buf.Reset()
		buf.Write([]byte("cycle"))

		assert.Equal(t, 1, buf.Len(), "cycle %d: line count", cycle)
		assert.Equal(t, "cycle", string(buf.Line(0)), "cycle %d: content", cycle)
	}
}

func TestANSIContent(t *testing.T) {
	t.Parallel()

	buf := New()
	line := []byte("\x1b[38;2;180;190;254m╰── \x1b[0m\x1b[38;2;166;173;200m⚙\x1b[0m")
	buf.Write(line)

	assert.Equal(t, string(line), string(buf.Line(0)), "ANSI content should be preserved")
}

func TestLineOutOfBounds(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("only"))

	assert.Nil(t, buf.Line(-1), "negative index should return nil")
	assert.Nil(t, buf.Line(1), "index >= Len() should return nil")
}

func TestNewPooledAndRelease(t *testing.T) {
	t.Parallel()

	buf := NewPooled()
	assert.True(t, buf.pooled, "pooled flag should be set")

	buf.Write([]byte("from pool"))
	assert.Equal(t, 1, buf.Len())

	buf.Release()

	bufReused := NewPooled()
	bufReused.Write([]byte("reused"))
	assert.Equal(t, 1, bufReused.Len())

	bufReused.Release()
}

func TestReleaseNonPooled(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Release()

	assert.Zero(t, buf.Len(), "release on non-pooled should be no-op")
}

func TestNewAtomic(t *testing.T) {
	t.Parallel()

	buf := NewAtomic()
	assert.True(t, buf.atomic, "atomic flag should be set")

	buf.Write([]byte("safe line"))
	assert.Equal(t, "safe line", string(buf.Line(0)))
}

func TestAtomicDiff(t *testing.T) {
	t.Parallel()

	first := NewAtomic()
	first.Write([]byte("line1"))
	first.Write([]byte("line2"))

	second := NewAtomic()
	second.Write([]byte("line1"))
	second.Write([]byte("changed"))

	assert.Equal(t, []int{1}, first.Diff(second))
}

func TestAtomicOverrideLastLine(t *testing.T) {
	t.Parallel()

	buf := NewAtomic()
	buf.Write([]byte("first"))
	buf.Write([]byte("second"))
	buf.OverrideLastLine([]byte("replaced"))

	assert.Equal(t, "replaced", string(buf.Line(1)))
}

func TestAtomicReset(t *testing.T) {
	t.Parallel()

	buf := NewAtomic()
	buf.Write([]byte("data"))
	buf.Reset()

	assert.Zero(t, buf.Len())
}

func TestOverrideLastLineShorter(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("a very long line here"))
	buf.OverrideLastLine([]byte("short"))

	assert.Equal(t, "short", string(buf.Line(0)))

	buf.Write([]byte("next"))
	assert.Equal(t, "next", string(buf.Line(1)))
}

func TestDiffMultipleChanged(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("alpha"))
	first.Write([]byte("beta"))
	first.Write([]byte("gamma"))
	first.Write([]byte("delta"))

	second := New()
	second.Write([]byte("alpha"))
	second.Write([]byte("BETA"))
	second.Write([]byte("gamma"))
	second.Write([]byte("DELTA"))

	assert.Equal(t, []int{1, 3}, first.Diff(second))
}

func TestDiffEmptyBuffers(t *testing.T) {
	t.Parallel()

	first := New()
	second := New()

	assert.Empty(t, first.Diff(second))
}

func TestWriteLinesEmpty(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.WriteLines(nil)

	assert.Zero(t, buf.Len())
}

func TestLinesBufferContiguity(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("aaa"))
	buf.Write([]byte("bbbbb"))
	buf.Write([]byte("c"))

	line0 := buf.Line(0)
	line1 := buf.Line(1)
	line2 := buf.Line(2)

	assert.Same(t, &buf.data[0], &line0[0], "line0 should start at data[0]")
	assert.Len(t, line0, 3)
	assert.Same(t, &buf.data[3], &line1[0], "line1 should follow line0")
	assert.Same(t, &buf.data[8], &line2[0], "line2 should follow line1")
}

func TestDiffPreAllocReuse(t *testing.T) { //nolint:paralleltest // AllocsPerRun doesn't support parallel
	first := New()
	second := New()

	first.Write([]byte("x"))
	second.Write([]byte("y"))

	allocs := testing.AllocsPerRun(10, func() {
		first.Diff(second)
	})
	assert.Zero(t, allocs, "Diff should have zero allocations")
}

func TestBytesEqualFallback(t *testing.T) {
	t.Parallel()

	first := New()
	first.Write([]byte("same"))

	second := New()
	second.Write([]byte("same"))

	assert.Empty(t, first.Diff(second), "identical content should have no diffs")

	third := New()
	third.Write([]byte("diff"))

	assert.Len(t, first.Diff(third), 1, "different content should have 1 diff")
}

func TestWriteAfterOverrideDoesNotLeak(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("long line that is long"))
	buf.OverrideLastLine([]byte("short"))
	buf.Write([]byte("after"))

	assert.Equal(t, "short", string(buf.Line(0)))
	assert.Equal(t, "after", string(buf.Line(1)))

	expected := "shortafter"
	assert.Equal(t, expected, string(buf.data[:len(expected)]), "data should be contiguous")
}

func TestPooledBufferResetAfterRelease(t *testing.T) {
	t.Parallel()

	buf := NewPooled()
	buf.Write([]byte("data"))
	buf.Release()

	bufReused := NewPooled()
	bufReused.Write([]byte("new"))
	assert.Equal(t, 1, bufReused.Len())

	bufReused.Release()
}

func TestVersionIncrements(t *testing.T) {
	t.Parallel()

	buf := New()
	v0 := buf.Version()

	buf.Write([]byte("line1"))
	assert.NotEqual(t, v0, buf.Version(), "Write should increment version")

	v1 := buf.Version()
	buf.WriteLines([][]byte{[]byte("line2"), []byte("line3")})
	assert.NotEqual(t, v1, buf.Version(), "WriteLines should increment version")

	v2 := buf.Version()
	buf.OverrideLastLine([]byte("replaced"))
	assert.NotEqual(t, v2, buf.Version(), "OverrideLastLine should increment version")

	v3 := buf.Version()
	buf.Reset()
	assert.NotEqual(t, v3, buf.Version(), "Reset should increment version")
}

func TestVersionUnchangedOnRead(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("data"))
	ver := buf.Version()

	_ = buf.Line(0)
	_ = buf.Len()
	_ = buf.Version()

	assert.Equal(t, ver, buf.Version(), "read operations should not increment version")
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `["hello","world"]`, string(data))
}

func TestMarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := New()

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `[]`, string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	buf := New()

	err := buf.UnmarshalJSON([]byte(`["alpha","beta","gamma"]`))
	require.NoError(t, err)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "alpha", string(buf.Line(0)))
	assert.Equal(t, "gamma", string(buf.Line(2)))
}

func TestUnmarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("stale"))

	err := buf.UnmarshalJSON([]byte(`[]`))
	require.NoError(t, err)

	assert.Zero(t, buf.Len())
}

func TestUnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	buf := New()

	err := buf.UnmarshalJSON([]byte(`not json`))
	assert.Error(t, err, "invalid JSON should return an error")
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("line one"))
	buf.Write([]byte("line two"))
	buf.Write([]byte("line three"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)

	bufRound := New()
	err = bufRound.UnmarshalJSON(data)
	require.NoError(t, err)

	assert.Equal(t, buf.Len(), bufRound.Len())

	for lineIdx := range buf.Len() {
		assert.Equal(t, string(buf.Line(lineIdx)), string(bufRound.Line(lineIdx)),
			"line %d mismatch", lineIdx)
	}
}

func TestBytesBasic(t *testing.T) {
	t.Parallel()

	buf := New()
	assert.Nil(t, buf.Bytes(), "empty buffer should return nil")

	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	assert.Equal(t, "hello\nworld", string(buf.Bytes()))
}

func TestStringMethod(t *testing.T) {
	t.Parallel()

	buf := New()
	buf.Write([]byte("alpha"))
	buf.Write([]byte("beta"))

	assert.Equal(t, "alpha\nbeta", buf.String())
}
