// Package runtimevars expands PANIX_* runtime variables embedded in
// configuration values. Variables are referenced as $NAME or ${NAME}; every
// other byte (foreign $VAR references, bare $, malformed references) is passed
// through untouched.
package runtimevars

import (
	"strings"

	"github.com/pkg/errors"
)

// Panix runtime variable names. Only these may appear as PANIX_* references.
const (
	// Arch is the detected remote machine architecture (for example x86_64).
	Arch = "PANIX_ARCH"
	// SecretLocalPath is the local path a command-sourced secret was staged to.
	SecretLocalPath = "PANIX_SECRET_LOCAL_PATH" //nolint:gosec // variable name, not a credential
)

// panixPrefix marks PANIX_* names owned by panix; anything else is a typo and a hard error.
const panixPrefix = "PANIX_"

// Vars holds the values available for expansion; a known variable absent from the map is a hard error.
type Vars map[string]string

// Expand replaces $NAME / ${NAME} references to known panix runtime variables
// with their values, passing foreign or malformed references through byte for
// byte. Known-but-absent and unknown PANIX_* names error and return "".
func (v Vars) Expand(input string) (string, error) {
	var out strings.Builder

	out.Grow(len(input))

	for pos := 0; pos < len(input); {
		if input[pos] != '$' {
			out.WriteByte(input[pos])

			pos++

			continue
		}

		name, width := reference(input[pos:])
		if width == 0 {
			out.WriteByte(input[pos])

			pos++

			continue
		}

		switch name {
		case Arch, SecretLocalPath:
			value, present := v[name]
			if !present {
				return "", errors.Errorf("panix runtime variable %s is not available in this context", name)
			}

			out.WriteString(value)
		default:
			if strings.HasPrefix(name, panixPrefix) {
				return "", errors.Errorf("unknown panix runtime variable %s", name)
			}

			out.WriteString(input[pos : pos+width])
		}

		pos += width
	}

	return out.String(), nil
}

// reference parses a $NAME or ${NAME} reference at the start of input,
// returning its name and byte width; width 0 means pass the input through.
func reference(input string) (string, int) {
	if len(input) < 2 || input[0] != '$' {
		return "", 0
	}

	if input[1] == '{' {
		end := strings.IndexByte(input[2:], '}')
		if end < 0 {
			return "", 0
		}

		name := input[2 : 2+end]
		if !isName(name) {
			return "", 0
		}

		return name, 2 + end + 1
	}

	if !isNameStart(input[1]) {
		return "", 0
	}

	end := 2
	for end < len(input) && isNameByte(input[end]) {
		end++
	}

	return input[1:end], end
}

// isName reports whether name matches [A-Za-z_][A-Za-z0-9_]*.
func isName(name string) bool {
	if name == "" || !isNameStart(name[0]) {
		return false
	}

	for idx := 1; idx < len(name); idx++ {
		if !isNameByte(name[idx]) {
			return false
		}
	}

	return true
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameByte(c byte) bool {
	return isNameStart(c) || (c >= '0' && c <= '9')
}
