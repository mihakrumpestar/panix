package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// start = relay (spec 4.1 step 5, 6.1, 9.1): one exec acquires the transaction
// lock, performs every pre-start mutation under it (sweep convergence, gc
// root, boot-mode profile set, OLD capture), spawns the detached guardian with
// the duplex pipes, then BECOMES the relay: panix stdin feeds the command
// pipe, the event pipe streams records and child output to panix stdout.

// startRelay runs the parent side of the spawn exec. It returns only on
// failure paths; on success it never returns (os.Exit from the relay after
// the guardian's event pipe closes).
func startRelay(cfg *startConfig) error {
	if err := ensureSlotDir(cfg.dir); err != nil {
		return err
	}

	lock, err := openLockedLog(cfg.dir)
	if err != nil {
		// The lock refusal keeps today's verdict and exit code (spec 4.1
		// step 5: failure = live transaction, zero effects).
		return err
	}
	defer lock.Close()

	mut, err := newStartMutator(cfg, lock)
	if err != nil {
		return err
	}
	defer mut.logw.Close()

	if err := runStartMutations(mut); err != nil {
		return err
	}

	// (e) Spawn the detached guardian with the duplex pipes (two os.Pipe
	// pairs, NOT a socketpair: the relay copies independently in each
	// direction) and become the relay. Never returns: the relay exits with
	// the classified outcome.
	os.Exit(mut.spawnAndRelay())

	return nil
}

// runStartMutations performs every pre-start mutation under start's own
// flock, in the pinned order (spec 4.1 step 5): sweep convergence and
// truncation, gc root, OLD capture, boot-mode profile set. Any failure writes
// the FAILED_PRECONDITION record and returns the error that exits nonzero.
func runStartMutations(mut *startMutator) error {
	cfg := mut.cfg

	// (a) Pre-start sweep: converge a non-terminal previous transaction from
	// its own records, then truncate the log in place (spec 9.3).
	if cfg.sweep {
		err := mut.sweep()
		if err != nil {
			return mut.failPrecondition("pre-start sweep failed: " + err.Error())
		}
	}

	// (b) GC root creation (spec 4.3): nix-store --add-root with the --realise
	// form verified in the v1.3 e2e.
	if cfg.gcRootTarget != "" {
		err := mut.gcRoot()
		if err != nil {
			return mut.failPrecondition("gc root creation failed: " + err.Error())
		}
	}

	// (c) OLD capture BEFORE the boot-mode set (see captureOld): after --set
	// NEW the profile reads NEW, which would make the boot-mode rollback
	// target the broken generation its own revert table must restore away.
	if cfg.profilePath != "" {
		err := mut.captureOld()
		if err != nil {
			return mut.failPrecondition("old capture failed: " + err.Error())
		}
	}

	// (d) Boot-mode pre-start profile set (spec 5: the previously
	// unimplemented mutation, now under start's lock).
	if cfg.bootSet != "" {
		err := mut.bootSet()
		if err != nil {
			return mut.failPrecondition("boot-mode profile set failed: " + err.Error())
		}
	}

	return nil
}

// startMutator carries the mutation state of the start phase: the transaction
// config (OLD lands here during the capture) and the record writer whose
// sequence continues from the current log tail.
type startMutator struct {
	cfg  *startConfig
	lock *slotLock
	logw *LogWriter
	core *txCore
}

// newStartMutator opens the start-phase record writer with a sequence seeded
// from the log tail (spec 7: one counter continues across writers).
func newStartMutator(cfg *startConfig, lock *slotLock) (*startMutator, error) {
	logw, err := OpenLogWriter(cfg.dir)
	if err != nil {
		return nil, err
	}

	var seq uint64

	if folded, err := guard.ScanFile(filepath.Join(cfg.dir, logName), ""); err == nil && folded.Found {
		seq = folded.Last.Seq
	}

	mut := &startMutator{cfg: cfg, lock: lock, logw: logw}
	mut.core = &txCore{
		profilePath: cfg.profilePath,
		nixEnv:      cfg.nixEnv,
		newClosure:  cfg.newClosure,
		gen:         cfg.gen,
		key:         cfg.key,
		writer:      guard.WriterGuardian,
		logw:        logw,
		seqMu:       &sync.Mutex{},
		seq:         &seq,
	}

	return mut, nil
}

