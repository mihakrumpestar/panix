package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// TestRelayClassificationTable pins the EOF classification (spec 9.1, EOF !=
// death): a terminal record for the recovered key maps to the viewer exit
// codes; a non-terminal or empty tail maps to cancelled (6), which sends
// panix down its inline-converge path.
func TestRelayClassificationTable(t *testing.T) {
	cases := []struct {
		name string
		recs []string
		want int
	}{
		{"committed", []string{
			rec("k", 1, guard.EventHello, guard.Record{PID: 1}),
			rec("k", 2, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
			rec("k", 3, guard.EventExit, guard.Record{St: string(stateCommitted)}),
		}, guard.ExitCommitted},
		{"reverted", []string{
			rec("k", 1, guard.EventRevertStart, guard.Record{Rs: "test"}),
			rec("k", 2, guard.EventReverted, guard.Record{St: string(stateReverted)}),
			rec("k", 3, guard.EventExit, guard.Record{St: string(stateReverted)}),
		}, guard.ExitReverted},
		{"revert_failed", []string{
			rec("k", 1, guard.EventRevertFailed, guard.Record{St: string(stateRevertFailed), RC: 1}),
		}, guard.ExitRevertFailed},
		{"failed_precondition", []string{
			rec("k", 1, guard.EventFailedPrecondition, guard.Record{St: string(stateFailedPrecondition)}),
		}, guard.ExitFailedPrecondition},
		{"activation_exited", []string{
			rec("k", 1, guard.EventExit, guard.Record{St: string(stateActivationExited), RC: 0}),
		}, guard.ExitActivationExited},
		{"non_terminal", []string{
			rec("k", 1, guard.EventHello, guard.Record{PID: 1}),
			rec("k", 2, guard.EventState, guard.Record{St: string(stateActivating)}),
		}, guard.ExitCancelled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := t.TempDir()
			writeLog(t, slot, tc.recs...)

			cfg := &startConfig{dir: slot}
			if code := classifyTail(cfg); code != tc.want {
				t.Fatalf("classifyTail = %d, want %d", code, tc.want)
			}
		})
	}

	t.Run("empty log", func(t *testing.T) {
		slot := t.TempDir()

		err := os.WriteFile(filepath.Join(slot, logName), nil, 0o600)
		if err != nil {
			t.Fatal(err)
		}

		if code := classifyTail(&startConfig{dir: slot}); code != guard.ExitCancelled {
			t.Fatalf("empty tail must classify cancelled, got %d", code)
		}
	})

	t.Run("missing log", func(t *testing.T) {
		if code := classifyTail(&startConfig{dir: t.TempDir()}); code != guard.ExitCancelled {
			t.Fatalf("missing log must classify cancelled, got %d", code)
		}
	})
}

// TestRelayLoopEOF pins the relay's EOF behavior: the event pipe closing is
// classified from the tail once, stdin frames still ride to the command pipe
// before the relay exits, and no convergence ever runs.
func TestRelayLoopEOF(t *testing.T) {
	slot := t.TempDir()

	evtR, evtW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	defer cmdR.Close()

	_ = evtW.Close() // instant EOF: the guardian never wired the pipes

	code := relayLoop(evtR, cmdW, strings.NewReader("{\"c\":\"confirm\",\"rid\":1}\n"), &startConfig{dir: slot})
	_ = evtR.Close()

	if code != guard.ExitCancelled {
		t.Fatalf("spawn-failure EOF shape must be cancelled, got %d", code)
	}

	// The frame must have crossed into the command pipe before the relay
	// closed it (never block stdin delivery on the event side).
	buf, rerr := io.ReadAll(cmdR)
	if rerr != nil && !strings.Contains(string(buf), "confirm") {
		t.Fatalf("stdin frame must reach the command pipe: err=%v data=%q", rerr, string(buf))
	}
}

