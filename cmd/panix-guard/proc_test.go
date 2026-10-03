package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Helper-process harness: spawnDetached re-execs this test binary with
// PANIX_GUARD_TEST_HELPER set; TestMain routes to the helper instead of the test
// runner. All helper output lands in the slot log (spawnDetached redirects stdout),
// which the parent then asserts on.
func TestMain(m *testing.M) {
	switch os.Getenv("PANIX_GUARD_TEST_HELPER") {
	case "dump":
		runHelperDump()

		return
	case "trylock":
		runHelperTrylock()

		return
	case "close3":
		runHelperClose3()

		return
	case "unlock3":
		runHelperUnlock3()

		return
	case "cleexec":
		runHelperCleExec()

		return
	case "fddump":
		runHelperFdDump()

		return
	}

	os.Exit(m.Run())
}

// runHelperCleExec simulates the guardian startup's fd handling: it prints
// the close-on-exec state of the inherited descriptors BEFORE (the ExtraFiles
// dup clears it) and AFTER the production openers (inheritedLockFile +
// inheritedWirePipes must set FD_CLOEXEC on every one of them, spec 6.1),
// then execs the fddump helper for the end-to-end leak check.
func runHelperCleExec() {
	printCloexecState("pre")

	_ = inheritedLockFile()

	cmdR, evtW := inheritedWirePipes()

	// The state must be printed before the wrapped handles close: the
	// CLOEXEC mark is what matters, not the handle's lifetime.
	printCloexecState("post")

	if cmdR != nil {
		_ = cmdR.Close()
	}

	if evtW != nil {
		_ = evtW.Close()
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Println("exec=self-error")
		os.Exit(0)
	}

	// Override (not append) the helper mode: an inherited cleexec value must
	// not win over the fddump hand-off or the helper would re-exec forever.
	os.Setenv("PANIX_GUARD_TEST_HELPER", "fddump")

	if err := syscall.Exec(self, []string{self}, os.Environ()); err != nil { //nolint:gosec // the exec is the CLOEXEC probe itself
		fmt.Printf("exec=%v\n", err)
		os.Exit(0)
	}
}

// printCloexecState reports the FD_CLOEXEC flag of the inherited descriptor
// numbers (env-declared; the ExtraFiles positions).
func printCloexecState(phase string) {
	for _, raw := range []string{os.Getenv(lockFDEnv), os.Getenv(cmdFDEnv), os.Getenv(evtFDEnv)} {
		fd, err := strconv.Atoi(raw)
		if err != nil || fd < 3 {
			continue
		}

		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_GETFD), 0)
		if errno != 0 {
			fmt.Printf("%s:fd%d=missing\n", phase, fd)

			continue
		}

		if flags&syscall.FD_CLOEXEC != 0 {
			fmt.Printf("%s:fd%d=cloexec\n", phase, fd)
		} else {
			fmt.Printf("%s:fd%d=noCloexec\n", phase, fd)
		}
	}
}

// runHelperFdDump reports whether fds 3..7 survived the exec: a descriptor
// present here after exec means a CLOEXEC leak (spec 6.1). On linux the
// readlink target names what leaked (the Go runtime itself opens a cgroup
// probe fd at startup, so the assertion is on TARGETS, not fd numbers).
func runHelperFdDump() {
	for fd := 3; fd <= 7; fd++ {
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_GETFD), 0)
		if errno != 0 {
			fmt.Printf("fd%d=closed\n", fd)

			continue
		}

		target, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
		if err != nil {
			target = "?"
		}

		fmt.Printf("fd%d=open target=%s\n", fd, target)
	}

	os.Exit(0)
}

func runHelperDump() {
	fmt.Printf("pid=%d\n", syscall.Getpid())
	fmt.Printf("pgid=%d\n", syscall.Getpgrp())

	if f := inheritedLockFile(); f != nil {
		if _, err := f.Stat(); err == nil {
			fmt.Println("fd3=open")
		} else {
			fmt.Println("fd3=broken")
		}
	} else {
		fmt.Println("fd3=closed")
	}

	if v := os.Getenv("PANIX_GUARD_TEST_MARK"); v != "" {
		fmt.Printf("env=%s\n", v)
	}

	os.Exit(0)
}

