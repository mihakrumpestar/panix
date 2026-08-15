package buffer

import (
	"encoding/json"

	"github.com/pkg/errors"
)

const DefaultLineBufLen = 256

// LineBuf is a pooled single-line buffer. Set copies data in.
// Width is computed lazily, call CellWidth(Bytes()) only when needed.
type LineBuf struct {
	buf []byte
}

func NewLineBuf() *LineBuf {
	return &LineBuf{buf: make([]byte, 0, DefaultLineBufLen)}
}

func (r *LineBuf) Set(data []byte) {
	if len(data) <= cap(r.buf) {
		r.buf = append(r.buf[:0], data...)
	} else {
		newCap := max(cap(r.buf)*2, DefaultLineBufLen)
		for newCap < len(data) {
			newCap *= 2
		}

		r.buf = make([]byte, len(data), newCap)
		copy(r.buf, data)
	}
}

func (r *LineBuf) Write(p []byte) {
	r.buf = append(r.buf, p...)
}

func (r *LineBuf) WriteByte(b byte) { //nolint:govet
	r.buf = append(r.buf, b)
}

func (r *LineBuf) WriteString(s string) {
	r.buf = append(r.buf, s...)
}

func (r *LineBuf) Bytes() []byte {
	return r.buf
}

func (r *LineBuf) String() string {
	return string(r.buf)
}

func (r *LineBuf) Len() int {
	if r == nil {
		return 0
	}

	return len(r.buf)
}

func (r *LineBuf) Reset() {
	r.buf = r.buf[:0]
}

// MarshalJSON implements json.Marshaler.
func (r *LineBuf) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String()) //nolint:wrapcheck // JSON serialization
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *LineBuf) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err != nil {
		return errors.Wrap(err, "unmarshal LineBuf")
	}

	r.Set([]byte(str))

	return nil
}