// failPrecondition writes the FAILED_PRECONDITION record (log is the
// transaction's existence proof) and returns the error that exits nonzero;
// the lock releases when the process exits (spec 6.1).
func (m *startMutator) failPrecondition(reason string) error {
	m.core.emit(guard.EventFailedPrecondition, guard.Record{
		St:  string(stateFailedPrecondition),
		RC:  1,
		Err: guard.BoundExcerpt(reason),
	})

	return fmt.Errorf("%w: %s", errStartMutation, reason)
}

// sweep classifies the slot's log tail and converges a non-terminal previous
// transaction (spec 9.3), then truncates the log in place. A terminal slot is
// just truncated.
func (m *startMutator) sweep() error {
	path := filepath.Join(m.cfg.dir, logName)

	folded, err := guard.ScanFile(path, "")
	if err != nil {
		return fmt.Errorf("%w: %w", errTailScan, err)
	}

	if folded.Found && !folded.Terminal {
		if code := m.converge(folded); code != guard.ConvergeExitConverged {
			return fmt.Errorf("%w (exit %d); the log is preserved for post-mortem", errSweepNotConverged, code)
		}
	}

	// In-place truncation through the writer on the same inode: same lock
	// OFD semantics, rename-based truncation is prohibited (spec 7). The
	// sequence counter resets with the log: the fresh transaction starts at
	// seq 1 exactly like a guardian spawned onto a clean slot. The stale
	// gc-root goes with it (the gc-root step re-creates it for the new
	// transaction); the persisted guardian binary stays: the deployer
	// transferred the fresh one into the slot before this exec (spec 4.1
	// step 3), so the binary here IS the one the detached spawn needs
	// (os.Executable re-exec) and the post-terminal ctl/attach execs run the
	// slot binary too. Removing it would break both; the previous deploy's
	// binary is already gone (the transfer's atomic mv replaced its directory
	// entry, which is the 10.3 cleanup at the next pre-start), and the
	// standalone converge --truncate clear removes the binary when a slot is
	// cleared outside a deploy (spec 9.3, 10.3).
	if err := m.logw.TruncateInPlace(); err != nil {
		return err
	}

	if err := removeGCRoot(m.cfg.dir); err != nil {
		return err
	}

	m.core.seqMu.Lock()
	*m.core.seq = 0
	m.core.seqMu.Unlock()

	return nil
}

// converge runs the decide classification core against the folded tail with
// the previous transaction's own context (spec 9.3): its records supply
// old/new/gen, its TXN embedding supplies the step lists, the argv fallback
// lists come second, and the generation arithmetic fills what remains.
func (m *startMutator) converge(folded guard.Folded) int {
	cfg := convergeContext(m.cfg, folded)

	logw := m.logw // shared writer; the sequence continues across the two cores

	core := &txCore{
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
		seqMu:           m.core.seqMu,
		seq:             m.core.seq,
	}

	return convergeFolded(cfg, core, folded)
}

// gcRoot creates the slot's indirect GC root (spec 4.3).
func (m *startMutator) gcRoot() error {
	step := []string{
		m.cfg.nixStore, "--add-root", filepath.Join(m.cfg.dir, gcRootName),
		"--indirect", "--realise", m.cfg.gcRootTarget,
	}

	return runMutationStep(step, m.logw)
}

// bootSet performs the boot-mode pre-start profile set (spec 5): idempotent,
// and the guardian's precondition re-check validates it ran.
func (m *startMutator) bootSet() error {
	step := []string{m.cfg.nixEnv, "-p", m.cfg.profilePath, "--set", m.cfg.bootSet}

	return runMutationStep(step, m.logw)
}

