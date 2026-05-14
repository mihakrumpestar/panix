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
type LinesBufVer struct {
	*LinesBuf

	version uint64
	mu      sync.Mutex

	_ no.Copy
}

// NewLinesBufVer creates a new LinesBufVer.
func NewLinesBufVer() *LinesBufVer {
	return &LinesBufVer{
		LinesBuf: NewLinesBuf(),
	}
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

	b.WriteLine(line)
	b.version++
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
		b.WriteLine(line)
	}

	b.version++
}

// OverrideLastLine rewrites the last line in-place.
func (b *LinesBufVer) OverrideLastLine(line []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.LinesBuf.OverrideLastLine(line)
	b.version++
}

// RemoveLastLine removes the last line. No-op if empty.
func (b *LinesBufVer) RemoveLastLine() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.LinesBuf.RemoveLastLine()
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

	n := b.LinesBuf.Len()
	if n == 0 {
		return nil
	}

	return b.LinesBuf.Line(n - 1)
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
// Allocates — prefer Line(i) when possible.
func (b *LinesBufVer) Lines() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.LinesBuf.Lines()
}

// String returns all lines joined by newlines.
func (b *LinesBufVer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	length := b.lenLocked()
	if length == 0 {
		return ""
	}

	size := len(b.LinesBuf.buf) + length - 1
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

// Reset clears all content while preserving underlying capacity.
func (b *LinesBufVer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.LinesBuf.Reset()
	b.version++
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

	b.Reset()

	for _, line := range lines {
		b.WriteLine([]byte(line))
	}

	return nil
}

// Locked helpers — called when mu is already held. Use LinesBuf methods
// directly to avoid re-locking.

func (b *LinesBufVer) lenLocked() int {
	return b.LinesBuf.Len()
}

func (b *LinesBufVer) lineLocked(i int) []byte {
	if i < 0 || i >= b.lenLocked() {
		return nil
	}

	return b.LinesBuf.Line(i)
}

func (b *LinesBufVer) bytesLocked() []byte {
	length := b.lenLocked()
	if length == 0 {
		return nil
	}

	size := len(b.LinesBuf.buf) + length - 1
	result := make([]byte, 0, size)

	for i := range length {
		if i > 0 {
			result = append(result, '\n')
		}

		result = append(result, b.LinesBuf.Line(i)...)
	}

	return result
}

func (b *LinesBufVer) appendLocked(buf []byte) {
	if b.lenLocked() == 0 {
		b.WriteLine(buf)
	} else {
		b.AppendToLine(buf)
	}
}
