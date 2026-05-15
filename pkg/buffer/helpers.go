package buffer

import "strings"

// LinesBufToStringForTests joins the buffer lines with \n into a string.
// Intended for test assertions only — not for production hot paths.
func LinesBufToStringForTests(linesBuf *LinesBuf) string {
	length := len(linesBuf.indexes)
	if length == 0 {
		return ""
	}

	size := len(linesBuf.buf) + length - 1

	var builder strings.Builder
	builder.Grow(size)

	for i := range length {
		if i > 0 {
			builder.WriteByte('\n')
		}

		builder.Write(linesBuf.Line(i))
	}

	return builder.String()
}
