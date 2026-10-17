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
		{"cyrillic combining mark", '\u0483', 0},
		{"hebrew point hiriq", '\u05B0', 0},
		{"arabic shadda", '\u0651', 0},
		{"katakana voiced sound mark", '\u3099', 0},
		{"katakana semi-voiced sound mark", '\u309A', 0},
		{"ideographic tone mark", '\u302A', 0},
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
		// U+1F5A5 is the machine icon in 11 built-in themes. It has
		// Emoji_Presentation=No, so it is one cell like the rest of the
		// ecosystem (go-runewidth, x/ansi, uniseg, lipgloss).
		{"desktop computer", '🖥', 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, runeWidth(tc.r), "runeWidth(%U)", tc.r)
		})
	}
}

// TestRuneWidth_TextPresentationPictographs pins Emoji_Presentation=No glyphs as
// single cell, so no blanket emoji range forces them to 2. U+1F5FF is absent
// because go-runewidth measures it 2.
func TestRuneWidth_TextPresentationPictographs(t *testing.T) {
	t.Parallel()

	exceptions := []struct{ lo, hi rune }{
		{0x1F321, 0x1F321},
		{0x1F324, 0x1F32C},
		{0x1F336, 0x1F336},
		{0x1F37D, 0x1F37D},
		{0x1F396, 0x1F397},
		{0x1F399, 0x1F39B},
		{0x1F39E, 0x1F39F},
		{0x1F3CB, 0x1F3CE},
		{0x1F3D4, 0x1F3DF},
		{0x1F3F1, 0x1F3F2},
		{0x1F3F7, 0x1F3F7},
		{0x1F43F, 0x1F43F},
		{0x1F441, 0x1F441},
		{0x1F4FD, 0x1F4FD},
		{0x1F549, 0x1F54A},
		{0x1F56F, 0x1F570},
		{0x1F573, 0x1F579},
		{0x1F587, 0x1F587},
		{0x1F58A, 0x1F58D},
		{0x1F590, 0x1F590},
		{0x1F5A5, 0x1F5A5},
		{0x1F5A8, 0x1F5A8},
		{0x1F5B1, 0x1F5B2},
		{0x1F5BC, 0x1F5BC},
		{0x1F5C2, 0x1F5C4},
		{0x1F5D1, 0x1F5D3},
		{0x1F5DC, 0x1F5DE},
		{0x1F5E1, 0x1F5E1},
		{0x1F5E3, 0x1F5E3},
		{0x1F5E8, 0x1F5E8},
		{0x1F5EF, 0x1F5EF},
		{0x1F5F3, 0x1F5F3},
		{0x1F5FA, 0x1F5FA},
	}

	for _, ex := range exceptions {
		for r := ex.lo; r <= ex.hi; r++ {
			assert.Equal(t, 1, runeWidth(r), "text-presentation pictograph %U must be width 1", r)
		}
	}
}
