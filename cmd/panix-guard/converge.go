package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// converge is the offline convergence verb (spec 9.3): it self-locks the slot
// (flock nonblocking; busy means a live transaction, exit 3, no mutation),
// classifies from the log tail (the last writer's key is recovered from the
// records themselves), converges a non-terminal transaction using the TXN
// record's own embedded step lists (argv fallback, then generation
// arithmetic), writes the terminal records, and with --truncate clears the
// slot: truncate the log in place, remove the gc-root, remove the persisted
// guardian binary (spec 10.3).

// guardianBinaryName is the persisted guardian binary's name inside the slot
// (the transfer path of the deployer, spec 10.3).
const guardianBinaryName = "panix-guard"

// guardianBinaryPath is the slot's persisted guardian binary.
func guardianBinaryPath(dir string) string { return filepath.Join(dir, guardianBinaryName) }

// runConverge parses the converge argv and exits with the converge exit code.
func runConverge(args []string) error {
	fs := flag.NewFlagSet("converge", flag.ContinueOnError)

	dir := fs.String("dir", "", "slot directory")
	key := fs.String("key", "", "per-deploy key (empty: recover the last writer's key)")
	profile := fs.String("profile", "", "profile path owned by the interrupted transaction")
	nixEnv := fs.String("nix-env", "", "absolute nix-env path")
	oldClosure := fs.String("old", "", "rollback target closure (fallback when no records name one)")
	newClosure := fs.String("new", "", "closure the interrupted transaction was deploying")
	gen := fs.Int64("gen", 0, "pre-deploy profile generation number (fallback)")
	commitArgvJSON := fs.String("commit-argv", "[]", "JSON steps of the commit transaction (fallback)")
	revertArgvJSON := fs.String("revert-argv", "[]", "JSON steps of the revert transaction (fallback)")
	invariant := fs.String("invariant-target", "", "expected profile target after commit (fallback)")
	truncate := fs.Bool("truncate", false, "clear the slot after convergence (the pre-start caller)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *dir == "" {
		return errConvergeUsage
	}

	commitArgv, err := parseArgvSteps(*commitArgvJSON)
	if err != nil {
		return err
	}

	revertArgv, err := parseArgvSteps(*revertArgvJSON)
	if err != nil {
		return err
	}

	cfg := &startConfig{
		dir:             *dir,
		key:             *key,
		profilePath:     *profile,
		nixEnv:          *nixEnv,
		oldClosure:      *oldClosure,
		newClosure:      *newClosure,
		gen:             *gen,
		commitArgv:      commitArgv,
		revertArgv:      revertArgv,
		invariantTarget: *invariant,
	}

	os.Exit(convergeRun(cfg, *truncate))

	return nil
}

