package main

import (
	"fmt"
	"github.com/mihakrumpestar/panix/internal/guard"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeStub creates an executable stub script in dir. The @LOG@ and @STATE@
// placeholders are replaced with paths inside the same dir so stubs can record
// invocations and change behavior between calls.
func writeStub(t *testing.T, dir, name, body string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	body = strings.ReplaceAll(body, "@LOG@", filepath.Join(dir, "invocations.log"))

	body = strings.ReplaceAll(body, "@STATE@", filepath.Join(dir, "state"))

	err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(p, 0o755); err != nil { //nolint:gosec // the stub must be executable; the quiet write satisfied G306
		t.Fatal(err)
	}

	return p
}

// invocations returns the argv lines the stubs recorded (one line per invocation).
func invocations(t *testing.T, dir string) []string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, "invocations.log")) //nolint:gosec // test fixture paths
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		t.Fatal(err)
	}

	var out []string

	for l := range strings.SplitSeq(strings.TrimRight(string(b), "\n"), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}

	return out
}

// fakeProfile builds profile -> system-N-link -> storePath and returns the profile path,
// emulating the NixOS profile layout the guardian resolves at runtime.
func fakeProfile(t *testing.T, dir string, gen int64, storePath string) string {
	t.Helper()

	link := filepath.Join(dir, fmt.Sprintf("system-%d-link", gen))

	err := os.Symlink(storePath, link)
	if err != nil {
		t.Fatal(err)
	}

	profile := filepath.Join(dir, "profile")

	err = os.Symlink(filepath.Base(link), profile)
	if err != nil {
		t.Fatal(err)
	}

	return profile
}

func newTestCfg(dir string) *startConfig {
	return &startConfig{
		dir:               dir,
		key:               "testkey",
		mode:              "switch",
		tier:              tierFull,
		confirmation:      gateAuto,
		activationTimeout: 5 * time.Second,
		confirmTimeout:    time.Second,
		healthChecksLocal: []string{},
	}
}

// runTx runs a transaction against a fresh slot and returns the terminal state and the
// full log. The lock and writer follow the production wiring.
func runTx(t *testing.T, cfg *startConfig) (txState, string) {
	t.Helper()

	if cfg.dir == "" {
		t.Fatal("runTx: cfg.dir must be set")
	}

	lock, err := openLockedLog(cfg.dir)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	defer lock.Close()

	logw, err := OpenLogWriter(cfg.dir)
	if err != nil {
		t.Fatalf("log writer: %v", err)
	}
	defer logw.Close()

	st := newTransaction(cfg, logw, lock.file, nil, nil).run()

	return st, readLog(t, cfg.dir)
}

func signalAfter(d time.Duration, sig syscall.Signal) {
	time.AfterFunc(d, func() {
		_ = syscall.Kill(os.Getpid(), sig)
	})
}

func assertContains(t *testing.T, log string, wants ...string) {
	t.Helper()

	for _, w := range wants {
		if !strings.Contains(log, w) {
			t.Errorf("log missing %q; log:\n%s", w, log)
		}
	}
}

// findRecord returns the first record of event in the slot log, decoded.
// Record field values travel as JSON, so tests must assert decoded fields,
// never raw text.
func findRecord(t *testing.T, dir, key, event string) (guard.Record, bool) {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		if r, ok := guard.ParseRecordLine(line, key); ok && r.Ev == event {
			return r, true
		}
	}

	return guard.Record{}, false
}

// nixEnvBody emulates nix-env: --set creates the next system-N-link generation and
// repoints the profile; --delete-generations is recorded and tolerated as a no-op
// (verified V6: a missing generation is a silent success).
func nixEnvBody() string {
	return `echo "$0 $@" >> @LOG@
if [ "$3" = "--set" ]; then
  d=$(dirname "$2")
  n=0
  for f in "$d"/*-link; do
    b=$(basename "$f")
    case "$b" in "*-link") continue ;; esac
    g=${b%-link}
    g=${g#*-}
    case "$g" in *[!0-9]*) continue ;; esac
    [ "$g" -gt "$n" ] && n=$g
  done
  n=$((n+1))
  ln -sfn "$4" "$d/system-$n-link"
  ln -sfn "system-$n-link" "$2"
fi
exit 0
`
}

const activateFailWithGenBody = `echo "$0 $@" >> @LOG@
ln -sfn "/nix/store/new" "@STUBS@/system-6-link"
ln -sfn "system-6-link" "@PROFILE@"
exit 3
`

