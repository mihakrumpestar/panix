// Package osrelease reads and parses os-release files per the freedesktop.org spec.
// See https://www.freedesktop.org/software/systemd/man/os-release.html
//
// Based on github.com/acobaugh/osrelease (BSD-2-Clause, Copyright 2017 Andrew Cobaugh).
package osrelease

import (
	"bytes"
	"os"

	"github.com/pkg/errors"
)

const (
	// EtcOsRelease is the primary os-release file path per freedesktop.org spec.
	EtcOsRelease = "/etc/os-release"
	// UsrLibOsRelease is the fallback os-release file path.
	UsrLibOsRelease = "/usr/lib/os-release"
)

// Read reads os-release, trying /etc/os-release then /usr/lib/os-release.
// Returns a map of key-value pairs. Returns an error only if neither file exists
// or the file cannot be read.
func Read() (map[string]string, error) {
	result, err := ReadFile(EtcOsRelease)
	if err != nil {
		return ReadFile(UsrLibOsRelease)
	}

	return result, nil
}

// ReadFile reads and parses the os-release file at the given path.
func ReadFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is caller-provided by design
	if err != nil {
		return nil, errors.Wrap(err, "read os-release file")
	}

	return Parse(data), nil
}

// ReadString parses os-release content from a string and returns key-value pairs.
func ReadString(content string) (map[string]string, error) {
	return Parse([]byte(content)), nil
}

// Parse parses os-release content from bytes and returns key-value pairs.
// Lines beginning with '#' and blank lines are skipped.
// Shell-style escaping is handled per the freedesktop.org spec:
// double-quoted and unquoted values have \", \$, \\, \` unescaped;
// single-quoted values are taken literally (no escape expansion).
func Parse(data []byte) map[string]string {
	result := make(map[string]string)

	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		key, val, ok := parseLine(line)
		if ok {
			result[key] = val
		}
	}

	return result
}

// parseLine parses a single line into key, value. Returns ok=false for comments and blank lines.
func parseLine(line []byte) (string, string, bool) {
	line = bytes.TrimLeft(line, " \t")
	if len(line) == 0 || line[0] == '#' {
		return "", "", false
	}

	before, after, ok := bytes.Cut(line, []byte{'='})
	if !ok {
		return "", "", false
	}

	key := string(bytes.TrimRight(before, " \t"))
	val := after

	val = bytes.Trim(val, " \t")

	if len(val) == 0 {
		return key, "", true
	}

	quote := val[0]
	if (quote == '"' || quote == '\'') && len(val) >= 2 && val[len(val)-1] == quote {
		val = val[1 : len(val)-1]
		if quote == '\'' {
			return key, string(val), true
		}

		return key, unescape(val), true
	}

	return key, unescape(val), true
}

// unescape expands shell escape sequences: \", \$, \\, \`
// Operates on []byte to minimize allocations, only allocates on the first
// escaped character found.
func unescape(data []byte) string {
	if bytes.IndexByte(data, '\\') < 0 {
		return string(data)
	}

	var buf []byte

	idx := 0
	for idx < len(data) {
		if data[idx] == '\\' && idx+1 < len(data) {
			next := data[idx+1]
			switch next {
			case '"', '$', '\\', '`':
				if buf == nil {
					buf = make([]byte, 0, len(data))
					buf = append(buf, data[:idx]...)
				}

				buf = append(buf, next)
				idx += 2

				continue
			}
		}

		if buf != nil {
			buf = append(buf, data[idx])
		}

		idx++
	}

	if buf != nil {
		return string(buf)
	}

	return string(data)
}
