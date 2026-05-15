//go:build !linux && !darwin && !freebsd

package pty

import (
	"io"
	"os/exec"
)

// Winsize represents the terminal window size on unsupported platforms.
type Winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
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
