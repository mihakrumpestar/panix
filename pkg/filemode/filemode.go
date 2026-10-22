// Package filemode parses and formats octal file modes in the canonical
// four-digit form, for example "0700" or "1777".
package filemode

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
)

const (
	// Pattern matches the canonical non-zero four-digit octal form.
	Pattern = "^(?:[1-7][0-7]{3}|0[1-7][0-7]{2}|00[1-7][0-7]|000[1-7])$"

	// Default is the canonical default mode.
	Default = "0700"

	modeDigits = 4
)

var errInvalid = errors.New("invalid file mode")

// Parse parses a canonical non-zero four-digit octal mode.
func Parse(value string) (uint32, error) {
	if len(value) != modeDigits {
		return 0, invalid(value)
	}

	for index := range len(value) {
		if value[index] < '0' || value[index] > '7' {
			return 0, invalid(value)
		}
	}

	mode, err := strconv.ParseUint(value, 8, 32)
	if err != nil || mode == 0 {
		return 0, invalid(value)
	}

	return uint32(mode), nil
}

// Format renders a mode as four octal digits. Mode must be non-zero and at
// most 0o7777.
func Format(mode uint32) string {
	return fmt.Sprintf("%04o", mode)
}

func invalid(value string) error {
	return errors.Wrapf(errInvalid, "file mode %q (use four octal digits, e.g. %q)", value, Default)
}
