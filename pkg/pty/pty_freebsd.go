//go:build freebsd

// Derived from github.com/creack/pty (MIT License, Copyright Andrew Dunham)

package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func open() (master, slave *os.File, err error) {
	// posix_openpt returns an already-unlocked PTY master.
	// No separate grantpt/unlockpt calls needed on FreeBSD.
	fd, _, e1 := syscall.Syscall(syscall.SYS_POSIX_OPENPT, uintptr(syscall.O_RDWR|syscall.O_CLOEXEC), 0, 0)
	if e1 != 0 {
		return nil, nil, errors.Wrap(e1, "pty: posix_openpt")
	}
	master = os.NewFile(uintptr(fd), "/dev/pts")
	defer func() {
		if err != nil {
			_ = master.Close()
		}
	}()

	sname, err := ptsname(master)
	if err != nil {
		return nil, nil, err
	}

	slave, err = os.OpenFile("/dev/"+sname, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "pty: open slave /dev/%s", sname)
	}

	return master, slave, nil
}

// ptsname returns the name of the slave pseudoterminal.
// Uses TIOCGPTN ioctl (same as Linux) to get the PTY number.
func ptsname(f *os.File) (string, error) {
	var n uint32
	if err := ioctl(f, unix.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil { //nolint:gosec // Expected unsafe pointer for ioctl syscall.
		return "", errors.Wrap(err, "pty: get ptsname")
	}
	return "/dev/pts/" + strconv.Itoa(int(n)), nil
}

// getTermios reads the termios state of the PTY through the master fd. The
// master and the slave share one termios state, so this covers the slave's
// line discipline even though the parent's slave fd is closed after Start.
func getTermios(f *os.File) (syscall.Termios, error) {
	var termios syscall.Termios

	err := ioctl(f, syscall.TIOCGETA, uintptr(unsafe.Pointer(&termios))) //nolint:gosec // Expected unsafe pointer for ioctl syscall.
	if err != nil {
		return syscall.Termios{}, errors.Wrap(err, "pty: tiocgeta")
	}

	return termios, nil
}

// setTermios writes the termios state of the PTY through the master fd.
func setTermios(f *os.File, termios syscall.Termios) error {
	err := ioctl(f, syscall.TIOCSETA, uintptr(unsafe.Pointer(&termios))) //nolint:gosec // Expected unsafe pointer for ioctl syscall.
	if err != nil {
		return errors.Wrap(err, "pty: tiocseta")
	}

	return nil
}

// echoTermiosBits returns the local flags that make the line discipline echo
// input back to the master. ECHONL echoes line feeds even when ECHO is off,
// so both must be cleared to silence a newline-delimited control channel.
func echoTermiosBits() uint32 {
	return syscall.ECHO | syscall.ECHONL
}
