package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// inspectStdout captures inspectRun's single JSON verdict.
func inspectStdout(t *testing.T, dir string) (int, string) {
	t.Helper()

	old := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = w
	code := inspectRun(dir)

	w.Close()

	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return code, string(out)
}

// decodeVerdict parses one inspect verdict.
func decodeVerdict(t *testing.T, out string) inspectVerdict {
	t.Helper()

	var v inspectVerdict

	err := json.Unmarshal([]byte(strings.TrimSpace(out)), &v)
	if err != nil {
		t.Fatalf("verdict is not one JSON object: %v; out:\n%s", err, out)
	}

	return v
}

// TestInspectGoldenTerminal pins the verdict shape for a terminal slot: the
// fold fields, the TXN embedding verbatim, the key, and the lock state.
func TestInspectGoldenTerminal(t *testing.T) {
	slot := t.TempDir()
	writeLog(t, slot,
		rec("k", 1, guard.EventHello, guard.Record{PID: 42, Old: "/nix/store/old", New: "/nix/store/new", Gen: 5, Mode: "switch", Tier: "full"}),
		rec("k", 2, guard.EventTxn, guard.Record{AA: `[["/nix/store/new/bin/activate"]]`, CA: `[["nix-env","--set","/nix/store/new"]]`, RA: `[["/nix/store/old/bin/activate"]]`, IV: "/nix/store/new"}),
		rec("k", 3, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0, Err: "noise excerpt"}),
		rec("k", 4, guard.EventExit, guard.Record{St: string(stateCommitted)}),
		"some child output line",
	)

	code, out := inspectStdout(t, slot)
	if code != guard.InspectExitOK {
		t.Fatalf("inspect exit: %d; out:\n%s", code, out)
	}

	v := decodeVerdict(t, out)
	if v.Status != string(stateCommitted) || !v.Terminal || v.Old != "/nix/store/old" || v.New != "/nix/store/new" || v.Gen != 5 || v.Mode != "switch" || v.Tier != "full" || v.RC != 0 || v.Key != "k" || v.Lock {
		t.Fatalf("wrong verdict: %+v", v)
	}

	if v.Err != "noise excerpt" {
		t.Fatalf("the err excerpt must surface: %q", v.Err)
	}

	if v.Txn == nil || v.Txn.IV != "/nix/store/new" || !strings.Contains(v.Txn.CA, "--set") {
		t.Fatalf("the TXN embedding must surface: %+v", v.Txn)
	}
}

// TestInspectGoldenActive pins a live (non-terminal) slot.
func TestInspectGoldenActive(t *testing.T) {
	slot := t.TempDir()
	writeLog(t, slot,
		rec("k", 1, guard.EventHello, guard.Record{PID: 42, Gen: 5}),
		rec("k", 2, guard.EventState, guard.Record{St: string(stateActivating), PID: 42}),
	)

	code, out := inspectStdout(t, slot)
	if code != guard.InspectExitOK {
		t.Fatalf("inspect exit: %d", code)
	}

	v := decodeVerdict(t, out)
	if v.Status != string(stateActivating) || v.Terminal || v.Txn != nil {
		t.Fatalf("wrong verdict: %+v", v)
	}
}