// TestStartMutationSequence drives the start-phase mutations with stubs: the
// gc-root argv, the boot-set argv, the OLD capture landing in the guardian
// argv, and the boot-mode ordering (capture BEFORE the set).
func TestStartMutationSequence(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure = "" // the deployer no longer supplies it
	cfg.newClosure = "/nix/store/new"
	cfg.gen = 5
	cfg.profilePath = fakeProfile(t, stubs, 5, "/nix/store/old")
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.nixStore = writeStub(t, stubs, "nix-store", "echo \"$0 $@\" >> @LOG@\nexit 0\n")
	cfg.sweep = true
	cfg.gcRootTarget = "/nix/store/new"

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		t.Fatal(err)
	}

	defer mut.logw.Close()

	if err := runStartMutations(mut); err != nil {
		t.Fatalf("mutations failed: %v", err)
	}

	// The OLD capture: post-converge profile state, in the guardian argv.
	if cfg.oldClosure != "/nix/store/old" || cfg.gen != 5 {
		t.Fatalf("capture OLD drifted: old=%q gen=%d", cfg.oldClosure, cfg.gen)
	}

	argv := strings.Join(startArgv(cfg), " ")
	if !strings.Contains(argv, "--old /nix/store/old") {
		t.Fatalf("the guardian argv must carry the captured OLD: %s", argv)
	}

	inv := strings.Join(invocations(t, stubs), "\n")
	if !strings.Contains(inv, "--add-root "+filepath.Join(slot, gcRootName)+" --indirect --realise /nix/store/new") {
		t.Fatalf("gc-root argv drifted: %s", inv)
	}
}

// TestStartBootModeOrdering pins the mutation order that keeps boot mode's
// rollback target honest: OLD is captured BEFORE the pre-start --set NEW, so
// the guardian argv carries the pre-deploy generation.
func TestStartBootModeOrdering(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.mode = "boot"
	cfg.newClosure = "/nix/store/new"
	cfg.profilePath = fakeProfile(t, stubs, 5, "/nix/store/old")
	cfg.nixEnv = writeStub(t, stubs, nixEnvName(), nixEnvBody())
	cfg.bootSet = cfg.newClosure

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		t.Fatal(err)
	}

	defer mut.logw.Close()

	if err := runStartMutations(mut); err != nil {
		t.Fatalf("mutations failed: %v", err)
	}

	if cfg.oldClosure != "/nix/store/old" {
		t.Fatalf("boot mode must capture the PRE-set profile as OLD, got %q", cfg.oldClosure)
	}

	if got, gen, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != cfg.newClosure {
		t.Fatalf("boot-set must have run: got %q gen=%d err=%v", got, gen, err)
	}
}

// nixEnvName names the nix-env stub for the boot ordering test.
func nixEnvName() string { return "nix-env" }

// TestStartSweepConvergesAndTruncates pins the pre-start sweep: a
// non-terminal previous transaction converges from its OWN TXN lists (never
// the argv fallback), then the log truncates in place.
func TestStartSweepConvergesAndTruncates(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.newClosure = "/nix/store/new2"
	cfg.gen = 5
	cfg.profilePath = fakeProfile(t, stubs, 6, "/nix/store/broken") // the interrupted tx left the profile moved
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.nixStore = writeStub(t, stubs, "nix-store", "exit 0\n")
	cfg.sweep = true
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revertArgvStub", "echo ARGVFALLBACK >> @LOG@\nexit 0\n")}}

	txnRevert := writeStub(t, stubs, "revertTxnStub", "echo TXNLIST >> @LOG@\nexit 0\n")
	writeLog(t, slot,
		rec("prev", 1, guard.EventHello, guard.Record{PID: 999, Old: "/nix/store/old", New: "/nix/store/broken", Gen: 5}),
		rec("prev", 2, guard.EventTxn, guard.Record{RA: mustJSON([][]string{{txnRevert}})}),
		rec("prev", 3, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
		rec("prev", 4, guard.EventRevertStart, guard.Record{Rs: "interrupted"}),
	)

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		t.Fatal(err)
	}

	defer mut.logw.Close()

	if err := runStartMutations(mut); err != nil {
		t.Fatalf("sweep failed: %v", err)
	}

	inv := strings.Join(invocations(t, stubs), "\n")
	if !strings.Contains(inv, "TXNLIST") {
		t.Fatalf("the sweep must converge from the TXN lists: %s", inv)
	}

	if strings.Contains(inv, "ARGVFALLBACK") {
		t.Fatalf("the argv fallback must not override the TXN lists: %s", inv)
	}

	if got, _, err := guard.ProfileTarget(cfg.profilePath); err != nil || got != "/nix/store/old" {
		t.Fatalf("profile after the sweep: %q err=%v", got, err)
	}

	// The truncation: the fresh transaction starts on a clean log.
	if st, err := os.Stat(filepath.Join(slot, logName)); err != nil || st.Size() != 0 {
		t.Fatalf("the log must be truncated after the sweep: size=%d err=%v", st.Size(), err)
	}
}

