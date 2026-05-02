package render

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

type Terminal struct {
	in       *os.File
	out      *os.File
	oldState *term.State
	sigCh    chan os.Signal
	width    int
	height   int
}

func NewTerminal(in, out *os.File) (*Terminal, error) {
	t := &Terminal{
		in:    in,
		out:   out,
		sigCh: make(chan os.Signal, 1),
	}

	w, h, err := term.GetSize(int(in.Fd()))
	if err != nil {
		w, h, err = term.GetSize(int(out.Fd()))
		if err != nil {
			w, h = 80, 24
		}
	}

	t.width = w
	t.height = h

	return t, nil
}

func (t *Terminal) Size() (int, int) {
	return t.width, t.height
}

func (t *Terminal) EnterRawMode() error {
	fd := int(t.in.Fd())

	state, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}

	t.oldState = state

	return nil
}

func (t *Terminal) ExitRawMode() {
	if t.oldState != nil {
		term.Restore(int(t.in.Fd()), t.oldState)
		t.oldState = nil
	}
}

func (t *Terminal) EnterAltScreen() {
	t.out.WriteString("\x1b[?1049h")
	t.out.Sync()
}

func (t *Terminal) ExitAltScreen() {
	t.out.WriteString("\x1b[?1049l")
	t.out.Sync()
}

func (t *Terminal) EnableMouse() {
	t.out.WriteString("\x1b[?1000h\x1b[?1006h\x1b[?1015h")
	t.out.Sync()
}

func (t *Terminal) DisableMouse() {
	t.out.WriteString("\x1b[?1000l\x1b[?1006l\x1b[?1015l")
	t.out.Sync()
}

func (t *Terminal) ShowCursor() {
	t.out.WriteString("\x1b[?25h")
	t.out.Sync()
}

func (t *Terminal) HideCursor() {
	t.out.WriteString("\x1b[?25l")
	t.out.Sync()
}

func (t *Terminal) Write(data []byte) (int, error) {
	return t.out.Write(data)
}

func (t *Terminal) Sync() {
	t.out.Sync()
}

func (t *Terminal) WatchResize() <-chan os.Signal {
	signal.Notify(t.sigCh, syscall.SIGWINCH)

	return t.sigCh
}

func (t *Terminal) UpdateSize() (int, int) {
	w, h, err := term.GetSize(int(t.in.Fd()))
	if err != nil {
		w, h, err = term.GetSize(int(t.out.Fd()))
		if err != nil {
			return t.width, t.height
		}
	}

	t.width = w
	t.height = h

	return w, h
}

func (t *Terminal) StopWatchResize() {
	signal.Stop(t.sigCh)
}
