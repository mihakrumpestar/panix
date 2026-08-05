package buffer

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/pkg/no"
)

// LinesBufVer is a versioned, thread-safe wrapper around LinesBuf.
// It tracks a version counter incremented on every mutation for cache
// invalidation. Create with NewLinesBufVer.
//
// The inner LinesBuf is unexported to prevent accidental unsynchronized
// access. Use the locked methods (Write, Line, Len, etc.) for thread-safe
// access, or Snapshot() for a consistent copy.
type LinesBufVer struct {
	inner *LinesBuf

	version uint64
	mu      sync.Mutex

	_ no.Copy
}

// NewLinesBufVer creates a new LinesBufVer.
func NewLinesBufVer() *LinesBufVer {
	return &LinesBufVer{
		inner: NewLinesBuf(),
	}
}

// SetMaxLines sets the trim threshold. 0 means unlimited.
// Trimming happens in batches (10% of maxLines) to amortise cache rebuilds.
func (b *LinesBufVer) SetMaxLines(n uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.maxLines = n
}

// LineOffset returns the cumulative count of lines trimmed from the front.
func (b *LinesBufVer) LineOffset() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.inner.lineOffset
}

// Append appends bytes to the current (last) line. If no lines exist yet,
// a new line is started.
func (b *LinesBufVer) Append(buf []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.appendLocked(buf)
}

// AppendString is a string version of Append.
func (b *LinesBufVer) AppendString(s string) {
	b.Append([]byte(s))
}

// Write writes a new line. The line data is copied into the contiguous buffer
// so the caller can safely reuse the source slice.
func (b *LinesBufVer) Write(line []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.WriteLine(line)
	b.version++
	b.maybeTrimLocked()
}

// WriteString writes a new line from a string.
func (b *LinesBufVer) WriteString(line string) {
	b.Write([]byte(line))
}

// WriteLines appends multiple lines.
func (b *LinesBufVer) WriteLines(lines [][]byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, line := range lines {
		b.inner.WriteLine(line)
	}

	b.version++
	b.maybeTrimLocked()
}

// OverrideLastLine rewrites the last line in-place.
func (b *LinesBufVer) OverrideLastLine(line []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.OverrideLastLine(line)
	b.version++
}

// RemoveLastLine removes the last line. No-op if empty.
func (b *LinesBufVer) RemoveLastLine() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.RemoveLastLine()
	b.version++
}

// Line returns the content of line i, or nil if out of bounds.
// This overrides LinesBuf.Line to add bounds checking.
func (b *LinesBufVer) Line(i int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.lineLocked(i)
}

// LastLine returns the content of the last line, or nil if empty.
// This overrides LinesBuf.LastLine for atomic support.
func (b *LinesBufVer) LastLine() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := b.inner.Len()
	if n == 0 {
		return nil
	}

	return b.inner.Line(n - 1)
}

// Len returns the number of lines. Thread-safe.
func (b *LinesBufVer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.lenLocked()
}

// Version returns the current version counter.
func (b *LinesBufVer) Version() uint64 {
	if b == nil {
		return 0
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	return b.version
}

// Lines returns the lines as [][]byte.
// Allocates, prefer Line(i) when possible.
func (b *LinesBufVer) Lines() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.inner.Lines()
}

// String returns all lines joined by newlines.
func (b *LinesBufVer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	length := b.lenLocked()
	if length == 0 {
		return ""
	}

	size := len(b.inner.buf) + length - 1
	out := make([]byte, 0, size)

	for i := range length {
		if i > 0 {
			out = append(out, '\n')
		}

		out = append(out, b.lineLocked(i)...)
	}

	return string(out)
}

// Bytes returns all lines joined by newlines as a byte slice.
func (b *LinesBufVer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.bytesLocked()
}

// Reset clears all content. maxLines is preserved.
func (b *LinesBufVer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.Reset()
	b.version++
}

// CopyFrom replaces content with a copy of src under the lock.
// maxLines is preserved.
func (b *LinesBufVer) CopyFrom(src *LinesBuf) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.inner.Reset()
	b.inner.CopyFrom(src)
	b.version++
}

// Snapshot returns a copy of the inner LinesBuf under the lock.
// The caller must call Release() when done. Carries lineOffset but not maxLines.
func (b *LinesBufVer) Snapshot() *LinesBuf {
	b.mu.Lock()
	defer b.mu.Unlock()

	snap := NewLinesBuf()
	snap.CopyFrom(b.inner)

	return snap
}

// JSON

// MarshalJSON implements json.Marshaler.
func (b *LinesBufVer) MarshalJSON() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lines := make([]string, b.lenLocked())
	for i := range b.lenLocked() {
		lines[i] = string(b.lineLocked(i))
	}

	return json.Marshal(lines) //nolint:wrapcheck // JSON serialization has no actionable wrapping
}

// UnmarshalJSON implements json.Unmarshaler.
func (b *LinesBufVer) UnmarshalJSON(data []byte) error {
	var lines []string

	err := json.Unmarshal(data, &lines)
	if err != nil {
		return fmt.Errorf("unmarshal lines: %w", err)
	}

	b.inner = NewLinesBuf()

	for _, line := range lines {
		b.Write([]byte(line))
	}

	return nil
}

// Locked helpers, called when mu is already held. Use LinesBuf methods
// directly to avoid re-locking.

func (b *LinesBufVer) lenLocked() int {
	return b.inner.Len()
}

func (b *LinesBufVer) lineLocked(i int) []byte {
	if i < 0 || i >= b.lenLocked() {
		return nil
	}

	return b.inner.Line(i)
}

func (b *LinesBufVer) bytesLocked() []byte {
	length := b.lenLocked()
	if length == 0 {
		return nil
	}

	size := len(b.inner.buf) + length - 1
	result := make([]byte, 0, size)

	for i := range length {
		if i > 0 {
			result = append(result, '\n')
		}

		result = append(result, b.inner.Line(i)...)
	}

	return result
}

func (b *LinesBufVer) appendLocked(buf []byte) {
	if b.lenLocked() == 0 {
		b.inner.WriteLine(buf)
	} else {
		b.inner.AppendToLine(buf)
	}
}

// maybeTrimLocked trims excess lines from the front when len > maxLines.
// Trims a batch (10% of maxLines, min 1) to avoid invalidating caches
// on every write. Does not bump version: the caller already did.
func (b *LinesBufVer) maybeTrimLocked() {
	if b.inner.maxLines == 0 {
		return
	}

	current := uint64(b.lenLocked()) //nolint:gosec // G115: line count is always non-negative
	if current <= b.inner.maxLines {
		return
	}

	excess := current - b.inner.maxLines

	// Trim at least 10% of maxLines to avoid trimming on every write.
	batchSize := max(b.inner.maxLines/10, 1) //nolint:mnd

	trimCount := max(batchSize, excess)

	b.inner.TrimFront(int(trimCount)) //nolint:gosec // G115: trimCount originated from int line count
}
