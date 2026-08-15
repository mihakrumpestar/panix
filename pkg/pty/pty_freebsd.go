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
