// Command panix-guard supervises a guarded activation transaction on the target machine.
// The protocol, state machine, and semantics are specified in
// docs/design/activation-guard.md (the implementation contract).
//
// Subcommands: start, attach, ctl, converge, inspect.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// Internal environment variables used across the exec chain (spec 4.1 step 3, 6.1).
const (
	lockFDEnv   = "PANIX_GUARD_LOCK_FD"  // inherited transaction lock fd number (ExtraFiles position 3)
	cmdFDEnv    = "PANIX_GUARD_CMD_FD"   // command pipe read end: wire frames in (position 4)
	evtFDEnv    = "PANIX_GUARD_EVT_FD"   // event pipe write end: wire records out (position 5)
	detachedEnv = "PANIX_GUARD_DETACHED" // marks the detached guardian instance ("1")
)

// startConfig carries the resolved spawn argv (spec 6.5). The guardian derives the
// runtime profile rules (restore, generation cleanup, invariant) from the profile path;
// nothing is re-resolved from the target's environment. The start-phase mutation
// targets (sweep, gc root, boot set, nix-store) ride the start argv and are consumed
// by the parent before the detached guardian is spawned (spec 4.1 step 5).
type startConfig struct {
	dir                   string
	key                   string
	profilePath           string
	nixEnv                string
	nixStore              string
	oldClosure            string
	newClosure            string
	gen                   int64
	mode                  string
	tier                  string
	confirmation          string
	activationTimeout     time.Duration
	confirmTimeout        time.Duration
	rebootOnRevertFailure bool
	healthChecksLocal     []string
	builtinUnitCheck      bool
	activationArgv        [][]string
	commitArgv            [][]string
	revertArgv            [][]string
	invariantTarget       string
	sweep                 bool
	gcRootTarget          string
	bootSet               string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(guard.ExitUsage)
	}

	var err error

	switch os.Args[1] {
	case "start":
		err = runStart(os.Args[2:])
	case "attach":
		err = runAttach(os.Args[2:])
	case "ctl":
		err = runCtl(os.Args[2:])
	case "converge":
		err = runConverge(os.Args[2:])
	case "inspect":
		err = runInspect(os.Args[2:])
	default:
		usage()
		os.Exit(guard.ExitUsage)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)
		os.Exit(exitCodeForError(err))
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: panix-guard <command> [flags]

