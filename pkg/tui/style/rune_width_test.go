package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuneWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii lowercase", 'a', 1},
		{"cjk ni", '你', 2},
		{"cjk wen", '文', 2},
		{"hiragana a", 'あ', 2},
		{"combining acute", '\u0301', 0},
		{"zero width joiner", '\u200D', 0},
		{"white heavy check mark", '✅', 2},
		{"clipboard emoji", '📋', 2},
		{"gear", '⚙', 1},
		{"ballot x", '✗', 1},
		{"box drawing horizontal", '─', 1},
		{"braille spinner", '⣾', 1},
		{"latin e acute", 'é', 1},
		{"nul", '\x00', 0},
		{"ideographic space", '　', 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, runeWidth(tc.r), "runeWidth(%U)", tc.r)
		})
	}
}
