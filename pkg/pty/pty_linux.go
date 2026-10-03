//go:build linux

// Derived from github.com/creack/pty (MIT License, Copyright Andrew Dunham)

package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

func open() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "pty: open /dev/ptmx")
	}

	// In case of error after this point, close the master fd.
	cleanup := true

	defer func() {
		if cleanup {
			_ = master.Close()
		}
	}()

	sname, err := ptsname(master)
	if err != nil {
		return nil, nil, err
	}

	err = unlockpt(master)
	if err != nil {
		return nil, nil, err
	}

	slave, err := os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0) //nolint:gosec // G304: PTY slave path from kernel ioctl is trusted
	if err != nil {
		return nil, nil, errors.Wrapf(err, "pty: open slave %s", sname)
	}

	cleanup = false

	return master, slave, nil
}

// ptsname returns the name of the slave pseudoterminal.
// Uses TIOCGPTN ioctl to get the PTY number and constructs the path.
func ptsname(f *os.File) (string, error) {
	var ptyNum uint32

	err := ioctl(f, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&ptyNum))) //nolint:gosec // G103: unsafe.Pointer required for ioctl syscall
	if err != nil {
		return "", errors.Wrap(err, "pty: get ptsname")
	}

	return "/dev/pts/" + strconv.Itoa(int(ptyNum)), nil
}

// unlockpt unlocks the slave pseudoterminal.
// Uses TIOCSPTLCK with a zero value to clear the lock.
func unlockpt(f *os.File) error {
	var u int32

	return ioctl(f, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&u))) //nolint:gosec // G103: unsafe.Pointer required for ioctl syscall
}

// getTermios reads the termios state of the PTY through the master fd. The
// master and the slave share one termios state, so this covers the slave's
// line discipline even though the parent's slave fd is closed after Start.
func getTermios(f *os.File) (syscall.Termios, error) {
	var termios syscall.Termios

	err := ioctl(f, syscall.TCGETS, uintptr(unsafe.Pointer(&termios))) //nolint:gosec // G103: unsafe.Pointer required for ioctl syscall
	if err != nil {
		return syscall.Termios{}, errors.Wrap(err, "pty: tcgets")
	}

	return termios, nil
}

// setTermios writes the termios state of the PTY through the master fd.
func setTermios(f *os.File, termios syscall.Termios) error {
	err := ioctl(f, syscall.TCSETS, uintptr(unsafe.Pointer(&termios))) //nolint:gosec // G103: unsafe.Pointer required for ioctl syscall
	if err != nil {
		return errors.Wrap(err, "pty: tcsets")
	}

	return nil
}

// echoTermiosBits returns the local flags that make the line discipline echo
// input back to the master. ECHONL echoes line feeds even when ECHO is off,
// so both must be cleared to silence a newline-delimited control channel.
func echoTermiosBits() uint32 {
	return syscall.ECHO | syscall.ECHONL
}