// TestStartSweepTerminalSlot pins the terminal branch: a terminal slot is
// just truncated, with zero convergence steps.
func TestStartSweepTerminalSlot(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.sweep = true
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "echo MUSTNOT >> @LOG@\nexit 0\n")}}

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
		rec(cfg.key, 2, guard.EventExit, guard.Record{St: string(stateCommitted)}),
	)

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		t.Fatal(err)
	}

	defer mut.logw.Close()

	if err := runStartMutations(mut); err != nil {
		t.Fatalf("sweep failed: %v", err)
	}

	if inv := invocations(t, stubs); len(inv) != 0 {
		t.Fatalf("a terminal slot must not converge: %v", inv)
	}

	if st, err := os.Stat(filepath.Join(slot, logName)); err != nil || st.Size() != 0 {
		t.Fatalf("the log must be truncated: size=%d err=%v", st.Size(), err)
	}
}

// TestStartMutationFailureRecord pins the FAILED_PRECONDITION record path: a
// failing mutation writes the record with the new key, reports the error
// that exits nonzero, and that error translates to the spec 9.5
// failed-precondition exit code (4).
func TestStartMutationFailureRecord(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.gcRootTarget = "/nix/store/new"
	cfg.nixStore = writeStub(t, stubs, "nix-store", "echo store exploded >&2\nexit 7\n")

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		t.Fatal(err)
	}

	defer mut.logw.Close()

	err = runStartMutations(mut)
	if err == nil {
		t.Fatal("a failing mutation must fail the start phase")
	}

	if code := exitCodeForError(err); code != guard.ExitFailedPrecondition {
		t.Fatalf("a failed start mutation must exit %d, got %d", guard.ExitFailedPrecondition, code)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventFailedPrecondition)
	if !ok || m.St != string(stateFailedPrecondition) {
		t.Fatalf("the failure must be recorded: %+v", m)
	}

	if !strings.Contains(m.Err, "gc root creation failed") {
		t.Fatalf("the record must name the failed mutation: %q", m.Err)
	}
}

// TestExitCodeForErrorTranslation pins the verb-error exit mapping (spec 9.5):
// the start-mutation sentinel exits failed_precondition (4) including through
// the failPrecondition wrap chain, and any other verb error keeps the generic
// failure exit (1).
func TestExitCodeForErrorTranslation(t *testing.T) {
	if code := exitCodeForError(fmt.Errorf("%w: gc root creation failed: boom", errStartMutation)); code != guard.ExitFailedPrecondition {
		t.Fatalf("errStartMutation must exit %d, got %d", guard.ExitFailedPrecondition, code)
	}

	// The sweep, gc-root, boot-set and step sentinels surface only wrapped by
	// failPrecondition; the wrap must carry them to the same exit code.
	if code := exitCodeForError(fmt.Errorf("%w: %w", errStartMutation, errMutationStep)); code != guard.ExitFailedPrecondition {
		t.Fatalf("a wrapped mutation failure must exit %d, got %d", guard.ExitFailedPrecondition, code)
	}

	// The plain dynamic error IS the test's subject: it pins the fallback
	// mapping for non-errStartMutation errors (generic failure exit, 1).
	//nolint:err113 // the unclassifiable plain error is the fixture, not a defect
	if code := exitCodeForError(errors.New("plain verb failure")); code != 1 {
		t.Fatalf("a plain verb error must exit 1, got %d", code)
	}
}