func TestAutoSuccessSwitch(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
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

	err := os.WriteFile(filepath.Join(slot, gcRootName), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	st, log := runTx(t, cfg)
	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventHello, guard.EventCommitStart, guard.EventCommitted, guard.EventExit)

	if _, err := os.Stat(filepath.Join(slot, gcRootName)); !os.IsNotExist(err) {
		t.Errorf("gc-root must be removed at terminal state")
	}

	inv := invocations(t, stubs)
	if len(inv) != 3 {
		t.Fatalf("expected activation + 2 commit steps, got %v", inv)
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.newClosure {
		t.Errorf("profile after commit: %q err=%v", got, err)
	}
}

func TestActivationFailureReverts(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
echo boom >&2
exit 3
`)}}
	cfg.commitArgv = [][]string{
		{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure},
	}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, "boom", guard.EventRevertStart, guard.EventReverted)

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "activation failed (rc=3)"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}

	inv := invocations(t, stubs)
	for _, line := range inv {
		if strings.Contains(line, "--set") {
			t.Errorf("profile must not be restored when it was never moved: %v", inv)
		}

		if strings.Contains(line, "boot") {
			t.Errorf("commit must not run after a failed activation: %v", inv)
		}
	}

	if got, gen, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.oldClosure || gen != 5 {
		t.Errorf("profile must be untouched: %q gen=%d err=%v", got, gen, err)
	}
}

func TestActivationTimeoutReverts(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.activationTimeout = 200 * time.Millisecond
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
sleep 30
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	start := time.Now()

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventRevertStart, guard.EventReverted)

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "activation timed out after 200ms"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("timed-out child was not killed: %s", elapsed)
	}
}

func TestMagicConfirmCommits(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 2 * time.Second
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.commitArgv = [][]string{{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.invariantTarget = cfg.newClosure

	signalAfter(100*time.Millisecond, syscall.SIGUSR1)

	st, log := runTx(t, cfg)
	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventConfirmConsumed, guard.EventCommitted)
}

func TestMagicDeadlineReverts(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 150 * time.Millisecond
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventConfirmIgnored, guard.EventReverted)

	m, ok := findRecord(t, slot, cfg.key, guard.EventConfirmIgnored)
	if !ok {
		t.Fatal("no CONFIRM_IGNORED record")
	}

	if want := "window expired"; !strings.Contains(m.Rs, want) {
		t.Fatalf("ignore reason %q missing %q", m.Rs, want)
	}

	m, ok = findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "no confirmation within the window"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}
}

func TestPendingConfirmConsumed(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
sleep 0.4
exit 0
`)}}
	cfg.commitArgv = [][]string{{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.invariantTarget = cfg.newClosure

	signalAfter(100*time.Millisecond, syscall.SIGUSR1)

	start := time.Now()

	st, log := runTx(t, cfg)
	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventConfirmConsumed)

	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("pending confirm must skip the window, took %s", elapsed)
	}
}

func TestDeadlineBeatsPendingConfirm(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second
	cfg.activationTimeout = 300 * time.Millisecond
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
sleep 2
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	signalAfter(100*time.Millisecond, syscall.SIGUSR1)

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventConfirmIgnored, guard.EventReverted)
}

func TestRevertRequestDuringActivation(t *testing.T) {
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

	signalAfter(100*time.Millisecond, syscall.SIGUSR2)

	start := time.Now()

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventReverted)

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "revert requested during activation"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("revert request did not kill the activation child: %s", elapsed)
	}
}

func TestCommitFailureRestoresProfile(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.commitArgv = [][]string{
		{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure},
		{writeStub(t, stubs, "boot", `echo "$0 $@" >> @LOG@
echo bootloader install failed >&2
exit 1
`)},
	}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventCommitStart, guard.EventRevertStart, guard.EventReverted)

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "commit failed"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}

	inv := invocations(t, stubs)

	joined := strings.Join(inv, "\n")
	if !strings.Contains(joined, "--set") || !strings.Contains(joined, cfg.oldClosure) {
		t.Fatalf("restore --set OLD must run after a partial commit: %v", inv)
	}

	if !strings.Contains(joined, "--delete-generations") {
		t.Fatalf("the failed generation must be deleted: %v", inv)
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.oldClosure {
		t.Errorf("profile after revert: %q err=%v", got, err)
	}
}

