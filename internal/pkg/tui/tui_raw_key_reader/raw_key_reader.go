package tui_raw_key_reader

import (
	"os"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

// RawKeyReaderMsg carries exactly the raw bytes read from stdin,
// *except* we filter out mouse sequences here.
type RawKeyReaderMsg []byte

// RawKeyReader wraps *os.File (stdin), tees every Read() into
// an internal channel (minus any mouse events), and still
// returns the bytes to Bubble Tea.
type RawKeyReader struct {
	*os.File
	ch chan RawKeyReaderMsg
}

// NewRawKeyReader creates a Reader over f with an internal buffer of size bufsize.
func NewRawKeyReader(f *os.File, bufsize int) *RawKeyReader {
	return &RawKeyReader{
		File: f,
		ch:   make(chan RawKeyReaderMsg, bufsize),
	}
}

// Read reads from the underlying stdin, copies the bytes into
// the channel (unless they look like a mouse seq), then returns
// them so Bubble Tea still sees the full stream.
func (r *RawKeyReader) Read(p []byte) (int, error) {
	n, err := r.File.Read(p)
	if n > 0 {
		buf := make([]byte, n)
		copy(buf, p[:n])
		if !isMouseEvent(buf) {
			select {
			case r.ch <- RawKeyReaderMsg(buf):
			default:
			}
		}
	}
	return n, err
}

// Fd lets Bubble Tea put the terminal into raw mode as usual.
func (r *RawKeyReader) Fd() uintptr {
	return r.File.Fd()
}

// Helpers

// next returns a tea.Cmd which will block on the next Msg
// and emit it to your Update.
func (r *RawKeyReader) Next() tea.Cmd {
	return func() tea.Msg {
		return <-r.ch
	}
}

// A simple detector for X10 (ESC [ M …) or SGR (ESC [ < … M/m) mouse sequences.
var (
	x10Prefix  = []byte{'\x1b', '[', 'M'}
	sgrMouseRe = regexp.MustCompile(`^\x1b\[[<][0-9;]+[Mm]`)
)

func isMouseEvent(b []byte) bool {
	// X10: ESC [ M <cb> <cx> <cy>
	if len(b) >= len(x10Prefix) && b[0] == 0x1b &&
		b[1] == '[' && b[2] == 'M' {
		return true
	}
	// SGR: ESC [ < params M or m
	return sgrMouseRe.Match(b)
}
