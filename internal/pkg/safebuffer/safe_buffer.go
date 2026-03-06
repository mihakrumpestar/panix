// Partially from: https://gist.github.com/arkan/5924e155dbb4254b64614069ba0afd81

package safebuffer

import (
	"bytes"
	"sync"

	"github.com/pkg/errors"
)

// Buffer is a goroutine safe bytes.Buffer
type Buffer struct {
	mutex  sync.Mutex
	buffer *bytes.Buffer
}

func NewBuffer(buf []byte) *Buffer {
	return &Buffer{buffer: bytes.NewBuffer(buf)}
}

// Write appends the contents of p to the buffer, growing the buffer as needed. It returns
// the number of bytes written.
func (s *Buffer) Write(p []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	n, err := s.buffer.Write(p)
	if err != nil {
		return n, errors.Wrap(err, "failed to write to buffer")
	}

	return n, nil
}

// String returns the contents of the unread portion of the buffer
// as a string.  If the Buffer is a nil pointer, it returns "<nil>".
func (s *Buffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.String()
}

func (s *Buffer) Bytes() []byte {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Bytes()
}

// Len returns the number of accumulated bytes.
func (s *Buffer) Len() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Len()
}
