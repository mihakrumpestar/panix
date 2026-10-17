package style

import (
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_RuneOverride(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	assert.Equal(t, 2, CellWidth([]byte("\u6f22")), "static table baseline")

	InstallMeasuredGlyphWidths(map[string]int{"": 5, "\u6f22": 1})

	assert.Equal(t, 1, CellWidth([]byte("\u6f22")))
	assert.Equal(t, 1, runeWidth('\u6f22'))
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_SequenceBeatsPerRuneSum(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	glyph := "\u2699\ufe0f" // gear + variation selector 16

	assert.Equal(t, 1, CellWidth([]byte(glyph)), "static table sums to one")

	InstallMeasuredGlyphWidths(map[string]int{glyph: 2})

	assert.Equal(t, 2, CellWidth([]byte(glyph)))
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_LongestSequenceWins(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	InstallMeasuredGlyphWidths(map[string]int{
		"\u6f22\u6f22":       3,
		"\u6f22\u6f22\u6f22": 5,
	})

	assert.Equal(t, 5, CellWidth([]byte("\u6f22\u6f22\u6f22")))
	assert.Equal(t, 3, CellWidth([]byte("\u6f22\u6f22")))
	assert.Equal(t, 2, CellWidth([]byte("\u6f22")), "an unmatched rune keeps the static width")
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_AllWidthPathsAgree(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	tests := []struct {
		name  string
		glyph string
	}{
		{"VS16 sequence", "\u2699\ufe0f"},
		{"ZWJ sequence", "\U0001F468\u200d\U0001F469"},
		{"ASCII-start keycap sequence", "1\ufe0f\u20e3"},
	}

	measured := map[string]int{}
	for _, testCase := range tests {
		measured[testCase.glyph] = 2
	}

	InstallMeasuredGlyphWidths(measured)

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, 2, CellWidth([]byte(testCase.glyph)), "CellWidth must use the measured sequence")

			truncated := TruncateToWidth([]byte(testCase.glyph+"X"), 2, false)
			assert.Equal(t, testCase.glyph, string(truncated), "truncation must not split the measured sequence")

			spliced := SpliceCells([]byte(testCase.glyph+"X"), 0, 2, []byte("Y"))
			assert.Equal(t, "Y X", string(spliced), "splicing must treat the sequence as two cells")

			content := buffer.NewLinesBuf()
			content.WriteLine([]byte(testCase.glyph + " X"))

			wrapped := buffer.NewLinesBuf()
			WrapBuf(wrapped, content, 2, "")

			lines := make([]string, wrapped.Len())
			for i := range wrapped.Len() {
				lines[i] = string(wrapped.Line(i))
			}

			wrapped.Release()
			content.Release()

			require.Len(t, lines, 2, "wrapping must measure the sequence as two cells")
			assert.Equal(t, testCase.glyph, lines[0])
			assert.Equal(t, "X", lines[1])
		})
	}
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_NilClears(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	InstallMeasuredGlyphWidths(map[string]int{"\u6f22": 1})
	assert.Equal(t, 1, CellWidth([]byte("\u6f22")))

	InstallMeasuredGlyphWidths(nil)
	assert.Equal(t, 2, CellWidth([]byte("\u6f22")))
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_EmptyMapClears(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	InstallMeasuredGlyphWidths(map[string]int{"\u6f22": 1})
	InstallMeasuredGlyphWidths(map[string]int{})

	assert.Nil(t, measuredGlyphWidths.Load())
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestInstallMeasuredGlyphWidths_ASCIIFastPathUnaffected(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	InstallMeasuredGlyphWidths(map[string]int{"a": 9, "\u6f22": 1})

	assert.Equal(t, 5, CellWidth([]byte("hello")), "plain ASCII stays one cell per byte")
	assert.Equal(t, 1, CellWidth([]byte("a")), "the ASCII fast path ignores rune overrides")
	assert.Equal(t, 3, CellWidth([]byte("a\u6f22b")))
}

//nolint:paralleltest // mutates the process-global measured width overrides
func TestResetMeasuredGlyphWidths(t *testing.T) {
	t.Cleanup(ResetMeasuredGlyphWidths)

	InstallMeasuredGlyphWidths(map[string]int{"\u6f22": 1})
	ResetMeasuredGlyphWidths()

	assert.Equal(t, 2, CellWidth([]byte("\u6f22")))
	assert.Nil(t, measuredGlyphWidths.Load())
}