// runHelperTrylock opens a FRESH fd on the slot log and tries to flock it: this is
// exactly how an outsider tests whether a live transaction holds the lock.
func runHelperTrylock() {
	dir := os.Getenv("PANIX_GUARD_TEST_SLOT")

	f, err := os.OpenFile(filepath.Join(dir, logName), os.O_RDWR, 0o600) //nolint:gosec // test fixture paths
	if err != nil {
		fmt.Println("trylock=open-error")
		os.Exit(0)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Println("trylock=blocked")
	} else {
		fmt.Println("trylock=acquired")

		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}

	f.Close()
	os.Exit(0)
}

func runHelperClose3() {
	f := inheritedLockFile()
	if f == nil {
		fmt.Println("close3=no-fd")
		os.Exit(0)
	}

	f.Close()
	fmt.Println("close3=done")
	os.Exit(0)
}

// runHelperUnlock3 demonstrates the verified V4 hazard: an explicit LOCK_UN in a child
// holding the inherited open file description destroys the parent's lock. Production
// code must never do this.
func runHelperUnlock3() {
	f := inheritedLockFile()
	if f == nil {
		fmt.Println("unlock3=no-fd")
		os.Exit(0)
	}

	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	fmt.Println("unlock3=done")
	os.Exit(0)
}

func spawnHelper(t *testing.T, mode, slotDir, envMark string, lockFD, logFile *os.File) {
	t.Helper()

	spawnHelperWire(t, mode, slotDir, envMark, logFile, guardianFDs{lock: lockFD})
}

// spawnHelperWire is spawnHelper with the full guardian fd set (spec 6.1:
// lock fd plus the two duplex pipe ends).
func spawnHelperWire(t *testing.T, mode, slotDir, envMark string, logFile *os.File, fds guardianFDs) {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}

	env := append(os.Environ(), "PANIX_GUARD_TEST_HELPER="+mode)
	if slotDir != "" {
		env = append(env, "PANIX_GUARD_TEST_SLOT="+slotDir)
	}

	if envMark != "" {
		env = append(env, "PANIX_GUARD_TEST_MARK="+envMark)
	}

	cmd, err := spawnDetached(self, nil, env, logFile, fds)
	if err != nil {
		t.Fatalf("spawn helper %s: %v", mode, err)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper %s exited with error: %v", mode, err)
	}
}

func readLog(t *testing.T, dir string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	return string(b)
}

func TestOpenLockedLogExclusive(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	_, err = openLockedLog(dir)
	if !errors.Is(err, ErrSlotLocked) {
		t.Fatalf("want ErrSlotLocked, got %v", err)
	}

	l1.Close()

	l2, err := openLockedLog(dir)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}

	l2.Close()
}

func TestLockSurvivesChildClose(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The child closes its inherited reference: the parent's reference keeps the lock.
	spawnHelper(t, "close3", dir, "", l1.file, l1.file)
	spawnHelper(t, "trylock", dir, "", nil, l1.file)

	log := readLog(t, dir)
	if !strings.Contains(log, "close3=done") {
		t.Fatalf("missing close3 marker in log: %q", log)
	}

	if !strings.Contains(log, "trylock=blocked") {
		t.Fatalf("lock must survive child close; log: %q", log)
	}

	l1.Close()
	// With every reference closed the lock is gone: a fresh open+flock succeeds.
	lf, err := os.OpenFile(filepath.Join(dir, logName), os.O_RDWR|os.O_APPEND, 0o600) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	spawnHelper(t, "trylock", dir, "", nil, lf)
	lf.Close()

	if !strings.Contains(readLog(t, dir), "trylock=acquired") {
		t.Fatalf("lock must be released once all references close")
	}
}

// TestUnlockInChildDestroysParentLock documents the verified V4 hazard: LOCK_UN on the
// inherited open file description destroys the parent's lock. This is why the guardian
// must never unlock the inherited fd.
func TestUnlockInChildDestroysParentLock(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatal(err)
	}

	spawnHelper(t, "unlock3", dir, "", l1.file, l1.file)
	spawnHelper(t, "trylock", dir, "", nil, l1.file)

	log := readLog(t, dir)
	if !strings.Contains(log, "unlock3=done") {
		t.Fatalf("missing unlock3 marker in log: %q", log)
	}

	if !strings.Contains(log, "trylock=acquired") {
		t.Fatalf("expected the child unlock to destroy the parent lock (V4 hazard)")
	}

	l1.Close()
}

