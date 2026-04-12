// Partially from: https://gist.github.com/arkan/5924e155dbb4254b64614069ba0afd81.

package atomicbuffer

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
)

// AtomicBuffer is a goroutine safe bytes.AtomicBuffer.
type AtomicBuffer struct {
	mutex  sync.Mutex
	buffer *bytes.Buffer
}

func New(buf []byte) *AtomicBuffer {
	return &AtomicBuffer{buffer: bytes.NewBuffer(buf)}
}

// Write appends the contents of p to the buffer, growing the buffer as needed. It returns
// the number of bytes written.
func (s *AtomicBuffer) Write(p []byte) (int, error) {
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
func (s *AtomicBuffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.String()
}

func (s *AtomicBuffer) Bytes() []byte {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Bytes()
}

// Len returns the number of accumulated bytes.
func (s *AtomicBuffer) Len() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Len()
}

func (s *AtomicBuffer) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *AtomicBuffer) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	s.Write([]byte(str))

	return nil
}
