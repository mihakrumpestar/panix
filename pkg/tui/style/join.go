// Derived from charm.land/lipgloss/v2. See pkg/tui/LICENSE.charmbracelet.

package style

import (
	"math"
	"strings"
)

// Position represents alignment for JoinHorizontal and JoinVertical.
// 0.0 = Top/Left, 0.5 = Center, 1.0 = Bottom/Right.
type Position float64

const (
	Top    Position = 0.0
	Left   Position = 0.0
	Center Position = 0.5
	Bottom Position = 1.0
	Right  Position = 1.0
)

const maxPadSpaces = 512

var padSpaces = strings.Repeat(" ", maxPadSpaces)

// JoinHorizontal joins potentially multi-line strings along a vertical axis,
// aligned by position (Top, Center, Bottom). It replaces
// lipgloss.JoinHorizontal in hot paths, using CellWidth instead of the heavy
// uax29-based ansi.StringWidth for width calculations.
func JoinHorizontal(pos Position, strs ...string) string {
	if len(strs) == 0 {
		return ""
	}

	if len(strs) == 1 {
		return strs[0]
	}

	blocks, maxWidths, lineWidths, maxHeight := splitAndMeasureBlocks(strs)
	alignBlocksAndWidths(blocks, lineWidths, pos, maxHeight)

	return mergeBlocks(blocks, maxWidths, lineWidths, maxHeight)
}

func splitAndMeasureBlocks(strs []string) ([][]string, []int, [][]int, int) {
	blocks := make([][]string, len(strs))
	maxWidths := make([]int, len(strs))
	lineWidths := make([][]int, len(strs))
	maxHeight := 0

	for idx, str := range strs {
		blocks[idx] = splitLines(str)
		lineWidths[idx] = make([]int, len(blocks[idx]))

		for lineIdx, line := range blocks[idx] {
			w := CellWidth(line)
			lineWidths[idx][lineIdx] = w

			if w > maxWidths[idx] {
				maxWidths[idx] = w
			}
		}

		if len(blocks[idx]) > maxHeight {
			maxHeight = len(blocks[idx])
		}
	}

	return blocks, maxWidths, lineWidths, maxHeight
}

func alignBlocksAndWidths(blocks [][]string, lineWidths [][]int, pos Position, maxHeight int) {
	for idx := range blocks {
		if len(blocks[idx]) >= maxHeight {
			continue
		}

		extra := maxHeight - len(blocks[idx])

		switch pos {
		case Top:
			for range extra {
				blocks[idx] = append(blocks[idx], "")
				lineWidths[idx] = append(lineWidths[idx], 0)
			}
		case Bottom:
			padded := make([]string, extra, extra+len(blocks[idx]))
			widthPadded := make([]int, extra, extra+len(lineWidths[idx]))
			blocks[idx] = append(padded, blocks[idx]...)
			lineWidths[idx] = append(widthPadded, lineWidths[idx]...)
		default:
			split := int(math.Round(float64(extra) * float64(pos)))
			padded := make([]string, maxHeight)
			widthPadded := make([]int, maxHeight)

			copy(padded[split:], blocks[idx])
			copy(widthPadded[split:], lineWidths[idx])
			blocks[idx] = padded
			lineWidths[idx] = widthPadded
		}
	}
}

func mergeBlocks(blocks [][]string, maxWidths []int, lineWidths [][]int, maxHeight int) string {
	var builder strings.Builder

	for row := range maxHeight {
		if row > 0 {
			builder.WriteByte('\n')
		}

		for col, block := range blocks {
			line := block[row]
			builder.WriteString(line)

			pad := maxWidths[col] - lineWidths[col][row]
			if pad > 0 {
				if pad <= maxPadSpaces {
					builder.WriteString(padSpaces[:pad])
				} else {
					builder.WriteString(strings.Repeat(" ", pad))
				}
			}
		}
	}

	return builder.String()
}

// JoinVertical joins strings vertically, aligning them by position
// (Left, Center, Right). It replaces lipgloss.JoinVertical in hot paths,
// using CellWidth instead of the heavy uax29-based ansi.StringWidth.
func JoinVertical(pos Position, strs ...string) string {
	if len(strs) == 0 {
		return ""
	}

	if len(strs) == 1 {
		return strs[0]
	}

	lines, maxWidth, lineWidths := splitAndMeasureLines(strs)

	return buildVerticalOutput(lines, maxWidth, lineWidths, pos)
}

// splitAndMeasureLines splits each string into lines and finds the max width.
func splitAndMeasureLines(strs []string) ([][]string, int, [][]int) {
	lines := make([][]string, len(strs))
	lineWidths := make([][]int, len(strs))
	maxWidth := 0

	for idx, str := range strs {
		lines[idx] = splitLines(str)
		lineWidths[idx] = make([]int, len(lines[idx]))

		for lineIdx, line := range lines[idx] {
			w := CellWidth(line)
			lineWidths[idx][lineIdx] = w

			if w > maxWidth {
				maxWidth = w
			}
		}
	}

	return lines, maxWidth, lineWidths
}
// buildVerticalOutput writes aligned lines into a builder.
func buildVerticalOutput(lines [][]string, maxWidth int, lineWidths [][]int, pos Position) string {
	var builder strings.Builder

	for blockIdx, block := range lines {
		if blockIdx > 0 {
			builder.WriteByte('\n')
		}

		for lineIdx, line := range block {
			pad := maxWidth - lineWidths[blockIdx][lineIdx]

			writeAlignedLine(&builder, pos, line, pad)
			builder.WriteByte('\n')
		}
	}

	result := builder.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result
}

// writeAlignedLine writes a line to the builder with padding applied according
// to the given position (Left, Center, Right).
func writeAlignedLine(builder *strings.Builder, pos Position, line string, pad int) {
	switch {
	case pos >= Right:
		writePad(builder, pad)
		builder.WriteString(line)
	case pos == Center:
		left := pad / 2 //nolint:mnd
		right := pad - left
		writePad(builder, left)
		builder.WriteString(line)
		writePad(builder, right)
	default:
		builder.WriteString(line)
		writePad(builder, pad)
	}
}

// writePad appends count spaces to the builder, using the pre-allocated
// padSpaces buffer for small counts and strings.Repeat for larger ones.
func writePad(builder *strings.Builder, count int) {
	if count <= 0 {
		return
	}

	if count <= maxPadSpaces {
		builder.WriteString(padSpaces[:count])
	} else {
		builder.WriteString(strings.Repeat(" ", count))
	}
}

// splitLines splits a string by newline, returning each line as a separate
// string. An empty input returns a single empty-string element, matching
// lipgloss behavior.
//

func splitLines(str string) []string {
	if str == "" {
		return []string{""}
	}

	n := 1 + strings.Count(str, "\n")
	lines := make([]string, 0, n)

	start := 0

	for idx := range len(str) {
		if str[idx] == '\n' {
			lines = append(lines, str[start:idx])
			start = idx + 1
		}
	}

	lines = append(lines, str[start:])

	return lines
}
