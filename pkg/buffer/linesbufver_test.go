package buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	assert.Zero(t, buf.Len(), "new buffer should have 0 lines")
	assert.Nil(t, buf.Line(0), "out-of-bounds Line should return nil")
}

func TestWriteAndLine(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "hello", string(buf.Line(0)))
	assert.Equal(t, "world", string(buf.Line(1)))
}

func TestWriteLines(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	lines := [][]byte{[]byte("a"), []byte("bb"), []byte("ccc")}
	buf.WriteLines(lines)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "a", string(buf.Line(0)))
	assert.Equal(t, "ccc", string(buf.Line(2)))
}

func TestReset(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
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

	buf := NewLinesBufVer()
	buf.Write([]byte("first"))
	buf.Write([]byte("second"))
	buf.OverrideLastLine([]byte("replaced"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "first", string(buf.Line(0)))
	assert.Equal(t, "replaced", string(buf.Line(1)))
}

func TestOverrideLastLineEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.OverrideLastLine([]byte("only"))

	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, "only", string(buf.Line(0)))
}

func TestMultipleResetCycles(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	for cycle := range 100 {
		buf.Reset()
		buf.Write([]byte("cycle"))

		assert.Equal(t, 1, buf.Len(), "cycle %d: line count", cycle)
		assert.Equal(t, "cycle", string(buf.Line(0)), "cycle %d: content", cycle)
	}
}

func TestANSIContent(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	line := []byte("\x1b[38;2;180;190;254m╰── \x1b[0m\x1b[38;2;166;173;200m⚙\x1b[0m")
	buf.Write(line)

	assert.Equal(t, string(line), string(buf.Line(0)), "ANSI content should be preserved")
}

func TestLineOutOfBounds(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("only"))

	assert.Nil(t, buf.Line(-1), "negative index should return nil")
	assert.Nil(t, buf.Line(1), "index >= Len() should return nil")
}

func TestNewAtomic(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("safe line"))
	assert.Equal(t, "safe line", string(buf.Line(0)))
}

func TestAtomicOverrideLastLine(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("first"))
	buf.Write([]byte("second"))
	buf.OverrideLastLine([]byte("replaced"))

	assert.Equal(t, "replaced", string(buf.Line(1)))
}

func TestAtomicReset(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("data"))
	buf.Reset()

	assert.Zero(t, buf.Len())
}

func TestOverrideLastLineShorter(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("a very long line here"))
	buf.OverrideLastLine([]byte("short"))

	assert.Equal(t, "short", string(buf.Line(0)))

	buf.Write([]byte("next"))
	assert.Equal(t, "next", string(buf.Line(1)))
}

func TestWriteLinesEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.WriteLines(nil)

	assert.Zero(t, buf.Len())
}

func TestVersionIncrements(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
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

	buf := NewLinesBufVer()
	buf.Write([]byte("data"))
	ver := buf.Version()

	_ = buf.Line(0)
	_ = buf.Len()
	_ = buf.Version()

	assert.Equal(t, ver, buf.Version(), "read operations should not increment version")
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `["hello","world"]`, string(data))
}

func TestMarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()

	data, err := buf.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `[]`, string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()

	err := buf.UnmarshalJSON([]byte(`["alpha","beta","gamma"]`))
	require.NoError(t, err)

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, "alpha", string(buf.Line(0)))
	assert.Equal(t, "gamma", string(buf.Line(2)))
}

func TestUnmarshalJSONEmpty(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("stale"))

	err := buf.UnmarshalJSON([]byte(`[]`))
	require.NoError(t, err)

	assert.Zero(t, buf.Len())
}

func TestUnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()

	err := buf.UnmarshalJSON([]byte(`not json`))
	assert.Error(t, err, "invalid JSON should return an error")
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("line one"))
	buf.Write([]byte("line two"))
	buf.Write([]byte("line three"))

	data, err := buf.MarshalJSON()
	require.NoError(t, err)

	bufRound := NewLinesBufVer()
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

	buf := NewLinesBufVer()
	assert.Nil(t, buf.Bytes(), "empty buffer should return nil")

	buf.Write([]byte("hello"))
	buf.Write([]byte("world"))

	assert.Equal(t, "hello\nworld", string(buf.Bytes()))
}

func TestStringMethod(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("alpha"))
	buf.Write([]byte("beta"))

	assert.Equal(t, "alpha\nbeta", buf.String())
}

func TestAppendAndWrite(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("first"))
	buf.Append([]byte(" +"))
	buf.Append([]byte("more"))
	buf.Write([]byte("second"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "first +more", string(buf.Line(0)))
	assert.Equal(t, "second", string(buf.Line(1)))
}

func TestAppendStringAndWrite(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Write([]byte("hello"))
	buf.AppendString(" world")
	buf.Write([]byte("next"))

	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, "hello world", string(buf.Line(0)))
	assert.Equal(t, "next", string(buf.Line(1)))
}

func TestAppendToNewBuffer(t *testing.T) {
	t.Parallel()

	buf := NewLinesBufVer()
	buf.Append([]byte("started"))
	assert.Equal(t, 1, buf.Len())
	assert.Equal(t, "started", string(buf.Line(0)))
}
