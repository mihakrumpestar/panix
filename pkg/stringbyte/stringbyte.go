//nolint:recvcheck // UnmarshalJSON needs a pointer, the other methods are value receivers
package stringbyte

import (
	"encoding/json"
	"unsafe"

	"github.com/pkg/errors"
)

// StringByte is a named string with zero-copy []byte access; comparability
// makes it usable as a map key.
type StringByte string

// Bytes returns the string's backing memory without allocating; the result is
// read-only, and modifying it corrupts the string.
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
