package overlay

import "github.com/mihakrumpestar/panix/pkg/tui/style"

// Compose draws box centered in the whole frame and splices it into the affected
// rows, deriving the width from the widest row. Use ComposeRegion when the
// terminal size is known. Returns where the box was drawn.
func Compose(frame Frame, box *Box) Placement {
	height := frame.Len()
	width := 0

	for i := range height {
		width = max(width, style.CellWidth(frame.Line(i)))
	}

	return ComposeRegion(frame, box, Rect{Height: height, Width: width})
}

// ComposeRegion centers box in region and splices its cells into the affected
// rows, preserving each row's content and styling outside the box columns. Rows
// outside the box are untouched. The region is clamped to the frame; a larger box
// anchors top-left and clips at the bottom and right edges. Returns the placement.
func ComposeRegion(frame Frame, box *Box, region Rect) Placement {
	if frame == nil || box == nil || box.Len() == 0 {
		return Placement{}
	}

	row := max(region.Row, 0)
	col := max(region.Col, 0)
	height := min(region.Height, frame.Len()-row)
	width := region.Width

	if height <= 0 || width <= 0 {
		return Placement{}
	}

	top, left := Rect{Row: row, Col: col, Height: height, Width: width}.Center(box.Width(), box.Len())

	drawn := Placement{
		Row:    top,
		Col:    left,
		Height: min(box.Len(), row+height-top),
		Width:  min(box.Width(), col+width-left),
	}

	for lineIdx := range drawn.Height {
		frame.SetLine(top+lineIdx, style.SpliceCells(
			frame.Line(top+lineIdx), left, left+drawn.Width, box.Line(lineIdx),
		))
	}

	return drawn
}
