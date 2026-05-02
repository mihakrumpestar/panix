package render

type LineDiff struct {
	Y int
}

// Diff compares newBuf against oldBuf and returns the list of changed lines.
// Uses lineVersions for O(1) unchanged-line detection. WriteDiff always
// writes full lines (0 to width-1) so there are no gap-based stale content bugs.
func Diff(newBuf, oldBuf *CellBuf) []LineDiff {
	if newBuf.width != oldBuf.width {
		return fullRedraw(newBuf)
	}

	height := min(oldBuf.height, newBuf.height)

	var diffs []LineDiff

	for y := range height {
		if newBuf.LineVersion(y) == oldBuf.LineVersion(y) {
			continue
		}

		if lineChanged(newBuf.Line(y), oldBuf.Line(y)) {
			diffs = append(diffs, LineDiff{Y: y})
		}
	}

	if newBuf.height > oldBuf.height {
		for y := oldBuf.height; y < newBuf.height; y++ {
			if lineChangedFromEmpty(newBuf.Line(y)) {
				diffs = append(diffs, LineDiff{Y: y})
			}
		}
	}

	return diffs
}

func lineChanged(newLine, oldLine []Cell) bool {
	n := min(len(newLine), len(oldLine))
	for x := range n {
		if !newLine[x].VisualEqual(oldLine[x]) {
			return true
		}
	}

	if len(newLine) > len(oldLine) {
		for x := len(oldLine); x < len(newLine); x++ {
			if !newLine[x].VisualEqual(EmptyCell) {
				return true
			}
		}
	}

	if len(oldLine) > len(newLine) {
		for x := len(newLine); x < len(oldLine); x++ {
			if !oldLine[x].VisualEqual(EmptyCell) {
				return true
			}
		}
	}

	return false
}

func lineChangedFromEmpty(line []Cell) bool {
	for x := range line {
		if !line[x].VisualEqual(EmptyCell) {
			return true
		}
	}

	return false
}

func fullRedraw(buf *CellBuf) []LineDiff {
	diffs := make([]LineDiff, buf.height)
	for y := range buf.height {
		diffs[y] = LineDiff{Y: y}
	}

	return diffs
}
