package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrSlotLocked reports that another live transaction holds the slot log lock
// (spec 4.1 step 3): the lock is held through an inherited open file description and
// releases only when every reference closes.
var ErrSlotLocked = errors.New("slot log is locked by a live transaction")

type slotLock struct{ file *os.File }

// Close releases this reference to the lock. The lock survives as long as any inherited
// reference (guardian, viewer) is still open (spec 4.1 step 3).
func (l *slotLock) Close() error { return l.file.Close() }

// ensureSlotDir creates the slot directory (0700) and verifies it is owned by the
// executing identity (spec 10.1, 12).
func ensureSlotDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: %w", errSlotOwner, err)
	}

	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%w: %w", errSlotOwner, err)
	}

	if us, ok := st.Sys().(*syscall.Stat_t); ok {
		// os.Getuid returns -1 when the identity is unavailable; treat that
		// as unverifiable rather than comparing against a wrapped -1.
		if uid := os.Getuid(); uid >= 0 && us.Uid != uint32(uid) { //nolint:gosec // the uid >= 0 guard rules out the negative wrap
			return fmt.Errorf("%w: %s is owned by uid %d, expected %d", errSlotOwner, dir, us.Uid, uid)
		}
	}

	return nil
}

// openLockedLog opens the slot log and takes the transaction lock (flock, nonblocking).
// ErrSlotLocked means a live transaction (viewer or guardian) already holds it.
func openLockedLog(dir string) (*slotLock, error) {
	f, err := os.OpenFile(slotLogPath(dir), os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // slot paths are the caller's own choice (spec 10.1); the child name is fixed
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errLogUnwritable, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()

		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrSlotLocked
		}

		return nil, fmt.Errorf("flock slot log: %w", err)
	}

	return &slotLock{file: f}, nil
}

// inheritedLockFile returns the transaction lock fd inherited through the exec chain
// (PANIX_GUARD_LOCK_FD), or nil when this process is not the detached guardian. The fd
// is marked close-on-exec so it does not leak into the guardian's own children. The
// caller must never LOCK_UN this fd: an explicit unlock in a forked child destroys the
// parent's lock (verified V4).
func inheritedLockFile() *os.File {
	raw := os.Getenv(lockFDEnv)
	if raw == "" {
		return nil
	}

	fd, err := strconv.Atoi(raw)
	if err != nil || fd < 3 {
		return nil
	}
	// Close-on-exec keeps the lock fd from leaking into the guardian's own children.
	// On linux CloseOnExec returns nothing; darwin returns an error we deliberately
	// ignore (a failed mark only leaks the fd into a short-lived child).
	syscall.CloseOnExec(fd)

	return os.NewFile(uintptr(fd), "panix-guard-slot-log-lock")
}

// guardianFDs carries the descriptors handed to the detached guardian through
// ExtraFiles (spec 6.1): the inherited transaction lock plus the two os.Pipe
// ends of the duplex wire (command read end, event write end). Nil entries are
// skipped and the env vars carry the resulting fd numbers, which start at 3
// and grow with the non-nil files in lock, cmd, evt order.
type guardianFDs struct {
	lock *os.File
	cmd  *os.File // command pipe read end: the guardian reads frames here
	evt  *os.File // event pipe write end: the guardian writes records here
}

// spawnDetached starts selfExe with args in a new session (Setsid, spec 6.1):
// stdio goes to logFile so pre-logging panics land in the transcript, and the
// extra descriptors are inherited as fds 3,4,5 (PANIX_GUARD_LOCK_FD and
// friends name them in the env).
//
// PINNED INVARIANT: Stdin/Stdout/Stderr are explicit, never nil. Go maps a
// nil stdio to /dev/null, which is silent wire death at birth (the relay
// would read an instant event-pipe EOF and misread it as guardian death).
func spawnDetached(selfExe string, args []string, env []string, logFile *os.File, fds guardianFDs) (*exec.Cmd, error) {
	cmd := exec.Command(selfExe, args...) //nolint:gosec // self re-exec with a caller-built argv is the detach mechanism (spec 6.1)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = logFile
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	extra := make([]*os.File, 0, 3)
	if fds.lock != nil {
		extra = append(extra, fds.lock)
	}

	if fds.cmd != nil {
		extra = append(extra, fds.cmd)
	}

	if fds.evt != nil {
		extra = append(extra, fds.evt)
	}

	cmd.ExtraFiles = extra

	env = append([]string{}, env...)

	for i, f := range extra {
		fd := 3 + i

		switch f {
		case fds.lock:
			env = append(env, lockFDEnv+"="+strconv.Itoa(fd))
		case fds.cmd:
			env = append(env, cmdFDEnv+"="+strconv.Itoa(fd))
		case fds.evt:
			env = append(env, evtFDEnv+"="+strconv.Itoa(fd))
		}
	}

	cmd.Env = env

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("spawn detached process: %w", err)
	}

	return cmd, nil
}

