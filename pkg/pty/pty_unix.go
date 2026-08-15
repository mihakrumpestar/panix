//go:build linux || darwin || freebsd

// Derived from github.com/aymanbagabas/go-pty (MIT License, Copyright 2023 Ayman Bagabas)
// and github.com/creack/pty (MIT License, Copyright Andrew Dunham)

package pty

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Winsize represents the terminal window size, aliased from unix.Winsize.
type Winsize = unix.Winsize

// Read reads output from the PTY master (what the child process writes to its terminal).
// On Linux, reading from a master PTY whose slave has been closed returns EIO;
// this is translated to io.EOF for correct Go semantics.
// See: https://github.com/creack/pty/issues/21
// Read reads output from the PTY master (what the child process writes to its terminal).
// When the child exits and the slave PTY closes, the Linux kernel returns EIO from the
// master read. This is a normal PTY lifecycle event, not an error. Read translates it
// to a zero-byte read with nil error so the caller can treat 0 bytes as end-of-stream.
// See: https://github.com/creack/pty/issues/21
func (p *Pty) Read(b []byte) (int, error) {
	bytesRead, err := p.master.Read(b)
	if err != nil {
		if isEIOError(err) {
			return 0, nil
		}

		return bytesRead, errors.Wrap(err, "pty: read")
	}

	return bytesRead, nil
}

// Resize resizes the PTY terminal window to the given width and height in characters.
func (p *Pty) Resize(w, h int) error {
	return p.SetWinsize(&Winsize{
		Row: uint16(h), //nolint:gosec // G115: safe conversion, terminal dimensions always fit in uint16
		Col: uint16(w), //nolint:gosec // G115: safe conversion, terminal dimensions always fit in uint16
	})
}

// SetWinsize sets the PTY window size using the raw Winsize struct.
func (p *Pty) SetWinsize(winsize *Winsize) error {
	conn, err := p.master.SyscallConn()
	if err != nil {
		return errors.Wrap(err, "pty: set winsize")
	}

	var ioctlErr error

	err = conn.Control(func(fd uintptr) {
		ioctlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, winsize)
	})
	if err != nil {
		return errors.Wrap(err, "pty: control")
	}

	if ioctlErr != nil {
		return errors.Wrap(ioctlErr, "pty: ioctl set winsize")
	}

	return nil
}

func newPty() (*Pty, error) {
	master, slave, err := open()
	if err != nil {
		return nil, err
	}

	return &Pty{
		master: master,
		slave:  slave,
		name:   slave.Name(),
	}, nil
}

// startCommand attaches the command to the PTY slave and starts it.
// After starting, the slave is closed in the parent process so that
// Read returns EOF when the child exits.
func (p *Pty) startCommand(cmd *exec.Cmd) error {
	cmd.Stdin = p.slave
	cmd.Stdout = p.slave
	cmd.Stderr = p.slave

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	err := cmd.Start()
	if err != nil {
		return errors.Wrap(err, "pty: start command")
	}

	// Close the slave in the parent process. The child inherited the fd via exec.
	// Without this close, Read on the master would block forever after the child exits
	// because the kernel sees an open slave reference in the parent.
	err = p.slave.Close()
	if err != nil {
		return errors.Wrap(err, "pty: close slave")
	}

	p.slave = nil

	return nil
}

// ioctl performs an ioctl call on the given file using SyscallConn.Control
// for safe interaction with the Go runtime poller.
func ioctl(f *os.File, cmd, ptr uintptr) error {
	conn, err := f.SyscallConn()
	if err != nil {
		// Fall back to blocking ioctl if SyscallConn is unavailable.
		return ioctlRaw(f.Fd(), cmd, ptr)
	}

	var ioctlErr error

	err = conn.Control(func(fd uintptr) {
		ioctlErr = ioctlRaw(fd, cmd, ptr)
	})
	if err != nil {
		return errors.Wrap(err, "pty: control")
	}

	if ioctlErr != nil {
		return errors.Wrap(ioctlErr, "pty: ioctl")
	}

	return nil
}

// ioctlRaw performs a raw ioctl syscall. The ptr argument must be a uintptr
// representation of the data pointer required by the specific ioctl command.
// Callers must ensure the pointer remains valid for the duration of the call.
func ioctlRaw(fd, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if e != 0 {
		return e
	}

	return nil
}

// isEIOError checks whether the error is an EIO from reading a master PTY
// whose slave has been closed. This is a Linux-specific kernel behavior.
func isEIOError(err error) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr == nil {
		return false
	}

	return errors.Is(pathErr.Err, syscall.EIO)
}
