package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalBorder(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	assert.Equal(t, []byte("┌"), brd.TopLeft)
	assert.Equal(t, []byte("┐"), brd.TopRight)
	assert.Equal(t, []byte("└"), brd.BottomLeft)
	assert.Equal(t, []byte("┘"), brd.BottomRight)
	assert.Equal(t, []byte("─"), brd.Horizontal)
	assert.Equal(t, []byte("│"), brd.Vertical)
	assert.Equal(t, []byte("┬"), brd.TopMid)
	assert.Equal(t, []byte("┴"), brd.BottomMid)
	assert.Equal(t, []byte("├"), brd.LeftMid)
	assert.Equal(t, []byte("┤"), brd.RightMid)
	assert.Equal(t, []byte("┼"), brd.MidMid)
}

func TestRoundedBorder(t *testing.T) {
	t.Parallel()

	brd := RoundedBorder()

	assert.Equal(t, []byte("╭"), brd.TopLeft)
	assert.Equal(t, []byte("╮"), brd.TopRight)
	assert.Equal(t, []byte("╰"), brd.BottomLeft)
	assert.Equal(t, []byte("╯"), brd.BottomRight)
	assert.Equal(t, []byte("─"), brd.Horizontal)
	assert.Equal(t, []byte("│"), brd.Vertical)
}

func TestHiddenBorder(t *testing.T) {
	t.Parallel()

	brd := HiddenBorder()

	assert.Empty(t, brd.TopLeft)
	assert.Empty(t, brd.Horizontal)
	assert.Empty(t, brd.Vertical)
}

func TestMarkdownBorder(t *testing.T) {
	t.Parallel()

	brd := MarkdownBorder()

	assert.Equal(t, []byte("|"), brd.TopLeft)
	assert.Equal(t, []byte("|"), brd.TopRight)
	assert.Equal(t, []byte("|"), brd.BottomLeft)
	assert.Equal(t, []byte("|"), brd.BottomRight)
	assert.Equal(t, []byte("-"), brd.Horizontal)
	assert.Equal(t, []byte("|"), brd.Vertical)
}

func TestBorder_NoPerSideColorByDefault(t *testing.T) {
	t.Parallel()

	brd := NormalBorder()

	assert.Empty(t, brd.topFg)
	assert.Empty(t, brd.rightFg)
	assert.Empty(t, brd.bottomFg)
	assert.Empty(t, brd.leftFg)
}