func TestDetachSetsidAndInheritance(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatal(err)
	}

	spawnHelper(t, "dump", dir, "marker-value", l1.file, l1.file)

	log := readLog(t, dir)
	if !strings.Contains(log, "fd3=open") {
		t.Fatalf("lock fd must be inherited as fd 3; log: %q", log)
	}

	if !strings.Contains(log, "env=marker-value") {
		t.Fatalf("env must pass through the detach exec; log: %q", log)
	}

	var pid, pgid = -1, -2

	for line := range strings.SplitSeq(log, "\n") {
		if v, ok := strings.CutPrefix(line, "pid="); ok {
			pid, _ = strconv.Atoi(v)
		}

		if v, ok := strings.CutPrefix(line, "pgid="); ok {
			pgid, _ = strconv.Atoi(v)
		}
	}

	if pid < 1 || pgid < 1 {
		t.Fatalf("missing pid/pgid dump: %q", log)
	}

	if pid != pgid {
		t.Fatalf("detached child must be its own session leader (Setsid): pid=%d pgid=%d", pid, pgid)
	}

	l1.Close()
}

// TestCLOEXECLeakRegression pins the spec 6.1 CLOEXEC discipline: the
// guardian's inherited descriptors (lock, command pipe, event pipe) carry
// FD_CLOEXEC after the production openers, so the activation child inherits
// none of them, and the end-to-end exec shows the slot log did not leak into
// a grandchild.
func TestCLOEXECLeakRegression(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer l1.Close()

	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	defer cmdW.Close()

	evtR, evtW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	defer evtR.Close()

	spawnHelperWire(t, "cleexec", dir, "", l1.file, guardianFDs{lock: l1.file, cmd: cmdR, evt: evtW})

	log := readLog(t, dir)

	// Before the production openers the ExtraFiles dups carry no CLOEXEC:
	// this is exactly the state a regression would reintroduce.
	for fd := 3; fd <= 5; fd++ {
		if !strings.Contains(log, fmt.Sprintf("pre:fd%d=noCloexec", fd)) {
			t.Fatalf("fd %d must start without CLOEXEC (the leak state this test detects); log:\n%s", fd, log)
		}

		if !strings.Contains(log, fmt.Sprintf("post:fd%d=cloexec", fd)) {
			t.Fatalf("fd %d must carry FD_CLOEXEC after the guardian openers; log:\n%s", fd, log)
		}
	}

	// End to end: after the exec chain, no descriptor in the grandchild may
	// point at the slot log (linux names the targets; elsewhere the flag
	// assertions above carry the contract).
	if _, err := os.Stat("/proc/self/fd"); err == nil {
		for line := range strings.SplitSeq(log, "\n") {
			target, ok := strings.CutPrefix(line, "fd")
			if !ok {
				continue
			}

			_, rest, found := strings.Cut(target, "=open target=")
			if found && strings.HasSuffix(rest, "/"+logName) {
				t.Fatalf("the slot log leaked into the grandchild: %q; log:\n%s", rest, log)
			}
		}
	}
}

func TestGuardianAlive(t *testing.T) {
	script := filepath.Join(t.TempDir(), "panix-guard-fake")

	err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 3\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(script, 0o755); err != nil { //nolint:gosec // the fake guardian must be executable; the quiet write satisfied G306
		t.Fatal(err)
	}

	cmd := exec.Command("sh", script, "key123") //nolint:gosec // the test's own stub script

	err = cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	// Start returns at the execve pipe close, before the kernel has published the new
	// argv in /proc: poll instead of racing (production ctl calls run long after spawn).
	deadline := time.Now().Add(2 * time.Second)

	for {
		line, ok := processCmdline(pid)
		if ok && strings.Contains(line, "panix-guard") && strings.Contains(line, "key123") {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("cmdline never matched: ok=%v line=%q", ok, line)
		}

		time.Sleep(50 * time.Millisecond)
	}

	if !guardianAlive(pid, "key123") {
		t.Fatal("guardianAlive false despite a matching cmdline")
	}

	if guardianAlive(pid, "other-key") {
		t.Fatal("a different deploy key must not match")
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if guardianAlive(pid, "key123") {
		t.Fatal("a dead pid must not match")
	}
}
