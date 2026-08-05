package stringbyte

import (
	"encoding/json"
	"unsafe"

	"github.com/pkg/errors"
)

// StringByte is a string that provides zero-copy []byte access.
// It is comparable (can be used as map key, supports ==/!=) because it is
// a named string type. Bytes() returns a read-only view of the string's
// backing memory via unsafe; do NOT modify the returned slice.
//
//nolint:recvcheck // intentional: MarshalJSON on value, UnmarshalJSON on pointer
type StringByte string

// Bytes returns the underlying byte slice without allocation.
// The returned slice shares the string's backing memory; do NOT modify it.
//
//nolint:gosec // G103: intentional use of unsafe for zero-copy string→[]byte
func (sb StringByte) Bytes() []byte {
	if len(sb) == 0 {
		return nil
	}

	return unsafe.Slice(unsafe.StringData(string(sb)), len(sb))
}

// String returns the string representation.
func (sb StringByte) String() string {
	return string(sb)
}

// MarshalJSON serializes StringByte as a JSON string.
func (sb StringByte) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(string(sb))

	return b, errors.Wrap(err, "marshal StringByte")
}

// UnmarshalJSON deserializes from a JSON string.
func (sb *StringByte) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err != nil {
		return errors.Wrap(err, "unmarshal StringByte")
	}

	*sb = StringByte(str)

	return nil
}
