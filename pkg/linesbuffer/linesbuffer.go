// Package linesbuffer provides a high-performance line buffer for TUI rendering.
//
// LinesBuffer stores line data in a contiguous []byte with offset/length
// references, enabling zero-allocation diffs between frames. A pre-allocated
// diffs slice is reused across Diff calls.
//
// Three construction modes:
//   - New():       direct construction (long-lived buffers)
//   - NewPooled(): from a sync.Pool (short-lived, intermediate buffers)
//   - NewAtomic(): mutex-guarded for concurrent read/write access
package linesbuffer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mihakrumpestar/panix/pkg/no"
)

const (
	defaultDataSize  = 16 * 1024 // 16KB initial data capacity
	defaultLineCount = 128       // initial line/ref/diff capacity
)

var defaultPool = sync.Pool{
	New: func() any {
		return New()
	},
}

// lineRef stores the offset and length of a line within the contiguous data buffer.
type lineRef struct {
	off, len_ uint64
}

// LinesBuffer stores lines as contiguous byte data with offset/length references.
// It supports zero-allocation frame diffs via a pre-allocated diffs slice.
//
// Lifecycle:
//   - Reset() clears content while preserving underlying buffers
//   - Write/WriteLines to add content
//   - Diff(old) to compare against a previous frame
//   - Line(i)/Len() to access line data
//
// Line data returned by Line() is valid until the next Reset on this buffer.
// Diff results are valid until the next Diff or Reset call.
type LinesBuffer struct {
	_ no.Copy

	data    []byte
	refs    []lineRef
	diffs   []int // Pre allocated slice of diff lines, for performance and zero-alloc
	version uint64

	mu     sync.Mutex
	atomic bool
	pooled bool
}

// New creates a new LinesBuffer suitable for long-lived use.
// Not pooled, not thread-safe.
func New() *LinesBuffer {
	return &LinesBuffer{
		data:  make([]byte, 0, defaultDataSize),
		refs:  make([]lineRef, 0, defaultLineCount),
		diffs: make([]int, 0, defaultLineCount),
	}
}

// NewPooled creates a LinesBuffer from a shared pool.
// The buffer is reset before being returned. Call Release() when done
// to return it to the pool. Not thread-safe.
func NewPooled() *LinesBuffer {
	b := defaultPool.Get().(*LinesBuffer) //nolint:forcetypeassert // Safe
	b.Reset()
	b.pooled = true

	return b
}

// NewAtomic creates a mutex-guarded LinesBuffer for concurrent read/write access.
// Not pooled; for long-lived use where goroutines may read and write simultaneously.
func NewAtomic() *LinesBuffer {
	return &LinesBuffer{
		data:   make([]byte, 0, defaultDataSize),
		refs:   make([]lineRef, 0, defaultLineCount),
		diffs:  make([]int, 0, defaultLineCount),
		atomic: true,
	}
}

// Release returns a pooled LinesBuffer to the pool.
// No-op for non-pooled buffers. Only call this on buffers from NewPooled().
func (b *LinesBuffer) Release() {
	if b.pooled {
		b.pooled = false
		defaultPool.Put(b)
	}
}

// Write appends a line to the buffer.
func (b *LinesBuffer) Write(line []byte) {
	if b.atomic {
		b.mu.Lock()
	}

	off := uint64(len(b.data))
	b.data = append(b.data, line...)
	b.refs = append(b.refs, lineRef{off, uint64(len(line))})

	b.version++
	if b.atomic {
		b.mu.Unlock()
	}
}

// WriteLines appends multiple lines.
func (b *LinesBuffer) WriteLines(lines [][]byte) {
	if b.atomic {
		b.mu.Lock()
	}

	for _, line := range lines {
		off := uint64(len(b.data))
		b.data = append(b.data, line...)
		b.refs = append(b.refs, lineRef{off, uint64(len(line))})
	}

	b.version++

	if b.atomic {
		b.mu.Unlock()
	}
}

// WriteString splits s by \n and writes each line.
func (b *LinesBuffer) WriteString(s string) {
	if s == "" {
		return
	}

	for line := range strings.SplitSeq(s, "\n") {
		b.Write([]byte(line))
	}
}

// OverrideLastLine rewrites the last line in-place by truncating the data
// buffer back to the last line's start and re-appending.
func (b *LinesBuffer) OverrideLastLine(line []byte) {
	if b.atomic {
		b.mu.Lock()
	}

	if len(b.refs) == 0 {
		if b.atomic {
			b.mu.Unlock()
		}

		b.Write(line)

		return
	}

	last := &b.refs[len(b.refs)-1]
	b.data = b.data[:last.off]
	b.data = append(b.data, line...)
	last.len_ = uint64(len(line))
	b.version++

	if b.atomic {
		b.mu.Unlock()
	}
}

