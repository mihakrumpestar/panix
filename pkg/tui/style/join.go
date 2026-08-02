package style

import (
	"bytes"
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

var padSpaces = bytes.Repeat([]byte(" "), maxPadSpaces)

type blockInfo struct {
	lines  [][]byte
	widths []int
	maxW   int
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

func splitLinesBytesScratch(joinScratch *joinScratch, data []byte) [][]byte {
	if len(data) == 0 {
		lines := joinScratch.allocLines(1)
		lines[0] = nil

		return lines
	}

	lineCount := 1

	for _, b := range data {
		if b == '\n' {
			lineCount++
		}
	}

	lines := joinScratch.allocLines(lineCount)
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

	joinScratch := newJoinScratch()
	defer joinScratch.release()

	if len(blocks) == 1 {
		infos := splitBlockScratch(joinScratch, blocks[0])
		for _, line := range infos.lines {
			buf.WriteLine(line)
		}

		return
	}

	infos := joinScratch.allocInfos(len(blocks))
	maxHeight := 0

	for i, block := range blocks {
		infos[i] = splitBlockScratch(joinScratch, block)
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

	joinScratch := newJoinScratch()
	defer joinScratch.release()

	infos := joinScratch.allocInfos(len(blocks))
	maxHeight := 0

	for i, block := range blocks {
		infos[i] = linesBufToBlockInfo(joinScratch, block)
		if len(infos[i].lines) > maxHeight {
			maxHeight = len(infos[i].lines)
		}
	}

	mergeBlocks(buf, infos, maxHeight, pos)
}

// linesBufToBlockInfo builds a blockInfo from a LinesBuf without scanning for
// newlines; lines and widths come directly from the LinesBuf's line index.
func linesBufToBlockInfo(s *joinScratch, buf *buffer.LinesBuf) blockInfo {
	n := buf.Len()
	lines := s.allocLines(n)
	widths := s.allocWidths(n)
	maxW := 0

	for idx := range n {
		line := buf.Line(idx)
		lines[idx] = line

		w := CellWidth(line)

		widths[idx] = w
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

	joinStratch := newJoinScratch()
	defer joinStratch.release()

	if len(blocks) == 1 {
		infos := splitBlockScratch(joinStratch, blocks[0])
		for _, line := range infos.lines {
			buf.WriteLine(line)
		}

		return
	}

	infos := joinStratch.allocInfos(len(blocks))
	maxWidth := 0

	for i, block := range blocks {
		infos[i] = splitBlockScratch(joinStratch, block)
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
				writeBlockLine(buf, blockIdx, info.lines[lineIdx], info.maxW-info.widths[lineIdx])
			} else {
				writeBlockLine(buf, blockIdx, nil, info.maxW)
			}
		}
	}
}

func writeBlockLine(buf *buffer.LinesBuf, blockIdx int, line []byte, pad int) {
	switch {
	case line == nil:
		p := padSpaces[:min(pad, maxPadSpaces)]
		if blockIdx == 0 {
			buf.WriteLine(p)
		} else {
			buf.AppendToLine(p)
		}
	case pad > 0:
		p := padSpaces[:min(pad, maxPadSpaces)]
		if blockIdx == 0 {
			buf.WriteLine(line, p)
		} else {
			buf.AppendToLine(line, p)
		}
	default:
		if blockIdx == 0 {
			buf.WriteLine(line)
		} else {
			buf.AppendToLine(line)
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
				left := pad / 2
				right := pad - left
				buf.WriteLine(padSpaces[:min(left, maxPadSpaces)], line, padSpaces[:min(right, maxPadSpaces)])
			default:
				buf.WriteLine(line, padSpaces[:min(pad, maxPadSpaces)])
			}
		}
	}
}
