//go:build darwin

// Derived from github.com/creack/pty (MIT License, Copyright Andrew Dunham)

package pty

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

func open() (master, slave *os.File, err error) {
	// Use syscall.Open to set O_CLOEXEC atomically, avoiding a race
	// between open and fork/exec in the parent.
	pFD, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "pty: open /dev/ptmx")
	}
	master = os.NewFile(uintptr(pFD), "/dev/ptmx")
	// In case of error after this point, close the master fd.
	defer func() {
		if err != nil {
			_ = master.Close()
		}
	}()

	sname, err := ptsname(master)
	if err != nil {
		return nil, nil, err
	}

	if err := grantpt(master); err != nil {
		return nil, nil, err
	}

	if err := unlockpt(master); err != nil {
		return nil, nil, err
	}

	slave, err = os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "pty: open slave %s", sname)
	}

	return master, slave, nil
}

// ptsname returns the name of the slave pseudoterminal.
// Uses TIOCPTYGNAME ioctl to retrieve the path from the kernel.
func ptsname(f *os.File) (string, error) {
	n := make([]byte, _iocParmLen(syscall.TIOCPTYGNAME))
	if err := ioctl(f, syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&n[0]))); err != nil { //nolint:gosec // Expected unsafe pointer for ioctl syscall.
		return "", errors.Wrap(err, "pty: get ptsname")
	}

	for i, c := range n {
		if c == 0 {
			return string(n[:i]), nil
		}
	}
	return "", errors.New("pty: TIOCPTYGNAME string not NUL-terminated")
}

// grantpt grants access to the slave pseudoterminal.
// Uses TIOCPTYGRANT ioctl.
func grantpt(f *os.File) error {
	return ioctl(f, syscall.TIOCPTYGRANT, 0)
}

// unlockpt unlocks the slave pseudoterminal.
// Uses TIOCPTYUNLK ioctl.
func unlockpt(f *os.File) error {
	return ioctl(f, syscall.TIOCPTYUNLK, 0)
}

// _iocParmLen extracts the parameter length from a BSD ioctl command value.
// Used to size the buffer for TIOCPTYGNAME.
func _iocParmLen(ioctl uintptr) uintptr {
	return (ioctl >> 16) & ((1 << 13) - 1)
}
