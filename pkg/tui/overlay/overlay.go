package overlay

import "github.com/mihakrumpestar/panix/pkg/buffer"

// Frame is a rendered frame whose rows can be read and replaced. Both
// *buffer.LinesBufDiff and *buffer.LinesBuf implement it.
type Frame interface {
	Len() int
	Line(i int) []byte
	SetLine(i int, line []byte)
}

// Rect is a rectangular region of a frame in cells.
type Rect struct {
	Row    int
	Col    int
	Height int
	Width  int
}

// Center returns the top-left position centering a boxW x boxH box in r, clamped
// so a larger box anchors at r's top-left (the caller clips the overflow). Odd
// free space puts the extra cell below or right of the box.
func (r Rect) Center(boxW, boxH int) (int, int) {
	row := max(r.Row, 0)
	col := max(r.Col, 0)

	if boxH < r.Height {
		row += (r.Height - boxH) / 2
	}

	if boxW < r.Width {
		col += (r.Width - boxW) / 2
	}

	return row, col
}

// Placement describes where a box was drawn on a frame, after clipping.
type Placement struct {
	Row    int // row of the box's first drawn line
	Col    int // cell of the box's first drawn column
	Height int // rows actually drawn
	Width  int // cells actually drawn
}

// compile-time check: the concrete buffer types satisfy Frame.
var (
	_ Frame = (*buffer.LinesBuf)(nil)
	_ Frame = (*buffer.LinesBufDiff)(nil)
)
