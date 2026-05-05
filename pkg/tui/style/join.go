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

	blocks, maxWidths, maxHeight := splitAndMeasureBlocks(strs)
	alignBlocks(blocks, pos, maxHeight)

	return mergeBlocks(blocks, maxWidths, maxHeight)
}

func splitAndMeasureBlocks(strs []string) ([][]string, []int, int) {
	blocks := make([][]string, len(strs))
	maxWidths := make([]int, len(strs))
	maxHeight := 0

	for idx, str := range strs {
		blocks[idx] = splitLines(str)
		for _, line := range blocks[idx] {
			w := CellWidth(line)
			if w > maxWidths[idx] {
				maxWidths[idx] = w
			}
		}

		if len(blocks[idx]) > maxHeight {
			maxHeight = len(blocks[idx])
		}
	}

	return blocks, maxWidths, maxHeight
}

func alignBlocks(blocks [][]string, pos Position, maxHeight int) {
	for idx := range blocks {
		if len(blocks[idx]) >= maxHeight {
			continue
		}

		extra := maxHeight - len(blocks[idx])

		switch pos {
		case Top:
			for range extra {
				blocks[idx] = append(blocks[idx], "")
			}
		case Bottom:
			padded := make([]string, extra, extra+len(blocks[idx]))
			blocks[idx] = append(padded, blocks[idx]...)
		default:
			split := int(math.Round(float64(extra) * float64(pos)))
			padded := make([]string, maxHeight)
			copy(padded[split:], blocks[idx])
			blocks[idx] = padded
		}
	}
}

func mergeBlocks(blocks [][]string, maxWidths []int, maxHeight int) string {
	var builder strings.Builder

	for row := range maxHeight {
		if row > 0 {
			builder.WriteByte('\n')
		}

		for col, block := range blocks {
			line := block[row]
			builder.WriteString(line)

			pad := maxWidths[col] - CellWidth(line)
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

	maxWidth := 0
	lines := make([][]string, len(strs))

	for i, str := range strs {
		lines[i] = splitLines(str)

		for _, line := range lines[i] {
			w := CellWidth(line)
			if w > maxWidth {
				maxWidth = w
			}
		}
	}

	var b strings.Builder

	for i, block := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}

		for _, line := range block {
			w := CellWidth(line)
			pad := maxWidth - w

			switch {
			case pos >= Right:
				if pad > 0 {
					if pad <= maxPadSpaces {
						b.WriteString(padSpaces[:pad])
					} else {
						b.WriteString(strings.Repeat(" ", pad))
					}
				}

				b.WriteString(line)
			case pos == Center:
				left := pad / 2
				right := pad - left

				if left > 0 {
					if left <= maxPadSpaces {
						b.WriteString(padSpaces[:left])
					} else {
						b.WriteString(strings.Repeat(" ", left))
					}
				}

				b.WriteString(line)

				if right > 0 {
					if right <= maxPadSpaces {
						b.WriteString(padSpaces[:right])
					} else {
						b.WriteString(strings.Repeat(" ", right))
					}
				}
			default:
				b.WriteString(line)

				if pad > 0 {
					if pad <= maxPadSpaces {
						b.WriteString(padSpaces[:pad])
					} else {
						b.WriteString(strings.Repeat(" ", pad))
					}
				}
			}

			b.WriteByte('\n')
		}
	}

	result := b.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result
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
