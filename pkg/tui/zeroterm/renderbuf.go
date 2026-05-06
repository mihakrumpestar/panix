package zeroterm

import (
	"strings"
)

// RenderBuffer stores lines as separate []byte buffers for optimal
// performance in typical TUI workloads (13% faster than contiguous).
// Each line maintains its own capacity for zero-allocation reuse.
//
// Lifecycle:
//   - Reset() clears content while preserving line buffers
//   - WriteLine / WriteString / WriteLines to add content
//   - Lines() to get the current frame's lines
//
// Lines are valid until the next Reset() on this buffer.
type RenderBuffer struct {
	lines [][]byte
}

// Reset clears all line content while preserving the underlying buffers.
// Outer slice is truncated but line capacities are preserved for reuse.
func (rb *RenderBuffer) Reset() {
	for i := range rb.lines {
		rb.lines[i] = rb.lines[i][:0]
	}

	rb.lines = rb.lines[:0]
}

// WriteLine appends a new line, reusing existing buffer if available.
func (rb *RenderBuffer) WriteLine(line []byte) {
	if len(rb.lines) < cap(rb.lines) {
		// Reuse existing line buffer
		idx := len(rb.lines)
		rb.lines = rb.lines[:idx+1]
		rb.lines[idx] = append(rb.lines[idx][:0], line...)
	} else {
		// Allocate new line buffer with extra capacity for growth
		// Use 2x the line length to amortize reallocations
		capacity := max(len(line)*2, 256) //nolint:mnd // Minimum reasonable line size

		newLine := make([]byte, 0, capacity)
		newLine = append(newLine, line...)
		rb.lines = append(rb.lines, newLine)
	}
}

// WriteLines appends multiple lines.
func (rb *RenderBuffer) WriteLines(lines [][]byte) {
	for _, line := range lines {
		rb.WriteLine(line)
	}
}

// WriteString splits s by \n and writes each line.
// Reuses existing line buffers when possible for zero allocation.
func (rb *RenderBuffer) WriteString(s string) {
	if s == "" {
		return
	}

	for line := range strings.SplitSeq(s, "\n") {
		rb.WriteLine([]byte(line))
	}
}

// Lines returns the current frame's lines.
// The returned slice shares the internal storage — callers must not modify it.
func (rb *RenderBuffer) Lines() [][]byte {
	return rb.lines
}
