//go:build !linux && !darwin && !freebsd

package pty

import (
	"io"
	"os"
	"os/exec"
)

// Winsize represents the terminal window size on unsupported platforms.
type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// termiosState is the stand-in for the platform termios type on unsupported
// platforms; it only carries the local flags touched by SetEcho.
type termiosState struct {
	Lflag uint64
}

// Read on unsupported platforms always returns EOF.
func (p *Pty) Read(b []byte) (int, error) {
	return 0, io.EOF
}

// Resize on unsupported platforms always returns ErrUnsupported.
func (p *Pty) Resize(w, h int) error {
	return ErrUnsupported
}

// SetWinsize on unsupported platforms always returns ErrUnsupported.
func (p *Pty) SetWinsize(ws *Winsize) error {
	return ErrUnsupported
}

func newPty() (*Pty, error) {
	return nil, ErrUnsupported
}

func (p *Pty) startCommand(cmd *exec.Cmd) error {
	return ErrUnsupported
}

// getTermios on unsupported platforms always fails with ErrUnsupported.
func getTermios(*os.File) (termiosState, error) {
	return termiosState{}, ErrUnsupported
}

// setTermios on unsupported platforms always fails with ErrUnsupported.
func setTermios(*os.File, termiosState) error {
	return ErrUnsupported
}

// echoTermiosBits on unsupported platforms has no flags to report; SetEcho
// never reaches it because getTermios fails first.
func echoTermiosBits() uint64 {
	return 0
}
