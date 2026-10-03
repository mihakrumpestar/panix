// Package pty provides a pseudo-terminal implementation for Unix systems.
//
// This package is derived from work by:
//   - Ayman Bagabas, https://github.com/aymanbagabas/go-pty (MIT License)
//   - Andrew Dunham, https://github.com/creack/pty (MIT License)
package pty

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

// ErrUnsupported is returned on platforms without PTY support.
var ErrUnsupported = errors.New("pty: unsupported platform")

// Pty represents a Unix pseudo-terminal.
// The master side is used for I/O with the child process.
type Pty struct {
	master *os.File
	slave  *os.File
	name   string
	closed bool
}

// New creates a new pseudo-terminal without starting a process.
// Call Start on the returned Pty to attach a command, or use
// Start directly for the common case.
func New() (*Pty, error) {
	return newPty()
}

// Start creates a new pseudo-terminal, attaches the command to it, and starts the process.
// The command's Stdin, Stdout, and Stderr are set to the PTY slave.
// After the process starts, the slave is closed in the parent so that
// Read returns EOF when the child exits.
// Returns the Pty (master side) for I/O with the child process.
func Start(cmd *exec.Cmd) (*Pty, error) {
	ptyInst, err := New()
	if err != nil {
		return nil, err
	}

	err = ptyInst.startCommand(cmd)
	if err != nil {
		_ = ptyInst.Close()

		return nil, err
	}

	return ptyInst, nil
}

// Write writes data to the PTY master, which appears as input on the child's terminal.
func (p *Pty) Write(b []byte) (int, error) {
	n, err := p.master.Write(b)
	if err != nil {
		return n, errors.Wrap(err, "pty: write")
	}

	return n, nil
}

// SetEcho toggles the terminal echo of the PTY line discipline.
//
// The master and the slave of a PTY pair share one termios state, so the
// ioctl is applied on the master fd: startCommand closes the parent's slave
// fd, which leaves the master as the only handle available after Start.
//
// SetEcho(false) clears the echo flags (ECHO and ECHONL) only. The line
// discipline stays in canonical mode with every other flag untouched, which
// is deliberately not a full raw-mode switch: canonical mode is fine for
// newline-delimited frames, while ECHONL would still echo line feeds even
// with ECHO off, so both flags must go.
//
// Callers that write command frames to the PTY master (a control channel in
// front of ssh) call SetEcho(false) after Start so the frames are not echoed
// back into the outbound stream. When the method is never called, the PTY
// keeps its default line discipline unchanged.
//
// On platforms without PTY support, and on a PTY without a master fd,
// SetEcho returns ErrUnsupported.
func (p *Pty) SetEcho(enabled bool) error {
	if p.master == nil {
		return ErrUnsupported
	}

	termios, err := getTermios(p.master)
	if err != nil {
		return errors.Wrap(err, "pty: get termios")
	}

	echoBits := echoTermiosBits()

	if enabled {
		termios.Lflag |= echoBits
	} else {
		termios.Lflag &^= echoBits
	}

	err = setTermios(p.master, termios)
	if err != nil {
		return errors.Wrap(err, "pty: set termios")
	}

	return nil
}

// Close closes the PTY master and slave file descriptors.
// It is safe to call Close multiple times.
func (p *Pty) Close() error {
	if p.closed {
		return nil
	}

	p.closed = true

	var closeErr error

	err := p.master.Close()
	if err != nil {
		closeErr = errors.Wrap(err, "pty: close master")
	}

	if p.slave != nil {
		err = p.slave.Close()
		if err != nil {
			if closeErr != nil {
				closeErr = errors.Wrapf(closeErr, "pty: close slave: %v", err)
			} else {
				closeErr = errors.Wrap(err, "pty: close slave")
			}
		}
	}

	return closeErr
}

// Fd returns the file descriptor of the PTY master.
func (p *Pty) Fd() uintptr {
	return p.master.Fd()
}

// Name returns the device name of the PTY slave (e.g., "/dev/pts/0").
func (p *Pty) Name() string {
	return p.name
}