// Reset clears all content while preserving underlying buffer capacity.
func (b *LinesBuffer) Reset() {
	if b.atomic {
		b.mu.Lock()
	}

	b.data = b.data[:0]
	b.refs = b.refs[:0]

	b.version++
	if b.atomic {
		b.mu.Unlock()
	}
}

// Diff compares this buffer against old and returns indices of changed lines.
// The returned slice shares internal storage — it is valid only until the
// next Diff or Reset call on this buffer. Zero allocations per call.
func (b *LinesBuffer) Diff(old *LinesBuffer) []int {
	if b.atomic {
		b.mu.Lock()
		old.mu.Lock()
		defer b.mu.Unlock()
		defer old.mu.Unlock()
	}

	commonLen := min(len(b.refs), len(old.refs))

	b.diffs = b.diffs[:0]
	for idx := range commonLen {
		nr, or_ := b.refs[idx], old.refs[idx]
		if nr.len_ == or_.len_ &&
			bytes.Equal(b.data[nr.off:nr.off+nr.len_], old.data[or_.off:or_.off+or_.len_]) {
			continue
		}

		b.diffs = append(b.diffs, idx)
	}

	for idx := commonLen; idx < len(b.refs); idx++ {
		b.diffs = append(b.diffs, idx)
	}

	return b.diffs
}

// Line returns the byte content of line i. The returned slice is a sub-slice
// of the internal data buffer — callers must not modify it, and it is valid
// only until the next Reset.
func (b *LinesBuffer) Line(i int) []byte {
	if i < 0 || i >= len(b.refs) {
		return nil
	}

	r := b.refs[i]

	return b.data[r.off : r.off+r.len_]
}

// Len returns the number of lines in the buffer.
func (b *LinesBuffer) Len() int {
	return len(b.refs)
}

// LastLine returns the content of the last line, or nil if the buffer is empty.
func (b *LinesBuffer) LastLine() []byte {
	if len(b.refs) == 0 {
		return nil
	}

	r := b.refs[len(b.refs)-1]

	return b.data[r.off : r.off+r.len_]
}

// RemoveLastLine removes the last line from the buffer.
// No-op if the buffer is empty.
func (b *LinesBuffer) RemoveLastLine() {
	if b.atomic {
		b.mu.Lock()
	}

	if len(b.refs) > 0 {
		b.refs = b.refs[:len(b.refs)-1]
		b.version++
	}

	if b.atomic {
		b.mu.Unlock()
	}
}

// Version returns the current version counter, incremented on every Write,
// WriteLines, OverrideLastLine, and Reset call. Callers can detect whether
// the buffer has changed since a previous observation.
func (b *LinesBuffer) Version() uint64 {
	return b.version
}

// String returns all lines joined by newlines.
// Returns "" if the buffer is empty.
func (b *LinesBuffer) String() string {
	return string(b.Bytes())
}

// Bytes returns all lines joined by newlines as a byte slice.
// Returns nil if the buffer is empty.
func (b *LinesBuffer) Bytes() []byte {
	if len(b.refs) == 0 {
		return nil
	}

	size := 0
	for _, ref := range b.refs {
		size += int(ref.len_) + 1 //nolint:gosec // G115: ref.len_ bounded by data allocation
	}

	result := make([]byte, 0, size)

	for i, ref := range b.refs {
		if i > 0 {
			result = append(result, '\n')
		}

		result = append(result, b.data[ref.off:ref.off+ref.len_]...)
	}

	return result
}

// MarshalJSON implements json.Marshaler. The buffer is serialized as a
// JSON array of strings, one per line.
func (b *LinesBuffer) MarshalJSON() ([]byte, error) {
	lines := make([]string, len(b.refs))
	for i, ref := range b.refs {
		lines[i] = string(b.data[ref.off : ref.off+ref.len_])
	}

	return json.Marshal(lines) //nolint:wrapcheck // JSON serialization has no actionable wrapping
}

// UnmarshalJSON implements json.Unmarshaler. The buffer is deserialized from
// a JSON array of strings, one per line. The buffer is Reset before loading.
func (b *LinesBuffer) UnmarshalJSON(data []byte) error {
	var lines []string

	err := json.Unmarshal(data, &lines)
	if err != nil {
		return fmt.Errorf("unmarshal lines: %w", err)
	}

	b.Reset()

	for _, line := range lines {
		b.Write([]byte(line))
	}

	return nil
}
