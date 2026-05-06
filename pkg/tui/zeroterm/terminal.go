// Based on charm.land/bubbletea/v2 — Copyright (c) 2020-2026 Charmbracelet, Inc.
// Licensed under the MIT License. See pkg/LICENSE for details.

package zeroterm

import (
	"os"

	"os/signal"
	"syscall"

	"github.com/pkg/errors"

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

func NewTerminal(inputFile, outputFile *os.File) (*Terminal, error) {
	tty := &Terminal{
		in:    inputFile,
		out:   outputFile,
		sigCh: make(chan os.Signal, 1),
	}

	//nolint:gosec // G115: safe — fd fits in int on all supported platforms
	termWidth, termHeight, err := term.GetSize(int(inputFile.Fd()))
	if err != nil {
		termWidth, termHeight, err = term.GetSize(int(outputFile.Fd())) //nolint:gosec // G115: safe — fd fits in int
		if err != nil {
			termWidth, termHeight = 80, 24
		}
	}

	tty.width = termWidth
	tty.height = termHeight

	return tty, nil
}

func (t *Terminal) Size() (int, int) {
	return t.width, t.height
}

func (t *Terminal) EnterRawMode() error {
	//nolint:gosec // G115: safe — fd fits in int on all supported platforms
	fd := int(t.in.Fd())

	state, err := term.MakeRaw(fd)
	if err != nil {
		return errors.Wrap(err, "terminal make raw")
	}

	t.oldState = state

	return nil
}

func (t *Terminal) ExitRawMode() {
	if t.oldState != nil {
		_ = term.Restore(int(t.in.Fd()), t.oldState) //nolint:gosec // G104/G115: best-effort restore
		t.oldState = nil
	}
}

func (t *Terminal) EnterAltScreen() {
	t.writeSeq("\x1b[?1049h")
}

func (t *Terminal) ExitAltScreen() {
	t.writeSeq("\x1b[?1049l")
}

func (t *Terminal) EnableMouse() {
	t.writeSeq("\x1b[?1000h\x1b[?1002h\x1b[?1006h")
}

func (t *Terminal) DisableMouse() {
	t.writeSeq("\x1b[?1000l\x1b[?1002l\x1b[?1006l")
}

func (t *Terminal) ShowCursor() {
	t.writeSeq("\x1b[?25h")
}

func (t *Terminal) HideCursor() {
	t.writeSeq("\x1b[?25l")
}

func (t *Terminal) Write(data []byte) (int, error) {
	n, err := t.out.Write(data)
	if err != nil {
		return n, errors.Wrap(err, "terminal write")
	}

	return n, nil
}

func (t *Terminal) Sync() {
	_ = t.out.Sync()
}

func (t *Terminal) WatchResize() <-chan os.Signal {
	signal.Notify(t.sigCh, syscall.SIGWINCH)

	return t.sigCh
}

func (t *Terminal) UpdateSize() (int, int) {
	//nolint:gosec // G115: safe — fd fits in int on all supported platforms
	termWidth, termHeight, err := term.GetSize(int(t.in.Fd()))
	if err != nil {
		termWidth, termHeight, err = term.GetSize(int(t.out.Fd())) //nolint:gosec // G115: safe
		if err != nil {
			return t.width, t.height
		}
	}

	t.width = termWidth
	t.height = termHeight

	return termWidth, termHeight
}

func (t *Terminal) StopWatchResize() {
	signal.Stop(t.sigCh)
}

// writeSeq writes an ANSI escape sequence and syncs the output. Errors are
// logged but not propagated — the terminal is unusable if these fail.
func (t *Terminal) writeSeq(seq string) {
	_, _ = t.out.WriteString(seq)
	_ = t.out.Sync()
}
