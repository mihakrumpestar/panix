package style

import (
	"math"

	"github.com/mihakrumpestar/panix/pkg/buffer"
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

var padSpaces = make([]byte, maxPadSpaces)

func init() {
	for i := range padSpaces {
		padSpaces[i] = ' '
	}
}

type blockInfo struct {
	lines  [][]byte
	widths []int
	maxW   int
}

func splitBlock(data []byte) blockInfo {
	lines := splitLinesBytes(data)
	widths := make([]int, len(lines))
	maxW := 0

	for i, line := range lines {
		w := CellWidth(line)

		widths[i] = w
		if w > maxW {
			maxW = w
		}
	}

	return blockInfo{lines: lines, widths: widths, maxW: maxW}
}

func splitBlockScratch(s *joinScratch, data []byte) blockInfo {
	lines := splitLinesBytesScratch(s, data)
	widths := s.allocWidths(len(lines))
	maxW := 0

	for i, line := range lines {
		w := CellWidth(line)

		widths[i] = w
		if w > maxW {
			maxW = w
		}
	}

	return blockInfo{lines: lines, widths: widths, maxW: maxW}
}

func splitLinesBytesScratch(s *joinScratch, data []byte) [][]byte {
	if len(data) == 0 {
		lines := s.allocLines(1)
		lines[0] = nil

		return lines
	}

	n := 1

	for _, b := range data {
		if b == '\n' {
			n++
		}
	}

	lines := s.allocLines(n)
	start := 0
	idx := 0

	for i, b := range data {
		if b == '\n' {
			lines[idx] = data[start:i]
			start = i + 1
			idx++
		}
	}

	lines[idx] = data[start:]

	return lines
}

// JoinHorizontal joins potentially multi-line byte content along a vertical
// axis, aligned by position (Top, Center, Bottom). Each element of blocks
// is treated as a single block that may contain newlines. Output is written
// into buf.
func JoinHorizontal(buf *buffer.LinesBuf, pos Position, blocks ...[]byte) {
	buf.Reset()

	if len(blocks) == 0 {
		return
	}

	s := newJoinScratch()
	defer s.release()

	if len(blocks) == 1 {
		infos := splitBlockScratch(s, blocks[0])
		for _, line := range infos.lines {
			buf.WriteLine(line)
		}

		return
	}

	infos := s.allocInfos(len(blocks))
	maxHeight := 0

	for i, block := range blocks {
		infos[i] = splitBlockScratch(s, block)
		if len(infos[i].lines) > maxHeight {
			maxHeight = len(infos[i].lines)
		}
	}

	mergeBlocks(buf, infos, maxHeight, pos)
}

// JoinHorizontalBufs joins LinesBuf blocks along a vertical axis, aligned by
// position (Top, Center, Bottom). Each *LinesBuf is treated as a single block.
// Output is written into buf. This variant avoids newline scanning since
// LinesBuf already tracks line boundaries.
func JoinHorizontalBufs(buf *buffer.LinesBuf, pos Position, blocks ...*buffer.LinesBuf) {
	buf.Reset()

	if len(blocks) == 0 {
		return
	}

	if len(blocks) == 1 {
		buf.AppendFrom(blocks[0])

		return
	}

	s := newJoinScratch()
	defer s.release()

	infos := s.allocInfos(len(blocks))
	maxHeight := 0

	for i, block := range blocks {
		infos[i] = linesBufToBlockInfo(s, block)
		if len(infos[i].lines) > maxHeight {
			maxHeight = len(infos[i].lines)
		}
	}

	mergeBlocks(buf, infos, maxHeight, pos)
}

// linesBufToBlockInfo builds a blockInfo from a LinesBuf without scanning for
// newlines — lines and widths come directly from the LinesBuf's line index.
func linesBufToBlockInfo(s *joinScratch, lb *buffer.LinesBuf) blockInfo {
	n := lb.Len()
	lines := s.allocLines(n)
	widths := s.allocWidths(n)
	maxW := 0

	for i := range n {
		line := lb.Line(i)
		lines[i] = line

		w := CellWidth(line)

		widths[i] = w
		if w > maxW {
			maxW = w
		}
	}

	return blockInfo{lines: lines, widths: widths, maxW: maxW}
}

// JoinVertical joins byte content vertically, aligning them by position
// (Left, Center, Right). Each element of blocks is treated as a single
// block that may contain newlines. Output is written into buf.
func JoinVertical(buf *buffer.LinesBuf, pos Position, blocks ...[]byte) {
	buf.Reset()

	if len(blocks) == 0 {
		return
	}

	s := newJoinScratch()
	defer s.release()

	if len(blocks) == 1 {
		infos := splitBlockScratch(s, blocks[0])
		for _, line := range infos.lines {
			buf.WriteLine(line)
		}

		return
	}

	infos := s.allocInfos(len(blocks))
	maxWidth := 0

	for i, block := range blocks {
		infos[i] = splitBlockScratch(s, block)
		if infos[i].maxW > maxWidth {
			maxWidth = infos[i].maxW
		}
	}

	buildVerticalOutput(buf, infos, maxWidth, pos)
}

func rowToLineIdx(row, numLines, maxHeight int, pos Position) int {
	if numLines >= maxHeight {
		return row
	}

	extra := maxHeight - numLines

	switch pos {
	case Top:
		if row < numLines {
			return row
		}

		return -1
	case Bottom:
		idx := row - extra
		if idx >= 0 {
			return idx
		}

		return -1
	default:
		split := int(math.Round(float64(extra) * float64(pos)))
		idx := row - split

		if idx >= 0 && idx < numLines {
			return idx
		}

		return -1
	}
}

func mergeBlocks(buf *buffer.LinesBuf, infos []blockInfo, maxHeight int, pos Position) {
	for row := range maxHeight {
		for blockIdx, info := range infos {
			lineIdx := rowToLineIdx(row, len(info.lines), maxHeight, pos)

			if lineIdx >= 0 {
				pad := info.maxW - info.widths[lineIdx]
				if pad > 0 {
					if blockIdx == 0 {
						buf.WriteLine(info.lines[lineIdx], padSpaces[:min(pad, maxPadSpaces)])
					} else {
						buf.AppendToLine(info.lines[lineIdx], padSpaces[:min(pad, maxPadSpaces)])
					}
				} else {
					if blockIdx == 0 {
						buf.WriteLine(info.lines[lineIdx])
					} else {
						buf.AppendToLine(info.lines[lineIdx])
					}
				}
			} else {
				pad := padSpaces[:min(info.maxW, maxPadSpaces)]
				if blockIdx == 0 {
					buf.WriteLine(pad)
				} else {
					buf.AppendToLine(pad)
				}
			}
		}
	}
}

func buildVerticalOutput(buf *buffer.LinesBuf, infos []blockInfo, maxWidth int, pos Position) {
	for blockIdx, info := range infos {
		if blockIdx > 0 {
			buf.EmptyLine()
		}

		for i, line := range info.lines {
			pad := maxWidth - info.widths[i]

			switch {
			case pos >= Right:
				buf.WriteLine(padSpaces[:min(pad, maxPadSpaces)], line)
			case pos == Center:
				left := pad / 2 //nolint:mnd
				right := pad - left
				buf.WriteLine(padSpaces[:min(left, maxPadSpaces)], line, padSpaces[:min(right, maxPadSpaces)])
			default:
				buf.WriteLine(line, padSpaces[:min(pad, maxPadSpaces)])
			}
		}
	}
}

func splitLinesBytes(data []byte) [][]byte {
	if len(data) == 0 {
		return [][]byte{nil}
	}

	n := 1

	for _, b := range data {
		if b == '\n' {
			n++
		}
	}

	lines := make([][]byte, 0, n)
	start := 0

	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}

	lines = append(lines, data[start:])

	return lines
}