// TestInspectLegacyAndGarbage pins the unparseable branch: the lock state
// stays truthful (the deployer's legacy sweep relies on it) and the exit code
// is InspectExitUnparseable.
func TestInspectLegacyAndGarbage(t *testing.T) {
	t.Run("legacy slot", func(t *testing.T) {
		slot := t.TempDir()

		err := os.WriteFile(filepath.Join(slot, logName), []byte("@PG1 k 1 5 STATE status=activating\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		code, out := inspectStdout(t, slot)
		if code != guard.InspectExitUnparseable {
			t.Fatalf("legacy slot must exit unparseable, got %d; out:\n%s", code, out)
		}

		v := decodeVerdict(t, out)
		if v.Status != "legacy" {
			t.Fatalf("wrong verdict: %+v", v)
		}
	})

	t.Run("broken v2 lines", func(t *testing.T) {
		slot := t.TempDir()

		err := os.WriteFile(filepath.Join(slot, logName), []byte(guard.RecordPrefix+" {not json}\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		code, out := inspectStdout(t, slot)
		if code != guard.InspectExitUnparseable {
			t.Fatalf("broken records must exit unparseable, got %d", code)
		}

		v := decodeVerdict(t, out)
		if v.Status != "unknown" {
			t.Fatalf("wrong verdict: %+v", v)
		}
	})
}

// TestInspectMissingSlot pins the missing-slot exit.
func TestInspectMissingSlot(t *testing.T) {
	if code := inspectRun(t.TempDir()); code != guard.InspectExitNoSlot {
		t.Fatalf("missing slot must exit %d, got %d", guard.InspectExitNoSlot, code)
	}

	if code := inspectRun(filepath.Join(t.TempDir(), "absent")); code != guard.InspectExitNoSlot {
		t.Fatalf("missing slot dir must exit %d, got %d", guard.InspectExitNoSlot, code)
	}
}

// TestInspectLockStateTruthful pins the lock probe: a held log lock reads
// true, a free one false, and both from the SAME fresh-OFD transient grab the
// production inspect uses.
func TestInspectLockStateTruthful(t *testing.T) {
	slot := t.TempDir()

	err := os.WriteFile(filepath.Join(slot, logName), []byte(rec("k", 1, guard.EventState, guard.Record{St: "activating"})+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, out := inspectStdout(t, slot)
	if v := decodeVerdict(t, out); v.Lock {
		t.Fatalf("a free slot must report lock=false: %+v", v)
	}

	held, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer held.Close()

	_, out = inspectStdout(t, slot)
	if v := decodeVerdict(t, out); !v.Lock {
		t.Fatalf("a held slot must report lock=true: %+v", v)
	}
}

// TestConvergeSelfLockBusy pins the self-lock: a held slot exits locked with
// zero mutations and an untouched log.
func TestConvergeSelfLockBusy(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "echo MUSTNOT >> @LOG@\nexit 0\n")}}

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999, Old: cfg.oldClosure, New: cfg.newClosure, Gen: 5}),
		rec(cfg.key, 2, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
	)

	before, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	held, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer held.Close()

	if code := convergeRun(cfg, false); code != guard.ConvergeExitLocked {
		t.Fatalf("a held slot must exit locked, got %d", code)
	}

	after, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Fatalf("a locked converge must not mutate the log")
	}

	if inv := invocations(t, stubs); len(inv) != 0 {
		t.Fatalf("a locked converge must not mutate the machine: %v", inv)
	}
}

// TestConvergeNoLogGenerationArithmetic pins the no-log branch (spec 9.3
// step 6): with no records and no argv OLD hint, the generation arithmetic
// resolves the rollback target from the profile's own generation links.
func TestConvergeNoLogGenerationArithmetic(t *testing.T) {
	t.Parallel()

	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	// No --old hint: the arithmetic must resolve the previous generation's
	// closure (gen 4) as the rollback target.
	cfg.newClosure = "/nix/store/gen5"
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.newClosure)

	linkErr := os.Symlink("/nix/store/gen4", filepath.Join(stubs, "system-4-link"))
	if linkErr != nil {
		t.Fatal(linkErr)
	}

	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "exit 0\n")}}

	if code := convergeRun(cfg, false); code != guard.ConvergeExitConverged {
		t.Fatalf("no-log arithmetic convergence must exit converged, got %d; log:\n%s", code, readLog(t, slot))
	}

	got, _, targetErr := guard.ProfileTarget(cfg.profilePath)
	if targetErr != nil || got != "/nix/store/gen4" {
		t.Errorf("profile after arithmetic convergence: %q err=%v", got, targetErr)
	}

	if _, ok := findRecord(t, slot, cfg.key, guard.EventReverted); !ok {
		t.Fatal("no REVERTED record")
	}
}