// convergeRun converges the slot (spec 9.3) and returns the process exit
// code: 0 converged (or already terminal, cleared with --truncate), 3 the
// lock is held by a live transaction (no mutation), 1 convergence failed (the
// log is left untouched).
func convergeRun(cfg *startConfig, truncate bool) int {
	if err := ensureSlotDir(cfg.dir); err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	lock, err := openLockedLog(cfg.dir)
	if errors.Is(err, ErrSlotLocked) {
		return guard.ConvergeExitLocked // a live transaction owns the slot: no mutation
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	defer func() {
		// Ignorable on the converge path: the OFD lock releases when this fd
		// closes (verified V4), a Close error cannot un-release it, and the
		// process exits through the caller's os.Exit right after.
		_ = lock.Close()
	}()

	logw, err := OpenLogWriter(cfg.dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	defer logw.Close()

	var (
		seq uint64
		mu  sync.Mutex
	)

	folded, err := guard.ScanFile(slotLogPath(cfg.dir), cfg.key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	if folded.Found {
		seq = folded.Last.Seq // writers continue the sequence, never restart (spec 7)
	}

	// One context resolution for both shapes (spec 9.3): the transaction's
	// own records supply identity and step lists when the log carries them;
	// the no-log case falls to the argv hints and the generation arithmetic
	// alone (spec 9.3 step 6).
	cfg = convergeContext(cfg, folded)

	core := convergeCore(cfg, logw, &mu, &seq)

	if !folded.Found {
		// No records: the caller invoked converge because its generation
		// arithmetic said the profile advanced without activation evidence,
		// so the action is revert (spec 9.3 step 6).
		code := convergeRevertOutcome(core, cfg, "no-log convergence")
		if truncate && code == guard.ConvergeExitConverged {
			return clearSlotCode(cfg.dir, logw)
		}

		return code
	}

	code := convergeFolded(cfg, core, folded)
	if truncate && code == guard.ConvergeExitConverged {
		return clearSlotCode(cfg.dir, logw)
	}

	return code
}

// clearSlotCode truncates the log in place, removes the gc-root and removes
// the persisted guardian binary (spec 10.3); a clear failure fails the
// converge (the classification already landed, but the slot is not clean).
func clearSlotCode(dir string, logw *LogWriter) int {
	err := logw.TruncateInPlace()
	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	err = removeGCRoot(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	err = os.Remove(guardianBinaryPath(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "panix-guard: %v\n", err)

		return guard.ConvergeExitFailed
	}

	return guard.ConvergeExitConverged
}

// convergeCore builds the offline convergence core (WriterDecide) for one
// slot with the caller's shared sequence counter.
func convergeCore(cfg *startConfig, logw *LogWriter, mu *sync.Mutex, seq *uint64) *txCore {
	return &txCore{
		profilePath:     cfg.profilePath,
		nixEnv:          cfg.nixEnv,
		oldClosure:      cfg.oldClosure,
		newClosure:      cfg.newClosure,
		gen:             cfg.gen,
		commitArgv:      cfg.commitArgv,
		revertArgv:      cfg.revertArgv,
		invariantTarget: cfg.invariantTarget,
		key:             cfg.key,
		writer:          guard.WriterDecide,
		logw:            logw,
		seqMu:           mu,
		seq:             seq,
	}
}

// convergeContext resolves one folded tail into the convergence context (spec
// 9.3): the transaction's own identity comes from its records (old/new/gen
// beat the argv hints), its step lists prefer the TXN embedding over the
// argv-supplied fallback lists (the mode-drift fix, spec 7), and the
// generation arithmetic fills an unresolvable OLD from the profile's own
// generation links.
func convergeContext(base *startConfig, folded guard.Folded) *startConfig {
	cfg := *base

	if folded.Key != "" {
		cfg.key = folded.Key
	}

	if folded.Old != "" {
		cfg.oldClosure = folded.Old
	}

	if folded.New != "" {
		cfg.newClosure = folded.New
	}

	if folded.Gen > 0 {
		cfg.gen = folded.Gen
	}

	// Profile identity (argv-then-fold precedence): the caller's argv fills
	// the context first and the transaction's own records override when they
	// name the profile, because the fold is the transaction's truth. The
	// deployer's post-mortem argv carries --profile/--nix-env as the fallback
	// half, and start's internal sweep supplies its own flags; the records
	// carry no nix-env, so that value stands from the argv alone. Slots are
	// per-profile (spec 10.1), so a fold override equals the start argv's
	// value and the sweep behaves exactly as before.
	if folded.ProfilePath != "" {
		cfg.profilePath = folded.ProfilePath
	}

	// TXN embedding first (nil only when the tail carries no usable TXN
	// record: an empty [] list is a real empty step list, e.g. test mode).
	if folded.TxnCommit != nil {
		cfg.commitArgv = folded.TxnCommit
	}

	if folded.TxnRevert != nil {
		cfg.revertArgv = folded.TxnRevert
	}

	if folded.TxnInvariant != "" {
		cfg.invariantTarget = folded.TxnInvariant
	}

	// Generation arithmetic fallback (spec 9.3 step 6): when no record and no
	// argv hint names the rollback target, the previous generation link is
	// the rollback closure and its number becomes the deletion threshold.
	if cfg.oldClosure == "" && cfg.profilePath != "" {
		if old, gen, ok := previousGenClosure(cfg.profilePath); ok {
			cfg.oldClosure = old
			cfg.gen = gen
		}
	}

	return &cfg
}

// previousGenClosure resolves the profile's previous generation link target
// (generation arithmetic, spec 9.3 step 6): the highest generation link below
// the profile's current generation. ok is false when the profile has no
// resolvable previous generation.
func previousGenClosure(profile string) (string, int64, bool) {
	_, curGen, err := guard.ProfileTarget(profile)
	if err != nil || curGen <= 1 {
		return "", 0, false
	}

	dir := filepath.Dir(profile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, false
	}

	bestGen, bestName := int64(0), ""

	for _, e := range entries {
		g, ok := guard.ParseGenLink(e.Name())
		if ok && g < curGen && g > bestGen {
			bestGen, bestName = g, e.Name()
		}
	}

	if bestGen == 0 {
		return "", 0, false
	}

	target, _, err := guard.ProfileTarget(filepath.Join(dir, bestName))
	if err != nil {
		return "", 0, false
	}

	return target, bestGen, true
}

// convergeFolded drives one classified tail to a terminal state (spec 9.3):
// the shared convergence core of converge and start's pre-start sweep.
func convergeFolded(cfg *startConfig, core *txCore, folded guard.Folded) int {
	switch folded.Last.Ev {
	case guard.EventCommitted, guard.EventReverted, guard.EventRevertFailed, guard.EventFailedPrecondition, guard.EventExit:
		return guard.ConvergeExitConverged // terminal: no-op (the sweep clears the slot)
	case guard.EventCommitStart:
		// Partial commit: re-run the commit (idempotent, spec 9.3); a failing commit
		// converges by revert.
		if core.convergeCommit() {
			core.emit(guard.EventCommitted, guard.Record{St: string(stateCommitted)})

			return guard.ConvergeExitConverged
		}

		return convergeRevertOutcome(core, cfg, "post-mortem convergence")
	case guard.EventRevertStart:
		// Finish the interrupted revert: the steps re-run idempotently (spec 9.3).
		return convergeRevertOutcome(core, cfg, "post-mortem convergence")
	}

	// HELLO/STATE/CONFIRM-* tails: the guardian died mid-flight. Kill the logged
	// orphan child first (spec 9.3): an orphaned activation holds the STC global lock
	// and the guardian's timeout died with it.
	if folded.ChildPID > 0 {
		if _, alive := processCmdline(folded.ChildPID); alive {
			_ = syscall.Kill(-folded.ChildPID, syscall.SIGKILL)
			core.emit(guard.EventOrphanKilled, guard.Record{CPID: folded.ChildPID})
		}
	}

	return convergeRevertOutcome(core, cfg, "post-mortem convergence")
}

// convergeRevertOutcome runs the revert convergence and writes the terminal
// records (spec 9.3). Error fidelity (spec 7): a failed post-mortem revert
// names its own failing step in the terminal record, never the stale
// classification reason.
func convergeRevertOutcome(core *txCore, cfg *startConfig, reason string) int {
	core.emit(guard.EventRevertStart, guard.Record{Rs: reason})

	if core.convergeRevert() && core.profileIs(cfg.oldClosure) {
		core.emit(guard.EventReverted, guard.Record{St: string(stateReverted)})

		_ = removeGCRoot(cfg.dir)

		return guard.ConvergeExitConverged
	}

	core.emit(guard.EventRevertFailed, guard.Record{
		St:  string(stateRevertFailed),
		Err: core.stepFailure(),
	})

	_ = removeGCRoot(cfg.dir)

	return guard.ConvergeExitFailed
}