// TestConvergePostMortemRestoresProfile pins the post-mortem profile
// restoration (spec 9.3): the deployer's converge argv carries no --profile,
// so the fold must supply the transaction's own profile path from the HELLO
// pf. With the profile restored, the revert's profile-advance shape runs the
// nix-env --set back-to-old step and the converge reports REVERTED (writer d,
// exit 0) instead of a profile-less REVERT_FAILED; before the fix the TXN
// revert succeeded but converge could never report success.
func TestConvergePostMortemRestoresProfile(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	profile := fakeProfile(t, stubs, 6, "/nix/store/new") // the profile advanced to NEW (gen 6)
	cfg := newTestCfg(slot)
	cfg.key = ""         // the post-mortem argv passes no --key: recover the last writer's
	cfg.profilePath = "" // no argv profile: the fold must restore it from the records
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())

	revert := writeStub(t, stubs, "revert", "echo REVERT >> @LOG@\nexit 0\n")
	writeLog(t, slot,
		rec("prev", 1, guard.EventHello, guard.Record{PID: 999, Pf: profile, Old: "/nix/store/old", New: "/nix/store/new", Gen: 5}),
		rec("prev", 2, guard.EventTxn, guard.Record{RA: mustJSON([][]string{{revert}})}),
		rec("prev", 3, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
	)

	if code := convergeRun(cfg, false); code != guard.ConvergeExitConverged {
		t.Fatalf("post-mortem converge exit: %d; log:\n%s", code, readLog(t, slot))
	}

	m, ok := findRecord(t, slot, "prev", guard.EventReverted)
	if !ok || m.W != guard.WriterDecide || m.St != string(stateReverted) {
		t.Fatalf("the post-mortem must report REVERTED from writer d: %+v ok=%v", m, ok)
	}

	inv := strings.Join(invocations(t, stubs), "\n")
	if !strings.Contains(inv, "--set /nix/store/old") {
		t.Fatalf("the profile-advance shape must run the nix-env --set back-to-old step: %s", inv)
	}

	if got, _, err := guard.ProfileTarget(profile); err != nil || got != "/nix/store/old" {
		t.Fatalf("the profile must be restored to OLD: %q err=%v", got, err)
	}
}

// TestConvergePrefersTxnLists is the mode-drift regression: decide converges
// the interrupted transaction with ITS OWN TXN lists, never the argv lists
// composed for the current deploy.
func TestConvergePrefersTxnLists(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	// The argv fallback lists (what the CURRENT deploy would use).
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revertArgv", "echo ARGVFALLBACK >> @LOG@\nexit 0\n")}}
	// The interrupted transaction's own lists (different mode semantics).
	txnRevert := writeStub(t, stubs, "revertTxn", "echo TXNLIST >> @LOG@\nexit 0\n")

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999, Old: cfg.oldClosure, New: cfg.newClosure, Gen: 5}),
		rec(cfg.key, 2, guard.EventTxn, guard.Record{RA: mustJSON([][]string{{txnRevert}})}),
		rec(cfg.key, 3, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
	)

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d; log:\n%s", code, readLog(t, slot))
	}

	inv := strings.Join(invocations(t, stubs), "\n")
	if !strings.Contains(inv, "TXNLIST") {
		t.Fatalf("decide must run the TXN lists: %s", inv)
	}

	if strings.Contains(inv, "ARGVFALLBACK") {
		t.Fatalf("decide must not recompose from the argv lists (mode drift): %s", inv)
	}
}

// TestConvergeFallsBackToArgvLists pins the fallback order: with no TXN record
// the argv lists still converge the slot.
func TestConvergeFallsBackToArgvLists(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revertArgv", "echo ARGVFALLBACK >> @LOG@\nexit 0\n")}}

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999, Old: cfg.oldClosure, New: cfg.newClosure, Gen: 5}),
		rec(cfg.key, 2, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
	)

	if code := convergeRun(cfg, false); code != 0 {
		t.Fatalf("decide exit: %d", code)
	}

	if inv := strings.Join(invocations(t, stubs), "\n"); !strings.Contains(inv, "ARGVFALLBACK") {
		t.Fatalf("without a TXN record the argv lists must run: %s", inv)
	}
}

// TestStartLockRefusal pins the unchanged lock-refusal verdict: a live
// transaction makes start fail with ErrSlotLocked and zero effects.
func TestStartLockRefusal(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.profilePath = fakeProfile(t, stubs, 5, "/nix/store/old")

	l1, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer l1.Close()

	if _, err := openLockedLog(slot); !errors.Is(err, ErrSlotLocked) {
		t.Fatalf("want ErrSlotLocked, got %v", err)
	}
}

// TestNilStdioReExecPin pins the spec 6.1 invariant: the self re-exec never
// leaves stdio nil (Go maps nil to /dev/null, which is silent wire death).
func TestNilStdioReExecPin(t *testing.T) {
	dir := t.TempDir()

	l1, err := openLockedLog(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer l1.Close()

	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("no sleep binary: %v", err)
	}

	cmd, err := spawnDetached(sleepBin, []string{"0.2"}, nil, l1.file, guardianFDs{lock: l1.file})
	if err != nil {
		t.Fatal(err)
	}

	if cmd.Stdin == nil || cmd.Stdout == nil || cmd.Stderr == nil {
		t.Fatalf("stdio must be explicit, never nil: in=%v out=%v err=%v", cmd.Stdin, cmd.Stdout, cmd.Stderr)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper run: %v", err)
	}
}