// inheritedWirePipes opens the command and event pipe descriptors inherited
// through ExtraFiles and marks both close-on-exec (spec 6.1 CLOEXEC
// discipline: the activation child must inherit none of the guardian's
// channels, or EOF-based death detection breaks and the lock outlives the
// guardian). Missing descriptors mean no wire; the machine then runs on
// signals alone.
func inheritedWirePipes() (cmd io.ReadCloser, evt io.WriteCloser) {
	if raw := os.Getenv(cmdFDEnv); raw != "" {
		if fd, err := strconv.Atoi(raw); err == nil && fd >= 3 {
			syscall.CloseOnExec(fd)
			cmd = os.NewFile(uintptr(fd), "panix-guard-command-pipe")
		}
	}

	if raw := os.Getenv(evtFDEnv); raw != "" {
		if fd, err := strconv.Atoi(raw); err == nil && fd >= 3 {
			syscall.CloseOnExec(fd)
			evt = os.NewFile(uintptr(fd), "panix-guard-event-pipe")
		}
	}

	return cmd, evt
}

// processCmdline returns the command line for pid: /proc when available (linux), ps
// otherwise (darwin). Runtime detection, no build tags (spec 10.3).
func processCmdline(pid int) (string, bool) {
	if _, err := os.Stat("/proc"); err == nil {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			return "", false
		}

		return strings.ReplaceAll(string(b), "\x00", " "), true
	}

	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output() //nolint:gosec // the pid is an integer, the args are fixed (spec 8 forensics)
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(string(out)), true
}

// guardianAlive reports whether pid is a live panix-guard process for this deploy key
// (spec 8). The pid alone is never trusted: the command line must match the binary name
// and the key, which defeats pid reuse.
func guardianAlive(pid int, key string) bool {
	line, ok := processCmdline(pid)
	if !ok {
		return false
	}

	return strings.Contains(line, "panix-guard") && key != "" && strings.Contains(line, key)
}

// removeGCRoot removes the slot's GC-root symlink; a missing symlink is not an error
// (terminal states clear the slot, spec 9.3).
func removeGCRoot(dir string) error {
	err := os.Remove(filepath.Join(dir, gcRootName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove gc-root: %w", err)
	}

	return nil
}

// rebootCandidates are the reboot commands tried in order (spec 6.3): the linux reboot
// paths first, then darwin's shutdown.
func rebootCandidates() [][]string {
	return [][]string{
		{"/run/current-system/sw/bin/reboot"},
		{"/usr/sbin/reboot"},
		{"/sbin/reboot"},
		{"/usr/bin/shutdown", "-r", "now"},
		{"/sbin/shutdown", "-r", "now"},
	}
}

// reboot requests an immediate machine reboot (gated, spec 6.3). Returns true when a
// candidate started; the caller treats that as the process ending. Returns false when
// every candidate failed and the state stays revert_failed.
func reboot(logw *LogWriter) bool {
	for _, cand := range rebootCandidates() {
		if _, err := exec.LookPath(cand[0]); err != nil {
			continue
		}

		cmd := exec.Command(cand[0], cand[1:]...) //nolint:gosec // the reboot candidates are a fixed table (spec 6.3)

		err := cmd.Start()
		if err != nil {
			continue
		}

		_ = cmd.Process.Release()

		logw.WriteLine("guard: reboot requested via " + cand[0])

		return true
	}

	return false
}