// TestConvergeTruncateClearsSlot pins the slot clear (spec 10.3): truncate the
// log in place, remove the gc-root, remove the persisted guardian binary.
func TestConvergeTruncateClearsSlot(t *testing.T) {
	slot := t.TempDir()
	cfg := newTestCfg(slot)

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}),
		rec(cfg.key, 2, guard.EventExit, guard.Record{St: string(stateCommitted)}),
	)

	for name, content := range map[string]string{
		gcRootName:         "/nix/store/old",
		guardianBinaryName: "\x7fELF-fake",
	} {
		err := os.WriteFile(filepath.Join(slot, name), []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	if code := convergeRun(cfg, true); code != guard.ConvergeExitConverged {
		t.Fatalf("converge exit: %d", code)
	}

	if st, err := os.Stat(filepath.Join(slot, logName)); err != nil || st.Size() != 0 {
		t.Fatalf("the log must be truncated: size=%d err=%v", st.Size(), err)
	}

	if _, err := os.Stat(filepath.Join(slot, gcRootName)); !os.IsNotExist(err) {
		t.Fatalf("the gc-root must be removed")
	}

	if _, err := os.Stat(guardianBinaryPath(slot)); !os.IsNotExist(err) {
		t.Fatalf("the persisted guardian binary must be removed")
	}
}

// TestConvergeFailedLeavesLog pins the failure contract: convergence failure
// keeps the log untouched even with --truncate, and the terminal
// REVERT_FAILED record names the failing step's argv and error (spec 7
// error fidelity, post-mortem writer side).
func TestConvergeFailedLeavesLog(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "exit 7\n")}}

	writeLog(t, slot,
		rec(cfg.key, 1, guard.EventHello, guard.Record{PID: 999, Old: cfg.oldClosure, New: cfg.newClosure, Gen: 5}),
		rec(cfg.key, 2, guard.EventState, guard.Record{St: string(stateActivating), PID: 999}),
	)

	before, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	if code := convergeRun(cfg, true); code != guard.ConvergeExitFailed {
		t.Fatalf("a failed convergence must exit failed, got %d", code)
	}

	after, err := os.ReadFile(filepath.Join(slot, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	// The log is left UNTRUNCATED: the original records all survive (the
	// convergence's own records append; only the clear is withheld).
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatalf("a failed convergence must leave the log untruncated")
	}

	// The post-mortem REVERT_FAILED record carries the failing step's own
	// argv and error (spec 7), not a bare status.
	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertFailed)
	if !ok {
		t.Fatal("no REVERT_FAILED record")
	}

	if want := `step ["` + filepath.Join(stubs, "revert") + `"]`; !strings.Contains(m.Err, want) {
		t.Fatalf("the post-mortem REVERT_FAILED excerpt must name the failing step: %q", m.Err)
	}

	if want := "exit status 7"; !strings.Contains(m.Err, want) {
		t.Fatalf("the post-mortem REVERT_FAILED excerpt must carry the step error: %q", m.Err)
	}
}

// TestExitCodeMatrix pins the per-verb exit-code vocabulary (spec 9.5) and
// its wiring: the shared constants carry the values and every local mapping
// routes through them.
func TestExitCodeMatrix(t *testing.T) {
	// The vocabulary values themselves (spec 9.5), pinned per verb block.
	guardian := map[int]int{ // the guardian outcome vocabulary
		guard.ExitCommitted:          0,
		guard.ExitReverted:           2,
		guard.ExitRevertFailed:       3,
		guard.ExitFailedPrecondition: 4,
		guard.ExitActivationExited:   5,
		guard.ExitCancelled:          6,
		guard.ExitUsage:              64,
	}

	ctl := map[int]int{
		guard.CtlExitConsumed:    0,
		guard.CtlExitDead:        3,
		guard.CtlExitNoSlot:      4,
		guard.CtlExitRefused:     5,
		guard.CtlExitAckTimedOut: 7,
	}

	converge := map[int]int{
		guard.ConvergeExitConverged: 0,
		guard.ConvergeExitFailed:    1,
		guard.ConvergeExitLocked:    3,
	}

	inspect := map[int]int{
		guard.InspectExitOK:          0,
		guard.InspectExitUnparseable: 1,
		guard.InspectExitNoSlot:      4,
	}

	probe := map[int]int{
		guard.LockProbeExitFree: 0,
		guard.LockProbeExitLive: 3,
	}

	for _, table := range []map[int]int{guardian, ctl, converge, inspect, probe} {
		for code, want := range table {
			if code != want {
				t.Errorf("vocabulary drifted: constant %d != spec %d", code, want)
			}
		}
	}

	// The state mapping routes through the shared vocabulary.
	states := map[txState]int{
		stateCommitted:          guard.ExitCommitted,
		stateReverted:           guard.ExitReverted,
		stateRevertFailed:       guard.ExitRevertFailed,
		stateFailedPrecondition: guard.ExitFailedPrecondition,
		stateActivationExited:   guard.ExitActivationExited,
	}

	for st, want := range states {
		if got := exitCodeFor(st); got != want {
			t.Errorf("exitCodeFor(%s) = %d, want %d", st, got, want)
		}
	}

	// The relay's classification is the same mapping (spot-check through the
	// terminal fold of a committed slot).
	slot := t.TempDir()
	writeLog(t, slot, rec("k", 1, guard.EventCommitted, guard.Record{St: string(stateCommitted), RC: 0}))

	if code := classifyTail(&startConfig{dir: slot}); code != guard.ExitCommitted {
		t.Fatalf("relay classification drifted: %d", code)
	}
}
