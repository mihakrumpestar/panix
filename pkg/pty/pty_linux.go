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