func TestRevertDeletesCreatedGeneration(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.tier = tierSelfSetting
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	activate := writeStub(t, stubs, "activate", strings.ReplaceAll(strings.ReplaceAll(
		activateFailWithGenBody, "@STUBS@", stubs), "@PROFILE@", cfg.profilePath))
	cfg.activationArgv = [][]string{{activate}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	inv := invocations(t, stubs)

	joined := strings.Join(inv, "\n")
	if !strings.Contains(joined, "--delete-generations 6") {
		t.Fatalf("the generation created by the self-setting activation must be deleted: %v", inv)
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.oldClosure {
		t.Errorf("profile after revert: %q err=%v", got, err)
	}
}

func TestPreconditionMismatch(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/expected-old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, "/nix/store/other")
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateFailedPrecondition {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	assertContains(t, log, guard.EventFailedPrecondition, "precondition mismatch")

	if inv := invocations(t, stubs); len(inv) != 0 {
		t.Fatalf("zero effects expected on precondition failure, got %v", inv)
	}
}

func TestMinimalTier(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.tier = tierMinimal
	cfg.profilePath = ""

	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	err := os.WriteFile(filepath.Join(slot, gcRootName), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	st, log := runTx(t, cfg)
	if st != stateActivationExited {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	if code := exitCodeFor(st); code != guard.ExitActivationExited {
		t.Fatalf("exit code for %s: %d", st, code)
	}

	if _, err := os.Stat(filepath.Join(slot, gcRootName)); !os.IsNotExist(err) {
		t.Errorf("gc-root must be removed at terminal state")
	}
}

func TestBuiltinUnitCheckFails(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.builtinUnitCheck = true
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	writeStub(t, stubs, "systemctl", `echo "$0 $@" >> @LOG@
if [ -f @STATE@ ]; then
  echo "unitB loaded active failed -"
else
  touch @STATE@
  echo "unitA loaded active failed -"
fi
exit 0
`)
	t.Setenv("PATH", stubs+":"+os.Getenv("PATH"))

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "new failed units: unitB"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}
}

func TestLocalCheckFailure(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.healthChecksLocal = []string{"exit 3"}
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
exit 0
`)}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", `echo "$0 $@" >> @LOG@
exit 0
`)}}

	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatal("no REVERT_START record")
	}

	if want := "check failed (rc=3): exit 3"; !strings.Contains(m.Rs, want) {
		t.Fatalf("revert reason %q missing %q", m.Rs, want)
	}
}

// TestRevertFailureNamesTheFailingStep pins the terminal-record error
// fidelity (spec 7): a failed revert must name its own failing step's argv
// and error in the REVERT_FAILED record and the log narrative, never the
// activation error that triggered the revert. The failing step here is the
// fork/exec ENOENT class (the file is absent, so no child ever starts and no
// step output streams), which is exactly the shape the e2e boot-revert leg
// diagnosed blind before this record carried the step evidence.
func TestRevertFailureNamesTheFailingStep(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", `echo "$0 $@" >> @LOG@
echo boom >&2
exit 3
`)}}
	// The boot-mode revert shape: the OLD activation step points at a path
	// no closure ships, so every retry fails at fork/exec with no output.
	cfg.revertArgv = [][]string{{"/nix/store/missing-toplevel/etc/panix-e2e-boot-activation", "boot"}}

	st, log := runTx(t, cfg)
	if st != stateRevertFailed {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertFailed)
	if !ok {
		t.Fatal("no REVERT_FAILED record")
	}

	if want := `step ["/nix/store/missing-toplevel/etc/panix-e2e-boot-activation" "boot"]`; !strings.Contains(m.Err, want) {
		t.Fatalf("the REVERT_FAILED excerpt must name the failing step argv: %q", m.Err)
	}

	if want := "no such file or directory"; !strings.Contains(m.Err, want) {
		t.Fatalf("the REVERT_FAILED excerpt must carry the step's errno: %q", m.Err)
	}

	if strings.Contains(m.Err, "boom") {
		t.Fatalf("the REVERT_FAILED excerpt must not echo the activation error: %q", m.Err)
	}

	assertContains(t, log,
		`guard: step ["/nix/store/missing-toplevel/etc/panix-e2e-boot-activation" "boot"] failed:`,
		"no such file or directory")
}

func TestExitCodeFor(t *testing.T) {
	cases := map[txState]int{
		stateCommitted:          guard.ExitCommitted,
		stateReverted:           guard.ExitReverted,
		stateRevertFailed:       guard.ExitRevertFailed,
		stateFailedPrecondition: guard.ExitFailedPrecondition,
		stateActivationExited:   guard.ExitActivationExited,
	}
	for st, want := range cases {
		if got := exitCodeFor(st); got != want {
			t.Errorf("exitCodeFor(%s) = %d, want %d", st, got, want)
		}
	}
}
