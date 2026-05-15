package style

import (
	"sync"

	"github.com/mihakrumpestar/panix/pkg/buffer"
)

// --- JoinScratch: pooled scratch arrays for join operations ---

type joinScratch struct {
	lines  [][]byte
	widths []int
	infos  []blockInfo
}

var joinScratchPool = sync.Pool{
	New: func() any {
		return &joinScratch{
			lines:  make([][]byte, 0, buffer.DefaultLinesBufLen),
			widths: make([]int, 0, buffer.DefaultLinesBufLen),
			infos:  make([]blockInfo, 0, 16), //nolint:mnd
		}
	},
}

func newJoinScratch() *joinScratch {
	return joinScratchPool.Get().(*joinScratch) //nolint:forcetypeassert
}

func (s *joinScratch) reset() {
	s.lines = s.lines[:0]
	s.widths = s.widths[:0]
	s.infos = s.infos[:0]
}

func (s *joinScratch) release() {
	s.reset()
	joinScratchPool.Put(s)
}

func (s *joinScratch) allocLines(n int) [][]byte {
	start := len(s.lines)
	end := start + n

	if end <= cap(s.lines) {
		s.lines = s.lines[:end]
	} else {
		newCap := cap(s.lines) * 2
		for newCap < end {
			newCap *= 2
		}

		newBuf := make([][]byte, end, newCap)
		copy(newBuf, s.lines)
		s.lines = newBuf
	}

	for i := start; i < end; i++ {
		s.lines[i] = nil
	}

	return s.lines[start:]
}

func (s *joinScratch) allocWidths(n int) []int {
	start := len(s.widths)
	end := start + n

	if end <= cap(s.widths) {
		s.widths = s.widths[:end]
	} else {
		newCap := cap(s.widths) * 2
		for newCap < end {
			newCap *= 2
		}

		newBuf := make([]int, end, newCap)
		copy(newBuf, s.widths)
		s.widths = newBuf
	}

	return s.widths[start:]
}

func (s *joinScratch) allocInfos(n int) []blockInfo {
	start := len(s.infos)
	end := start + n

	if end <= cap(s.infos) {
		s.infos = s.infos[:end]
	} else {
		newCap := cap(s.infos) * 2
		for newCap < end {
			newCap *= 2
		}

		newBuf := make([]blockInfo, end, newCap)
		copy(newBuf, s.infos)
		s.infos = newBuf
	}

	return s.infos[start:]
}