// TestSpawnFailureBranch pins the spawn-failure classification: a spawn error
// leaves the cancelled shape (6) from an empty tail, without convergence.
func TestSpawnFailureBranch(t *testing.T) {
	slot := t.TempDir()

	if err := os.WriteFile(filepath.Join(slot, logName), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := newTestCfg(slot)

	l1, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer l1.Close()

	// A guardian binary path that cannot exec: the spawn fails before any
	// record exists.
	if _, err := spawnDetached(filepath.Join(slot, "missing-guardian"), []string{"start"}, nil, l1.file, guardianFDs{lock: l1.file}); err == nil {
		t.Fatal("a missing guardian binary must fail the spawn")
	}

	if code := classifyTail(cfg); code != guard.ExitCancelled {
		t.Fatalf("spawn failure must classify cancelled, got %d", code)
	}
}

// TestTxnRecordInRun pins the TXN emission on a normal transaction: HELLO is
// followed by the TXN embedding, and the lists round trip.
func TestTxnRecordInRun(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)

	st, log := runTx(t, cfg)
	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventTxn)
	if !ok {
		t.Fatalf("no TXN record; log:\n%s", log)
	}

	if !strings.Contains(m.AA, "activate") || !strings.Contains(m.CA, "--set") || !strings.Contains(m.RA, "revert") || m.IV != cfg.newClosure {
		t.Fatalf("TXN embedding incomplete: %+v", m)
	}

	// The fold decodes the lists for the deployer and the sweep.
	b, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	folded := guard.Fold(guard.ParseRecords(string(b), cfg.key))
	if len(folded.TxnCommit) != 1 || folded.TxnInvariant != cfg.newClosure {
		t.Fatalf("fold must decode the TXN lists: %+v", folded)
	}
}

// TestTxnOverBudgetAborts pins the budget abort: an over-budget embedding
// fails the transaction with a minimal FAILED_PRECONDITION, exit code 4, and
// zero mutations (the log is the existence proof).
func TestTxnOverBudgetAborts(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{"/nix/store/" + strings.Repeat("a", 5000) + "/activate"}}
	cfg.commitArgv = [][]string{{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "exit 0\n")}}
	cfg.invariantTarget = cfg.newClosure

	st, log := runTx(t, cfg)
	if st != stateFailedPrecondition {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	if code := exitCodeFor(st); code != guard.ExitFailedPrecondition {
		t.Fatalf("over-budget abort must exit 4, got %d", code)
	}

	if _, ok := findRecord(t, slot, cfg.key, guard.EventTxn); ok {
		t.Fatal("no TXN record may ship when the embedding is over budget")
	}

	if inv := invocations(t, stubs); len(inv) != 0 {
		t.Fatalf("zero mutations expected on the budget abort, got %v", inv)
	}

	assertContains(t, log, guard.EventFailedPrecondition, guard.EventExit)
}

// TestPreviousGenClosure pins the generation arithmetic fallback used when
// no record names the rollback target.
func TestPreviousGenClosure(t *testing.T) {
	stubs := t.TempDir()
	profile := fakeProfile(t, stubs, 6, "/nix/store/gen6")

	// A previous generation link: gen 5.
	err := os.Symlink("/nix/store/gen5", filepath.Join(stubs, "system-5-link"))
	if err != nil {
		t.Fatal(err)
	}

	old, gen, ok := previousGenClosure(profile)
	if !ok || old != "/nix/store/gen5" || gen != 5 {
		t.Fatalf("previous gen closure: %q gen=%d ok=%v", old, gen, ok)
	}

	// Gen 1 has no previous: the arithmetic cannot fabricate a target.
	profile1 := fakeProfile(t, t.TempDir(), 1, "/nix/store/gen1")
	if _, _, ok := previousGenClosure(profile1); ok {
		t.Fatal("gen 1 must have no previous generation")
	}
}

// TestAckRecordsWithoutRidNeverAcks pins the ack correlation rule on the
// record shape: signal-driven acks carry no rid; the deployer's matching
// (rid equality) can never fire on them.
func TestAckRecordsWithoutRidNeverAcks(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 200 * time.Millisecond
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", "exit 0\n")}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "exit 0\n")}}

	// No wire, no signal: the window expires -> CONFIRM_IGNORED without rid.
	st, log := runTx(t, cfg)
	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventConfirmIgnored)
	if !ok || m.Rid != 0 {
		t.Fatalf("signal-era ack must carry no rid: %+v", m)
	}
}
