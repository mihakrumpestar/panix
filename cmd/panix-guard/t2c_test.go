package main

import (
	"github.com/mihakrumpestar/panix/internal/guard"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeLog builds a slot log from raw record strings for viewer and decide tests.
func writeLog(t *testing.T, dir string, lines ...string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(dir, logName), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// rec renders one fixture record line with the envelope filled in.
func rec(key string, seq uint64, ev string, r guard.Record) string {
	r.K, r.W, r.Seq, r.TS, r.Ev = key, guard.WriterGuardian, seq, 100, ev

	return guard.FormatRecord(r)
}

func followStdout(t *testing.T, dir, key string, offset int64) (int, string) {
	t.Helper()

	old := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = w
	code := followLog(dir, key, offset)

	w.Close()

	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return code, string(out)
}

func withFastViewer(t *testing.T) {
	t.Helper()

	oldPoll, oldWait := viewerPoll, viewerHelloWait
	viewerPoll = 20 * time.Millisecond
	viewerHelloWait = 300 * time.Millisecond

	t.Cleanup(func() { viewerPoll, viewerHelloWait = oldPoll, oldWait })
}

func TestFollowCommitted(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	key := "testkey"
	writeLog(t, slot,
		rec(key, 1, guard.EventHello, guard.Record{PID: 1}),
		rec(key, 2, guard.EventState, guard.Record{St: string(stateActivating)}),
		rec(key, 3, guard.EventCommitStart, guard.Record{}),
		rec(key, 4, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
		rec(key, 5, guard.EventExit, guard.Record{St: string(stateCommitted)}),
	)

	if code := followLog(slot, key, 0); code != guard.ExitCommitted {
		t.Fatalf("follow exit code: %d", code)
	}
}

func TestFollowReverted(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	key := "testkey"
	writeLog(t, slot,
		rec(key, 1, guard.EventHello, guard.Record{PID: 1}),
		rec(key, 2, guard.EventRevertStart, guard.Record{Rs: "test"}),
		rec(key, 3, guard.EventReverted, guard.Record{St: string(stateReverted), RC: 0}),
		rec(key, 4, guard.EventExit, guard.Record{St: string(stateReverted)}),
	)

	if code := followLog(slot, key, 0); code != guard.ExitReverted {
		t.Fatalf("follow exit code: %d", code)
	}
}

func TestFollowStreamsOutput(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	key := "testkey"
	writeLog(t, slot,
		"activation output line",
		rec(key, 1, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
		rec(key, 2, guard.EventExit, guard.Record{St: string(stateCommitted)}),
	)

	code, out := followStdout(t, slot, key, 0)
	if code != guard.ExitCommitted {
		t.Fatalf("follow exit code: %d", code)
	}

	if !strings.Contains(out, "activation output line") {
		t.Fatalf("child output must stream: %q", out)
	}

	if !strings.Contains(out, "@PG2") {
		t.Fatalf("records must pass through for panix's stream reader: %q", out)
	}
}

func TestFollowRestartAfterTruncation(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	key := "testkey"
	writeLog(t, slot,
		rec(key, 1, guard.EventState, guard.Record{St: string(stateActivating)}),
		rec(key, 2, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
	)

	st, err := os.Stat(filepath.Join(slot, logName))
	if err != nil {
		t.Fatal(err)
	}
	// An offset beyond the file size (post front-truncation) must restart at 0 and
	// still converge on the terminal record (spec 9.2).
	if code := followLog(slot, key, st.Size()+1000); code != guard.ExitCommitted {
		t.Fatalf("follow exit code after truncation: %d", code)
	}
}

func TestFollowGuardianDeath(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	key := "testkey"
	// A fresh dead pid: spawn, kill, reap.
	cmd := exec.Command("sleep", "5")

	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	writeLog(t, slot,
		rec(key, 1, guard.EventHello, guard.Record{PID: pid}),
		rec(key, 2, guard.EventState, guard.Record{St: string(stateActivating), PID: pid}),
	)

	if code := followLog(slot, key, 0); code != guard.ExitCancelled {
		t.Fatalf("guardian death must yield cancelled, got %d", code)
	}
}

func TestFollowNoHello(t *testing.T) {
	withFastViewer(t)

	slot := t.TempDir()

	err := os.WriteFile(filepath.Join(slot, logName), nil, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()

	code := followLog(slot, "testkey", 0)
	if code != guard.ExitCancelled {
		t.Fatalf("no-hello must yield cancelled, got %d", code)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("hello grace not applied: %s", elapsed)
	}
}

// ctl tests run the transaction in a goroutine of the test process: the HELLO
// record carries the test process pid, and the ctl liveness check matches the
// test binary's
// own command line (which contains the binary base name used as the key).
func ctlTestKey() string { return filepath.Base(os.Args[0]) }

func startTx(t *testing.T, cfg *startConfig) (chan txState, string) {
	t.Helper()
	slot := t.TempDir()
	cfg.dir = slot
	// The ctl liveness check matches the guardian cmdline against the key: use the
	// test binary base name, which is always part of this process's command line.
	cfg.key = ctlTestKey()

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}

	logw, err := OpenLogWriter(slot)
	if err != nil {
		t.Fatalf("log writer: %v", err)
	}

	done := make(chan txState, 1)
	go func() { done <- newTransaction(cfg, logw, lock.file, nil, nil).run() }()

	return done, slot
}

func waitForStatus(t *testing.T, dir, key, status string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f, err := guard.ScanFile(filepath.Join(dir, logName), key); err == nil && f.Found && f.Status == status {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("status %s never appeared", status)
}

func TestCtlConfirmConsumed(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.commitArgv = [][]string{{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.invariantTarget = cfg.newClosure

	done, slotDir := startTx(t, cfg)
	waitForStatus(t, slotDir, cfg.key, string(stateActivated), 3*time.Second)

	if code := ctlSend(slotDir, cfg.key, "confirm", 2*time.Second); code != guard.CtlExitConsumed {
		t.Fatalf("confirm ack: %d", code)
	}

	if st := <-done; st != stateCommitted {
		t.Fatalf("state=%s", st)
	}
}

func TestCtlTimeoutAckTimedOut(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
sleep 30
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	done, slotDir := startTx(t, cfg)

	time.Sleep(100 * time.Millisecond) // let HELLO land
	// The auto tier never consumes a confirm: the request is delivered but no
	// ack arrives, which is the ack-timeout outcome (7, spec 9.5) rather than
	// refused; the request stays pending (it is discarded when the revert
	// wins).
	if code := ctlSend(slotDir, cfg.key, "confirm", 300*time.Millisecond); code != guard.CtlExitAckTimedOut {
		t.Fatalf("confirm ack timeout: %d", code)
	}

	err := syscall.Kill(os.Getpid(), syscall.SIGUSR2)
	if err != nil {
		t.Fatal(err)
	}

	if st := <-done; st != stateReverted {
		t.Fatalf("state=%s", st)
	}
}

func TestCtlRevertRequestConsumed(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
sleep 30
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	done, slotDir := startTx(t, cfg)

	time.Sleep(100 * time.Millisecond)

	if code := ctlSend(slotDir, cfg.key, "revert-request", 2*time.Second); code != guard.CtlExitConsumed {
		t.Fatalf("revert-request ack: %d", code)
	}

	if st := <-done; st != stateReverted {
		t.Fatalf("state=%s", st)
	}
}

func TestCtlDeadAndNoSlot(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.profilePath = ""
	// A dead guardian pid: spawn, kill, reap.
	cmd := exec.Command("sleep", "5")

	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	writeLog(t, slot,
		rec(ctlTestKey(), 1, guard.EventHello, guard.Record{PID: pid}),
	)

	if code := ctlSend(slot, ctlTestKey(), "confirm", 100*time.Millisecond); code != guard.CtlExitDead {
		t.Fatalf("dead guardian: %d", code)
	}

	empty := t.TempDir()
	if code := ctlSend(empty, ctlTestKey(), "confirm", 100*time.Millisecond); code != guard.CtlExitNoSlot {
		t.Fatalf("missing slot: %d", code)
	}

	_ = stubs
}

func TestConvergeActivating(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())

	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	err := os.WriteFile(filepath.Join(slot, gcRootName), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// A live orphan child holding the activation: decide must kill it.
	orphan := exec.Command("sleep", "30")

	orphan.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err = orphan.Start()
	if err != nil {
		t.Fatal(err)
	}

	orphanPID := orphan.Process.Pid
	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999}),
		rec(cfg.key, 2, guard.EventState, guard.Record{St: string(stateActivating), PID: 999, CPID: orphanPID}),
	)

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d", code)
	}

	log := readLog(t, slot)
	assertContains(t, log, guard.EventOrphanKilled, guard.EventRevertStart, guard.EventReverted)
	// The orphan must be gone: Wait returns promptly instead of hanging.
	done := make(chan error, 1)
	go func() { done <- orphan.Wait() }()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("orphan child was not killed")
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.oldClosure {
		t.Errorf("profile after decide: %q err=%v", got, err)
	}

	if _, err := os.Stat(filepath.Join(slot, gcRootName)); !os.IsNotExist(err) {
		t.Errorf("gc-root must be removed after convergence")
	}
}

func TestConvergePartialCommit(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 6, cfg.newClosure) // partial commit moved the profile
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.commitArgv = [][]string{
		{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure},
		{writeStub(t, stubs, "boot", `echo "$0 $@" >> @LOG@
exit 0
`)},
	}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.invariantTarget = cfg.newClosure
	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999}),
		rec(cfg.key, 2, guard.EventState, guard.Record{St: string(stateCommitting), PID: 999}),
		rec(cfg.key, 3, guard.EventCommitStart, guard.Record{}),
	)

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d", code)
	}

	log := readLog(t, slot)
	assertContains(t, log, guard.EventCommitted)
	// Seq continues from the last record (spec 7): the appended COMMITTED is seq 4+.
	m, ok := findRecord(t, slot, cfg.key, guard.EventCommitted)
	if !ok || m.Seq < 4 {
		t.Fatalf("seq must continue across writers: %+v", m)
	}
}

