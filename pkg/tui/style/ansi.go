package style

import (
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

var (
	ansiReset      = []byte("\x1b[m")
	ansiBold       = []byte("\x1b[1m")
	ansiForeground = []byte("\x1b[38;2;")
	ansiBackground = []byte("\x1b[48;2;")
)

type ansiStyle struct {
	prefix []byte
	reset  []byte
}

func newANSIStyle(s Style) ansiStyle {
	prefix := s.stylePrefix()

	return ansiStyle{prefix: prefix, reset: ansiReset}
}

func (a ansiStyle) render(buf *buffer.LinesBuf, content [][]byte) {
	buf.Reset()

	if a.prefix == nil || content == nil {
		for range content {
			buf.EmptyLine()
		}

		return
	}

	for _, line := range content {
		buf.WriteLine(a.prefix, line, a.reset)
	}
}