commands:
  start     run a guarded activation transaction: mutate under the lock, spawn
            the detached guardian, then relay the duplex wire (the live stream)
  attach    reconnect viewer: tail the slot log from an offset until a terminal state
  ctl       send confirm or revert-request to the live guardian and wait for the ack
  converge  offline convergence: self-lock, classify, converge, optional --truncate
  inspect   print one JSON verdict: status, terminal, old/new/gen/mode, rc, lock state`)
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	cfg := &startConfig{}
	fs.StringVar(&cfg.dir, "dir", "", "slot directory")
	fs.StringVar(&cfg.key, "key", "", "per-deploy key (deploy nonce)")
	fs.StringVar(&cfg.profilePath, "profile", "", "profile path owned by this transaction")
	fs.StringVar(&cfg.nixEnv, "nix-env", "", "absolute nix-env path for guardian-managed steps")
	// --old is consumed by the DETACHED guardian only: the deployer no longer
	// supplies it; start captures OLD itself after its convergence (spec 4.1
	// step 5) and passes the captured value in the guardian argv.
	fs.StringVar(&cfg.oldClosure, "old", "", "pre-deploy closure store path (rollback target; set by start)")
	fs.StringVar(&cfg.newClosure, "new", "", "new closure store path")
	fs.Int64Var(&cfg.gen, "gen", 0, "pre-deploy profile generation number")
	fs.StringVar(&cfg.mode, "mode", "", "resolved activation mode (switch/test/boot)")
	fs.StringVar(&cfg.tier, "tier", "", "tier class (full/standard/self-setting/minimal)")
	fs.StringVar(&cfg.confirmation, "confirmation", gateAuto, "confirmation gate (auto/magic)")
	fs.DurationVar(&cfg.activationTimeout, "activation-timeout", 15*time.Minute, "activation phase bound")
	fs.DurationVar(&cfg.confirmTimeout, "confirm-timeout", time.Minute, "magic tier confirmation window")
	fs.BoolVar(&cfg.rebootOnRevertFailure, "reboot-on-revert-failure", false, "reboot after a failed revert (gated)")
	healthChecksJSON := fs.String("health-checks-local", "[]", "JSON array of local check commands")
	fs.BoolVar(&cfg.builtinUnitCheck, "builtin-unit-check", false, "enable the builtin failed-units baseline check (nixos)")
	activationArgvJSON := fs.String("activation-argv", "[]", "JSON argv of the activation child (single step)")
	commitArgvJSON := fs.String("commit-argv", "[]", "JSON steps of the commit transaction")
	revertArgvJSON := fs.String("revert-argv", "[]", "JSON steps of the revert transaction")
	fs.StringVar(&cfg.invariantTarget, "invariant-target", "", "expected profile target after the transaction (empty skips)")
	// Start-phase mutations under start's own flock (spec 4.1 step 5).
	fs.BoolVar(&cfg.sweep, "sweep", false, "converge a non-terminal previous transaction and truncate the log")
	fs.StringVar(&cfg.gcRootTarget, "gc-root-target", "", "gc-root target closure (empty skips the step)")
	fs.StringVar(&cfg.bootSet, "boot-set", "", "closure to set on the profile before the transaction (boot mode; empty skips)")
	fs.StringVar(&cfg.nixStore, "nix-store", "", "absolute nix-store path for the gc-root step")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if cfg.dir == "" || cfg.key == "" || cfg.newClosure == "" || cfg.tier == "" {
		return errStartUsage
	}

	switch cfg.tier {
	case tierFull, tierStandard, tierSelfSetting, tierMinimal:
	default:
		return fmt.Errorf("%w: %q", errStartUnknownTier, cfg.tier)
	}

	switch cfg.confirmation {
	case gateAuto, gateMagic:
	default:
		return fmt.Errorf("%w: %q", errStartUnknownGate, cfg.confirmation)
	}

	var err error
	if cfg.healthChecksLocal, err = parseJSONStrings(*healthChecksJSON); err != nil {
		return err
	}

	if cfg.activationArgv, err = parseArgvSteps(*activationArgvJSON); err != nil {
		return err
	}

	if cfg.commitArgv, err = parseArgvSteps(*commitArgvJSON); err != nil {
		return err
	}

	if cfg.revertArgv, err = parseArgvSteps(*revertArgvJSON); err != nil {
		return err
	}

	if len(cfg.activationArgv) != 1 {
		return errStartActivationSteps
	}

	if os.Getenv(detachedEnv) == "1" {
		os.Exit(guardianEntry(cfg))
	}

	// Parent side (the pre-start spawn exec, spec 4.1 step 5): acquire the
	// transaction lock, mutate under it, spawn the detached guardian with the
	// duplex pipes, then BECOME the relay (spec 6.1). The lock releases when
	// this process exits; the guardian's inherited reference holds it beyond
	// that.
	return startRelay(cfg)
}

// startArgv rebuilds the detached guardian's argv: the spec 6.5 contract plus
// the OLD start captured itself. The start-phase mutation flags (sweep, gc
// root, boot set, nix store) are NOT re-passed: the guardian runs no
// mutations; its preconditions were already established under the same lock.
func startArgv(cfg *startConfig) []string {
	return []string{
		"start",
		"--dir", cfg.dir,
		"--key", cfg.key,
		"--profile", cfg.profilePath,
		"--nix-env", cfg.nixEnv,
		"--old", cfg.oldClosure,
		"--new", cfg.newClosure,
		"--gen", strconv.FormatInt(cfg.gen, 10),
		"--mode", cfg.mode,
		"--tier", cfg.tier,
		"--confirmation", cfg.confirmation,
		"--activation-timeout", cfg.activationTimeout.String(),
		"--confirm-timeout", cfg.confirmTimeout.String(),
		"--reboot-on-revert-failure=" + strconv.FormatBool(cfg.rebootOnRevertFailure),
		"--health-checks-local", mustJSON(cfg.healthChecksLocal),
		"--builtin-unit-check=" + strconv.FormatBool(cfg.builtinUnitCheck),
		"--activation-argv", mustJSON(cfg.activationArgv),
		"--commit-argv", mustJSON(cfg.commitArgv),
		"--revert-argv", mustJSON(cfg.revertArgv),
		"--invariant-target", cfg.invariantTarget,
	}
}

// guardianEntry is the detached guardian main loop (spec 6.1, 6.2): it holds the
// inherited transaction lock, which releases only when this process exits, and
// wires the duplex pipes before any record is emitted.
func guardianEntry(cfg *startConfig) int {
	lock := inheritedLockFile()
	if lock == nil {
		fmt.Fprintln(os.Stderr, "start: guardian must run detached (missing lock fd)")

		return 1
	}
	defer lock.Close()

	logw, err := OpenLogWriter(cfg.dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return 1
	}
	defer logw.Close()

	// The wire pipes open (and their CLOEXEC marks land) before the first
	// record: the relay must see every record from HELLO on. Without an
	// event pipe the machine runs signals-only (degraded mode) with no wire
	// writer at all, so no synthetic link records can appear.
	cmdIn, evtOut := inheritedWirePipes()

	var wire *wireWriter
	if evtOut != nil {
		wire = newWireWriter(evtOut, nil, nil)
	}

	t := newTransaction(cfg, logw, lock, wire, cmdIn)

	if wire != nil {
		wire.setHooks(
			func() {
				t.core.emitLogOnly(guard.EventLinkDegraded, guard.Record{Rs: "wire frame dropped (slow reader)"})
			},
			func(err error) { t.core.emitLogOnly(guard.EventLinkDown, guard.Record{Rs: wireDownReason(err)}) },
		)
	}

	st, panicked := runGuarded(t.run, func(err error) {
		// Terminal error record: log first, then wire best-effort (the dual
		// write already orders it that way), then the caller exits nonzero.
		t.core.emit(guard.EventExit, guard.Record{
			St:  string(stateFailedPrecondition),
			RC:  1,
			Err: err.Error(),
		})
	})

	wire.close() // terminal records: log-then-wire-then-exit (spec 6.1)

	if panicked {
		return 1
	}

	return exitCodeFor(st)
}

// runGuarded runs the state machine under the top-level recover (spec 6.1):
// a panic reports through onError (which emits the terminal error record,
// log first) and marks the outcome panicked so the caller exits nonzero.
func runGuarded(run func() txState, onError func(error)) (st txState, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true

			onError(fmt.Errorf("%w: %v", errGuardianPanic, r))
		}
	}()

	return run(), false
}

// exitCodeFor maps the terminal state to the guardian outcome vocabulary
// (spec 9.5, the shared per-verb table).
func exitCodeFor(st txState) int {
	switch st {
	case stateCommitted:
		return guard.ExitCommitted
	case stateReverted:
		return guard.ExitReverted
	case stateRevertFailed:
		return guard.ExitRevertFailed
	case stateFailedPrecondition:
		return guard.ExitFailedPrecondition
	case stateActivationExited:
		return guard.ExitActivationExited
	default:
		return guard.ExitCancelled
	}
}

// exitCodeForError maps a verb error to the process exit code (spec 9.5): a
// failed start-phase mutation already wrote the FAILED_PRECONDITION record
// (zero effects) and exits with the failed-precondition code; every other
// verb error keeps the generic failure exit.
func exitCodeForError(err error) int {
	if errors.Is(err, errStartMutation) {
		return guard.ExitFailedPrecondition
	}

	return 1
}

// parseJSONStrings parses a JSON array of strings (local check commands).
func parseJSONStrings(s string) ([]string, error) {
	var out []string

	err := json.Unmarshal([]byte(s), &out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errParseJSONStrings, err)
	}

	return out, nil
}

// parseArgvSteps parses a JSON array of argv arrays (spec 6.5) and rejects empty steps.
func parseArgvSteps(s string) ([][]string, error) {
	var steps [][]string

	err := json.Unmarshal([]byte(s), &steps)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errParseArgvSteps, err)
	}

	for i, step := range steps {
		if len(step) == 0 || step[0] == "" {
			return nil, fmt.Errorf("%w: step %d", errArgvStepEmpty, i)
		}
	}

	return steps, nil
}

// mustJSON serializes spawn argv payload fields; these values are built in-process and
// always serialize.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}

	return string(b)
}