func TestConvergeTerminalNoOp(t *testing.T) {
	withFastViewer(t)
	slot := t.TempDir()
	cfg := newTestCfg(slot)
	cfg.key = "testkey"
	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999}),
		rec(cfg.key, 2, guard.EventReverted, guard.Record{St: string(stateReverted), RC: 0}),
		rec(cfg.key, 3, guard.EventExit, guard.Record{St: string(stateReverted)}),
	)

	before, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d", code)
	}

	after, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Fatalf("terminal slot must be a no-op; log changed:\n%s\nvs\n%s", before, after)
	}
}

func TestConvergeNoLog(t *testing.T) {
	withFastViewer(t)
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	// Generation arithmetic: the profile advanced to gen 5 without activation
	// evidence; the rollback target is the previous generation's closure (gen 4).
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/gen4", "/nix/store/gen5", 4
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.newClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d", code)
	}

	inv := invocations(t, stubs)

	joined := strings.Join(inv, "\n")
	if !strings.Contains(joined, "--delete-generations 5") {
		t.Fatalf("the advanced generation must be deleted: %v", inv)
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.oldClosure {
		t.Errorf("profile after no-log convergence: %q err=%v", got, err)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "no-log convergence"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}

	assertContains(t, readLog(t, slot), guard.EventReverted)
}