// captureOld resolves the profile chain into the rollback target and its
// generation (spec 4.1 step 5: readlink -f plus the generation number). The
// captured values replace the argv hints: the guardian's precondition and the
// revert's runtime rules must validate against the same post-converge
// profile.
func (m *startMutator) captureOld() error {
	target, gen, err := guard.ProfileTarget(m.cfg.profilePath)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", m.cfg.profilePath, err)
	}

	m.cfg.oldClosure = target
	m.cfg.gen = gen

	return nil
}

// spawnAndRelay builds the guardian argv (with the captured OLD), spawns the
// detached guardian over the two pipe pairs, and becomes the relay. It
// returns the process exit code (the caller exits with it): the relay's own
// classification, or the cancelled shape on a spawn failure.
func (m *startMutator) spawnAndRelay() int {
	self, err := os.Executable()
	if err != nil {
		// Without a resolvable self the spawn cannot happen: classify from
		// the tail (empty tail: the cancelled shape; the deployer's inline
		// converge reports the failed-precondition-shaped outcome).
		return classifyTail(m.cfg)
	}

	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		return classifyTail(m.cfg)
	}

	evtR, evtW, err := os.Pipe()
	if err != nil {
		_ = cmdR.Close()
		_ = cmdW.Close()

		return classifyTail(m.cfg)
	}

	childArgs := startArgv(m.cfg)

	childEnv := append(os.Environ(), detachedEnv+"=1")

	if _, err := spawnDetached(self, childArgs, childEnv, m.lock.file, guardianFDs{
		lock: m.lock.file,
		cmd:  cmdR,
		evt:  evtW,
	}); err != nil {
		// Spawn failure (spec 6.1): the relay sees the event-pipe EOF,
		// classifies from the tail once, releases the lock reference by
		// exiting nonzero.
		_ = cmdR.Close()
		_ = cmdW.Close()
		_ = evtR.Close()
		_ = evtW.Close()

		return classifyTail(m.cfg)
	}

	// The guardian holds its inherited ends; the relay keeps cmdW (stdin
	// feed) and evtR (stream source).
	_ = cmdR.Close()
	_ = evtW.Close()

	_ = m.logw.Close()

	return relayLoop(evtR, cmdW, os.Stdin, m.cfg)
}

// relayLoop is the live window (spec 6.1 step 3, 9.1): stdin feeds the
// command pipe, the event pipe streams to stdout. Stdin EOF closes the
// command pipe (the guardian logs LINK_DOWN and continues degraded); the
// event side drains to EOF and then classifies the outcome from the log tail
// exactly once.
func relayLoop(evtR, cmdW *os.File, stdin io.Reader, cfg *startConfig) int {
	go func() {
		_, _ = io.Copy(cmdW, stdin)
		_ = cmdW.Close()
	}()

	drainEvents(evtR, os.Stdout)
	_ = evtR.Close()

	return classifyTail(cfg)
}

// drainEvents copies the event pipe to stdout without ever stalling the
// transaction (spec 9.1): a slow or dead stdout never stops the drain, write
// errors drop the chunk and the reads continue to EOF.
func drainEvents(r *os.File, w io.Writer) {
	buf := make([]byte, 32<<10)

	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}

		if rerr != nil {
			return
		}
	}
}

// classifyTail maps the log tail to the viewer exit codes exactly once
// (spec 9.1, EOF != death): a terminal record for the current (recovered)
// key maps to its outcome code; anything else is cancelled (6), which sends
// panix down its inline-converge path. The relay NEVER converges and NEVER
// converges on a timer.
func classifyTail(cfg *startConfig) int {
	folded, err := guard.ScanFile(filepath.Join(cfg.dir, logName), "")
	if err != nil || !folded.Found || !folded.Terminal {
		return guard.ExitCancelled
	}

	return exitCodeFor(txState(folded.Status))
}

// runMutationStep runs one start-phase mutation step with its output streamed
// into the log (the guardian does not exist yet, so the log is the only
// transcript).
func runMutationStep(step []string, logw *LogWriter) error {
	out, err := execStep(step)
	for _, line := range splitLines(string(out)) {
		logw.WriteLine(line)
	}

	if err != nil {
		return fmt.Errorf("%w: %s (rc=%d)", errMutationStep, step[0], exitCodeOf(err))
	}

	return nil
}
