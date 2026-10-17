package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// wideRune is a double width rune fixture for straddle cases.
const wideRune = "\u6f22" //nolint:gosmopolitan // double width rune test fixture

// spliceCase is one row of the SpliceCells tables. want is the exact expected
// output bytes, escape sequences included.
type spliceCase struct {
	name        string
	line        string
	start       int
	end         int
	replacement string
	want        string
}

// runSpliceCases splices per case and asserts the exact output bytes.
func runSpliceCases(t *testing.T, cases []spliceCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SpliceCells([]byte(tt.line), tt.start, tt.end, []byte(tt.replacement))

			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestSpliceCells(t *testing.T) {
	t.Parallel()

	runSpliceCases(t, []spliceCase{
		{
			name: "plain splice keeps both sides",
			line: "abcdef", start: 2, end: 4, replacement: "XY",
			want: "abXYef",
		},
		{
			name: "short replacement is space padded to the range",
			line: "abcdef", start: 2, end: 5, replacement: "XY",
			want: "abXY f",
		},
		{
			name: "long replacement is truncated to the range",
			line: "abcdef", start: 2, end: 3, replacement: "XYZ",
			want: "abXdef",
		},
		{
			name: "row shorter than start is blank padded up to it",
			line: "ab", start: 4, end: 6, replacement: "XY",
			want: "ab  XY",
		},
		{
			name: "row shorter than end covers no tail cells",
			line: "abcdef", start: 2, end: 10, replacement: "XY",
			want: "abXY      ",
		},
		{
			name: "row shorter than the whole range aligns both sides",
			line: "ab", start: 4, end: 8, replacement: "X",
			want: "ab  X   ",
		},
		{
			name: "empty range inserts without dropping cells",
			line: "abcdef", start: 3, end: 3, replacement: "X",
			want: "abcXdef",
		},
		{
			name: "start beyond the row width pads the head",
			line: "ab", start: 5, end: 7, replacement: "XY",
			want: "ab   XY",
		},
		{
			name: "clamped range splices from the row start",
			line: "abcdef", start: -2, end: 1, replacement: "X",
			want: "Xbcdef",
		},
	})
}

func TestSpliceCellsWideRunes(t *testing.T) {
	t.Parallel()

	line := "a" + wideRune + "b"

	runSpliceCases(t, []spliceCase{
		{
			name: "wide rune split at start fills its head cells with blanks",
			line: line, start: 2, end: 3, replacement: "X",
			want: "a Xb",
		},
		{
			name: "wide rune split at end fills its tail cells with blanks",
			line: line, start: 1, end: 2, replacement: "X",
			want: "aX b",
		},
	})
}

func TestSpliceCellsStyled(t *testing.T) {
	t.Parallel()

	runSpliceCases(t, []spliceCase{
		{
			name: "styled sides keep their style state",
			line: "\x1b[31mabcdef\x1b[m", start: 2, end: 4, replacement: "XY",
			want: "\x1b[31mab\x1b[mXY\x1b[m\x1b[31mef\x1b[m",
		},
		{
			name: "tail style set before the cut is restored after the splice",
			line: "\x1b[31mabc\x1b[mdef", start: 1, end: 2, replacement: "X",
			want: "\x1b[31ma\x1b[mX\x1b[m\x1b[31mc\x1b[mdef",
		},
		{
			name: "style change between head and tail is replayed",
			line: "ab\x1b[31mcd", start: 2, end: 2, replacement: "X",
			want: "ab\x1b[31m\x1b[mX\x1b[m\x1b[31mcd",
		},
		{
			name: "replacement keeps its own styles",
			line: "abcdef", start: 2, end: 4, replacement: "\x1b[32mXY\x1b[m",
			want: "ab\x1b[32mXY\x1b[m\x1b[mef",
		},
		{
			name: "end style state of the line survives a full replacement",
			line: "\x1b[31mab", start: 0, end: 2, replacement: "X",
			want: "\x1b[31m\x1b[mX \x1b[m\x1b[31m",
		},
	})
}

func TestSpliceCellsWidthPreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		line        string
		start       int
		end         int
		replacement string
		wantWidth   int
	}{
		{name: "plain row", line: "abcdef", start: 2, end: 4, replacement: "XY", wantWidth: 6},
		{name: "wide rune straddle at start", line: "a" + wideRune + "b", start: 2, end: 3, replacement: "X", wantWidth: 4},
		{name: "wide rune straddle at end", line: "a" + wideRune + "b", start: 1, end: 2, replacement: "X", wantWidth: 4},
		{name: "row extended to the replacement range", line: "ab", start: 4, end: 8, replacement: "X", wantWidth: 8},
		{name: "styled row", line: "\x1b[31mabc\x1b[mdef", start: 1, end: 2, replacement: "X", wantWidth: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SpliceCells([]byte(tt.line), tt.start, tt.end, []byte(tt.replacement))

			assert.Equal(t, tt.wantWidth, CellWidth(got), "the composed row must keep its cell width")
		})
	}
}

// TestSpliceCellsIdempotent pins the compositing invariant of overlays:
// splicing the same replacement into the same range twice yields identical
// bytes on plain rows.
func TestSpliceCellsIdempotent(t *testing.T) {
	t.Parallel()

	line := []byte("0123456789")

	once := SpliceCells(line, 3, 5, []byte("XY"))
	twice := SpliceCells(once, 3, 5, []byte("XY"))

	assert.Equal(t, "012XY56789", string(once))
	assert.Equal(t, string(once), string(twice))
}
