package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// Tier classes (spec 2): the transaction logic branches on these, while the concrete
// activation/commit/revert commands arrive as panix-composed argv lists (spec 6.5).
const (
	tierFull        = "full"
	tierStandard    = "standard"
	tierSelfSetting = "self-setting"
	tierMinimal     = "minimal"
)

// Confirmation gates (the user-facing rollback: auto|magic values, spec 6.2).
const (
	gateAuto  = "auto"
	gateMagic = "magic"
)

// Transaction states (spec 6.2). Starred states are terminal.
type txState string

const (
	statePre                txState = "pre"
	stateActivating         txState = "activating"
	stateChecking           txState = "checking"
	stateActivated          txState = "activated"
	stateCommitting         txState = "committing"
	stateCommitted          txState = "committed" // terminal
	stateReverting          txState = "reverting"
	stateReverted           txState = "reverted"            // terminal
	stateRevertFailed       txState = "revert_failed"       // terminal
	stateFailedPrecondition txState = "failed_precondition" // terminal
	stateActivationExited   txState = "activation_exited"   // terminal, minimal tier
)

// Timing constants (spec 6.2, 2).
const (
	heartbeatInterval = 5 * time.Second
	commitBudgetFloor = 120 * time.Second
	deadlineGrace     = 30 * time.Second
	revertRetries     = 2
	revertBackoff     = 2 * time.Second
	checkTimeout      = 30 * time.Second
	killWaitBound     = 10 * time.Second
	conservativePath  = "/run/current-system/sw/bin:/usr/bin:/bin"
)

// txSignal is the normalized request input to the state machine (spec 8):
// a confirm or a revert request, arriving from either transport (wire frames
// or signals) with the same transitions.
type txSignal int

const (
	sigConfirm txSignal = iota
	sigRevertRequest
)

// txInput is one queued request: the normalized signal plus the wire
// request id (0 for signal-driven requests, whose records carry no rid).
type txInput struct {
	sig txSignal
	rid int64
}

type transaction struct {
	cfg  *startConfig
	logw *LogWriter
	lock *os.File
	core *txCore
	wire *wireWriter // nil when the event pipe is absent (degraded mode)

	mu              sync.Mutex
	seq             uint64
	st              txState
	childPID        int
	lastChildLine   string
	pendingConfirm  bool
	revertRequested bool
	confirmRid      int64 // pending wire confirm rid (0 = none/signal-driven)
	revertRid       int64 // pending wire revert rid (0 = none/signal-driven)

	baseline   map[string]bool
	baselineOK bool

	sigCh chan txInput
	done  chan struct{} // closed once at terminal; stops the heartbeat
}

func newTransaction(cfg *startConfig, logw *LogWriter, lock *os.File, wire *wireWriter, cmdIn io.Reader) *transaction {
	t := &transaction{
		cfg:   cfg,
		logw:  logw,
		lock:  lock,
		wire:  wire,
		st:    statePre,
		sigCh: make(chan txInput, 8),
		done:  make(chan struct{}),
	}
	t.core = &txCore{
		profilePath:     cfg.profilePath,
		nixEnv:          cfg.nixEnv,
		oldClosure:      cfg.oldClosure,
		newClosure:      cfg.newClosure,
		gen:             cfg.gen,
		mode:            cfg.mode,
		commitArgv:      cfg.commitArgv,
		revertArgv:      cfg.revertArgv,
		invariantTarget: cfg.invariantTarget,
		key:             cfg.key,
		writer:          guard.WriterGuardian,
		logw:            logw,
		wire:            wire,
		onLine:          t.recordChildLine,
		seqMu:           &t.mu,
		seq:             &t.seq,
	}

	if cmdIn != nil {
		// The command pipe drains from before HELLO (spec 6.1): frames that
		// arrive before the state machine can act queue in sigCh with the
		// same pre-HELLO semantics as signals.
		go t.readCommands(cmdIn)
	}

	return t
}

// readCommands drains the command pipe for the transaction's whole life:
// valid frames join the signal queue in arrival order, unknown or malformed
// lines get an ack-shaped record (no rid: never an ack) and the drain keeps
// going, and a terminal read failure logs LINK_DOWN (log only, spec 6.1).
func (t *transaction) readCommands(r io.Reader) {
	err := readWireCommands(r,
		func(command string, rid int64) {
			sig := sigRevertRequest
			if command == "confirm" {
				sig = sigConfirm
			}

			t.setWireRid(sig, rid)

			t.sigCh <- txInput{sig: sig, rid: rid}
		},
		func(reason string) {
			t.core.emit(guard.EventLateRequestAck, guard.Record{Rs: reason})
		})
	if err != nil {
		t.core.emitLogOnly(guard.EventLinkDown, guard.Record{Rs: "command pipe failed: " + err.Error()})

		return
	}

	t.core.emitLogOnly(guard.EventLinkDown, guard.Record{Rs: "command pipe EOF"})
}

// setWireRid records the pending wire rid for one request kind so the ack
// records can carry it (spec 8: re-delivery is idempotent, the newest wins).
func (t *transaction) setWireRid(sig txSignal, rid int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if sig == sigConfirm {
		t.confirmRid = rid
	} else {
		t.revertRid = rid
	}
}

// takeRid returns and clears the pending wire rid for one request kind.
func (t *transaction) takeRid(sig txSignal) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if sig == sigConfirm {
		rid := t.confirmRid
		t.confirmRid = 0

		return rid
	}

	rid := t.revertRid
	t.revertRid = 0

	return rid
}

// run executes the whole transaction (spec 6.2) and returns the terminal state. The
// transaction holds the inherited lock for its whole life; the lock releases when this
// process exits.
func (t *transaction) run() txState {
	raw := make(chan os.Signal, 8)

	signal.Notify(raw, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM, syscall.SIGINT)

	defer signal.Stop(raw)
	go func() {
		for s := range raw {
			// Signals are the degraded transport: no wire rid (spec 8).
			switch s {
			case syscall.SIGUSR1:
				t.sigCh <- txInput{sig: sigConfirm}
			default:
				t.sigCh <- txInput{sig: sigRevertRequest}
			}
		}
	}()

	go t.heartbeat()
	defer close(t.done) // single close: stops the heartbeat (spec 6.2)

	t.emit(guard.EventHello, guard.Record{
		PID:  os.Getpid(),
		Old:  t.cfg.oldClosure,
		New:  t.cfg.newClosure,
		Gen:  t.cfg.gen,
		Mode: t.cfg.mode,
		Tier: t.cfg.tier,
		Pf:   t.cfg.profilePath,
		DL:   t.deadline().Unix(),
	})

	if !t.emitTxn() {
		// The embedded step lists cannot fit the record budget: the minimal
		// FAILED_PRECONDITION is the transaction's existence proof, and no
		// mutation runs (spec 7).
		return t.terminal(stateFailedPrecondition, 0, "transaction step lists exceed the TXN record budget")
	}

	t.emitState()

	// PRE: precondition and baseline (spec 6.2).
	if !t.checkPrecondition() {
		return t.terminal(stateFailedPrecondition, 0, "profile state does not match the pre-start expectation")
	}

	t.captureBaseline()

	// ACTIVATING.
	rc, aborted, excerpt := t.phaseActivate()
	if t.cfg.tier == tierMinimal {
		// Minimal tier has no commit and no revert: the activation outcome is terminal
		// either way (spec 6.2, minimal state machine).
		rcOut := rc
		if aborted {
			rcOut = -1
		}

		return t.terminal(stateActivationExited, rcOut, excerpt)
	}

	if aborted {
		return t.phaseRevert("revert requested during activation", excerpt)
	}

	if rc != 0 {
		reason := fmt.Sprintf("activation failed (rc=%d)", rc)
		if rc < 0 {
			reason = excerpt
		}

		return t.phaseRevert(reason, excerpt)
	}

	// CHECKING (local checks, spec 6.2).
	pass, why := t.phaseChecking()
	if !pass {
		return t.phaseRevert(why, t.lastChildErr())
	}

	// A revert request that arrived during checks or right at the gate (spec 6.2:
	// CHECKING + revert-request -> REVERTING) is honored before any commit.
	if t.pollSignals() {
		return t.phaseRevert("revert requested", "")
	}

	// ACTIVATED (spec 6.2): the confirmation gate and panix's record-driven confirm
	// both depend on this state being visible in the log, so emit it unconditionally.
	t.setState(stateActivated)
	t.emitState()

	// The confirmation gate (magic only, spec 6.2): no confirmation within the window
	// means revert. A confirm that arrived while activating is consumed immediately.
	if t.cfg.confirmation == gateMagic && t.cfg.tier != tierMinimal {
		if !t.awaitConfirm() {
			return t.phaseRevert("no confirmation within the window", "")
		}
	}

	// Commit (may be empty: nixos test, self-setting; spec 4.2).
	if len(t.cfg.commitArgv) > 0 {
		if !t.phaseCommit() {
			return t.phaseRevert("commit failed", t.lastChildErr())
		}
	}

	if !t.checkInvariant() {
		return t.phaseRevert("post-transaction invariant violated", "")
	}

	if t.cfg.tier == tierMinimal {
		return t.terminal(stateActivationExited, rc, "")
	}

	return t.terminal(stateCommitted, 0, "")
}

// deadline is the internal overall bound (spec 6.2): the sum of the phase bounds plus a
// grace. Phase timers inside the machine enforce each phase independently.
func (t *transaction) deadline() time.Time {
	commitBudget := max(t.cfg.activationTimeout/2, commitBudgetFloor)

	total := t.cfg.activationTimeout + t.cfg.confirmTimeout + commitBudget + deadlineGrace

	return time.Now().Add(total)
}

// heartbeat re-emits the full state snapshot every 5s (spec 7); it runs for the whole
// transaction so every phase is covered, including non-interruptible ones.
func (t *transaction) heartbeat() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			t.emitState()
		}
	}
}

// emit writes one record for the transaction (the shared core emitter: the
// writer tag, sequence counter and timestamp live there, spec 7).
func (t *transaction) emit(ev string, extra guard.Record) { t.core.emit(ev, extra) }

func (t *transaction) emitState() {
	t.mu.Lock()
	st, childPID := t.st, t.childPID
	t.mu.Unlock()
	t.emit(guard.EventState, guard.Record{
		St:   string(st),
		PID:  os.Getpid(),
		CPID: childPID,
		Old:  t.cfg.oldClosure,
		New:  t.cfg.newClosure,
		Gen:  t.cfg.gen,
		Mode: t.cfg.mode,
		Tier: t.cfg.tier,
	})
}

func (t *transaction) emitStateWithStatus(status string) {
	t.emit(guard.EventState, guard.Record{St: status, PID: os.Getpid()})
}

// emitTxn writes the TXN record right after HELLO (spec 6.5, 7): the
// transaction's own activation/commit/revert step lists and invariant target,
// embedded verbatim so post-mortem consumers never recompose them. Returns
// false when the embedding exceeds the TXN budget (caller aborts).
func (t *transaction) emitTxn() bool {
	t.mu.Lock()
	seq := t.seq + 1
	t.mu.Unlock()

	record := guard.Record{
		K:   t.cfg.key,
		W:   guard.WriterGuardian,
		Seq: seq,
		TS:  time.Now().Unix(),
		AA:  mustJSON(t.cfg.activationArgv),
		CA:  mustJSON(t.cfg.commitArgv),
		RA:  mustJSON(t.cfg.revertArgv),
		IV:  t.cfg.invariantTarget,
	}

	if guard.RecordLineBytes(record) > guard.TxnMaxLineBytes {
		return false
	}

	t.emit(guard.EventTxn, guard.Record{AA: record.AA, CA: record.CA, RA: record.RA, IV: record.IV})

	return true
}

func (t *transaction) setState(s txState) {
	t.mu.Lock()
	t.st = s
	t.mu.Unlock()
}

func (t *transaction) getState() txState {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.st
}

func (t *transaction) setChildPID(pid int) {
	t.mu.Lock()
	t.childPID = pid
	t.mu.Unlock()
}

func (t *transaction) recordChildLine(line string) {
	t.mu.Lock()
	t.lastChildLine = line
	t.mu.Unlock()
}

func (t *transaction) lastChildErr() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.lastChildLine
}

// checkPrecondition verifies the profile still points where the pre-start expected it
// (spec 6.2): boot mode set it to NEW at pre-start, everything else left it on OLD.
// An empty profile path (minimal tier) skips the check.
func (t *transaction) checkPrecondition() bool {
	if t.cfg.profilePath == "" {
		return true
	}

	want := t.cfg.oldClosure
	if t.cfg.mode == "boot" {
		want = t.cfg.newClosure
	}

	got, _, err := guard.ProfileTarget(t.cfg.profilePath)
	if err != nil {
		t.core.writeLine(fmt.Sprintf("guard: precondition probe failed: %v", err))

		return false
	}

	if got != want {
		t.core.writeLine(fmt.Sprintf("guard: precondition mismatch: profile is %s, expected %s", got, want))

		return false
	}

	return true
}

// checkInvariant verifies the profile resolves to the expected target after the
// transaction's effects (spec 6.5, delegated to the shared core).
func (t *transaction) checkInvariant() bool { return t.core.invariantOK() }

// captureBaseline snapshots the failed systemd units before activation (NixOS only,
// spec 6.2). When systemctl is unavailable the builtin check disables itself.
func (t *transaction) captureBaseline() {
	if !t.cfg.builtinUnitCheck {
		return
	}

	units, ok := captureFailedUnits(t.logw)
	if !ok {
		t.core.writeLine("guard: builtin unit check disabled (systemctl unavailable)")

		return
	}

	t.mu.Lock()
	t.baseline, t.baselineOK = units, true
	t.mu.Unlock()
}

// builtinUnitDelta fails when units failed during activation that were not failed before
// (spec 6.2, builtin check). A disabled or unprobeable check passes with a log note.
func (t *transaction) builtinUnitDelta() (bool, string) {
	t.mu.Lock()
	baseline, ok := t.baseline, t.baselineOK
	t.mu.Unlock()

	if !ok {
		return true, ""
	}

	cur, probeOK := captureFailedUnits(t.logw)
	if !probeOK {
		t.core.writeLine("guard: builtin unit check skipped (systemctl unavailable)")

		return true, ""
	}

	var fresh []string

	for u := range cur {
		if !baseline[u] {
			fresh = append(fresh, u)
		}
	}

	if len(fresh) > 0 {
		sort.Strings(fresh)

		return false, "new failed units: " + strings.Join(fresh, ", ")
	}

	return true, ""
}

// phaseChecking runs the local checks (CHECKING state, spec 6.2): the builtin failed
// units delta plus the user's health_checks_local commands. Signals are polled between
// checks; a revert request aborts the remaining checks (spec 6.2: CHECKING +
// revert-request -> REVERTING).
func (t *transaction) phaseChecking() (bool, string) {
	if len(t.cfg.healthChecksLocal) == 0 && !t.cfg.builtinUnitCheck {
		return true, ""
	}

	t.setState(stateChecking)
	t.emitState()

	if ok, why := t.builtinUnitDelta(); !ok {
		return false, why
	}

	for _, command := range t.cfg.healthChecksLocal {
		if t.pollSignals() {
			return false, "revert requested"
		}

		ok, why := runLocalCheck(command, t.core.writeLine)
		if !ok {
			return false, why
		}
	}

	return true, ""
}

// pollSignals drains the request queue without blocking. Returns true when a
// revert request is pending (caller reverts); a confirm is recorded as pending
// and consumed at ACTIVATED (spec 6.2, 8).
func (t *transaction) pollSignals() bool {
	for {
		select {
		case in := <-t.sigCh:
			switch in.sig {
			case sigConfirm:
				t.mu.Lock()
				t.pendingConfirm = true
				t.mu.Unlock()
			case sigRevertRequest:
				t.mu.Lock()
				t.revertRequested = true
				t.mu.Unlock()

				return true
			}
		default:
			return false
		}
	}
}

// phaseActivate runs the activation child (spec 6.4) under the activation timeout and
// the signal rules: a confirm becomes pending, a revert request or the deadline kills
// the child and aborts.
func (t *transaction) phaseActivate() (rc int, aborted bool, excerpt string) {
	t.setState(stateActivating)
	t.emitState()

	if len(t.cfg.activationArgv) != 1 || len(t.cfg.activationArgv[0]) == 0 {
		return -1, false, "activation argv is invalid"
	}

	argv := t.cfg.activationArgv[0]
	// The activation child argv is panix-composed by contract (spec 6.5).
	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // panix-composed argv is the contract (spec 6.5)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = envWithPath(os.Environ(), conservativePath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, false, fmt.Sprintf("activation stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, false, fmt.Sprintf("activation stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return -1, false, fmt.Sprintf("activation start: %v", err)
	}

	t.setChildPID(cmd.Process.Pid)

	var wg sync.WaitGroup

	wg.Add(2)
	go t.drain(&wg, stdout)
	go t.drain(&wg, stderr)

	done := make(chan int, 1)

	go func() {
		err := cmd.Wait()

		rc := 0
		if err != nil {
			rc = exitCodeOf(err)
		}

		wg.Wait() // drain everything the child wrote before reaping (spec 6.4)

		done <- rc
	}()

	activationTimer := time.NewTimer(t.cfg.activationTimeout)
	defer activationTimer.Stop()

	for {
		select {
		case rcv := <-done:
			if rcv == 0 {
				return 0, false, ""
			}

			return rcv, false, t.lastChildErr()
		case <-activationTimer.C:
			t.killChild()
			t.awaitChildExit(done)

			return -1, false, fmt.Sprintf("activation timed out after %s", t.cfg.activationTimeout)
		case in := <-t.sigCh:
			switch in.sig {
			case sigConfirm:
				t.mu.Lock()
				t.pendingConfirm = true
				t.mu.Unlock()
			case sigRevertRequest:
				t.killChild()
				t.awaitChildExit(done)

				return 0, true, t.lastChildErr()
			}
		}
	}
}

// awaitChildExit waits for the activation child to be reaped after a kill, bounded
// (spec 6.3 step 1). A child that will not die is left for converge to clean up.
func (t *transaction) awaitChildExit(done <-chan int) {
	select {
	case <-done:
	case <-time.After(killWaitBound):
		t.core.writeLine("guard: activation child did not exit after kill; orphan left for converge")
	}
}

// drain feeds child output lines into the serialized log writer and keeps the last
// non-empty line as the error excerpt source (spec 6.4, 7).
func (t *transaction) drain(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')

		trimmed := strings.TrimRight(line, "\n")
		if trimmed != "" {
			t.core.writeLine(trimmed)
			t.recordChildLine(trimmed)
		}

		if err != nil {
			return
		}
	}
}

// killChild SIGKILLs the whole child process group (spec 6.4).
func (t *transaction) killChild() {
	t.mu.Lock()
	pid := t.childPID
	t.mu.Unlock()

	if pid <= 0 {
		return
	}

	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// awaitConfirm is the magic-tier gate (spec 6.2): confirm => commit, revert request or
// window expiry => revert. A confirm that arrived while activating is consumed here
// without waiting.
func (t *transaction) awaitConfirm() bool {
	t.mu.Lock()
	pending := t.pendingConfirm
	t.mu.Unlock()

	if pending {
		t.emit(guard.EventConfirmConsumed, guard.Record{Rid: t.takeRid(sigConfirm)})

		return true
	}

	confirmTimer := time.NewTimer(t.cfg.confirmTimeout)
	defer confirmTimer.Stop()

	for {
		select {
		case in := <-t.sigCh:
			switch in.sig {
			case sigConfirm:
				t.emit(guard.EventConfirmConsumed, guard.Record{Rid: t.takeRid(sigConfirm)})

				return true
			case sigRevertRequest:
				t.emit(guard.EventRevertRequested, guard.Record{Rid: t.takeRid(sigRevertRequest)})

				return false
			}
		case <-confirmTimer.C:
			t.emit(guard.EventConfirmIgnored, guard.Record{Rid: t.takeRid(sigConfirm), Rs: "window expired"})

			return false
		}
	}
}

// phaseCommit runs the commit core (spec 4.2). Once COMMIT_START is written the commit
// is non-interruptible: signals are acked late (spec 6.2).
func (t *transaction) phaseCommit() bool {
	t.setState(stateCommitting)
	ok := t.core.convergeCommit()
	t.ackLateRequests()

	return ok
}

// phaseRevert runs the revert (spec 6.3): REVERT_START before any step, kill a live
// child first, restore the profile when it points at NEW (runtime-derived, spec 6.5),
// then the panix-composed revert steps with STC-lock backoff, then generation cleanup.
// phaseRevert runs the revert (spec 6.3): REVERT_START before any step, kill a live
// child first, then the shared convergence core (profile restore, panix-composed
// revert steps with STC-lock backoff, generation cleanup).
func (t *transaction) phaseRevert(reason, excerpt string) txState {
	t.setState(stateReverting)
	// A confirm that arrived but lost the race to a revert is discarded with a
	// record (spec 6.2: deadline wins over pending-confirm).
	t.discardPendingConfirm()
	t.emit(guard.EventRevertStart, guard.Record{Rid: t.takeRid(sigRevertRequest), Rs: reason, Err: excerpt})
	t.killChild()
	stepsOK := t.core.convergeRevert()
	t.ackLateRequests()

	return t.finishRevert(stepsOK, excerpt)
}

// finishRevert writes the terminal revert records and applies the optional gated reboot
// (spec 6.3): the reboot fires only when the revert steps failed but the profile is
// back on OLD, and never in boot mode. Error fidelity (spec 7): a failed revert
// names its own failing step's argv and error in the terminal record, never the
// activation error that triggered the revert (the pre-step restore counts as a
// step, so a restore failure is covered too).
func (t *transaction) finishRevert(stepsOK bool, excerpt string) txState {
	if stepsOK && t.core.profileIs(t.cfg.oldClosure) {
		return t.terminal(stateReverted, 0, excerpt)
	}

	if !stepsOK {
		if stepFailure := t.core.stepFailure(); stepFailure != "" {
			excerpt = stepFailure
		}
	}

	if t.cfg.rebootOnRevertFailure && t.cfg.mode != "boot" && t.core.profileIs(t.cfg.oldClosure) {
		t.core.writeLine("guard: revert steps failed; profile is restored; rebooting into the old generation (reboot_on_revert_failure)")
		t.emitStateWithStatus("reboot_requested")

		if reboot(t.logw) {
			// A successful reboot request ends this process; the lines below only run
			// when every reboot candidate failed.
			t.core.writeLine("guard: reboot request failed")
		}
	}

	return t.terminal(stateRevertFailed, 0, excerpt)
}

// terminal writes the terminal records, removes the gc-root and returns the state
// (spec 6.2: always exit 0, outcomes live in records).
func (t *transaction) terminal(st txState, rc int, excerpt string) txState {
	t.setState(st)

	if ev := terminalEvent(st); ev != "" {
		t.emit(ev, guard.Record{St: string(st), RC: rc, Err: excerpt})
	}

	t.emit(guard.EventExit, guard.Record{St: string(st), RC: rc})
	_ = removeGCRoot(t.cfg.dir)

	return st
}

func terminalEvent(st txState) string {
	switch st {
	case stateCommitted:
		return guard.EventCommitted
	case stateReverted:
		return guard.EventReverted
	case stateRevertFailed:
		return guard.EventRevertFailed
	case stateFailedPrecondition:
		return guard.EventFailedPrecondition
	default:
		return "" // activation_exited reports through EXIT only
	}
}

// ackLateRequests drains buffered requests after a non-interruptible phase and
// acks each (spec 6.2: late requests get LATE_REQUEST_ACK; wire requests carry
// their rid, signal-driven ones carry none).
func (t *transaction) ackLateRequests() {
	for {
		select {
		case in := <-t.sigCh:
			t.emit(guard.EventLateRequestAck, guard.Record{Rid: in.rid})
		default:
			return
		}
	}
}

// discardPendingConfirm emits CONFIRM_IGNORED when a confirm arrived but a revert won
// the race first (spec 6.2: deadline wins over pending-confirm).
func (t *transaction) discardPendingConfirm() {
	t.mu.Lock()
	pending := t.pendingConfirm
	t.pendingConfirm = false
	t.mu.Unlock()

	if pending {
		t.emit(guard.EventConfirmIgnored, guard.Record{Rid: t.takeRid(sigConfirm), Rs: "superseded by revert"})
	}
}

// exitCodeOf extracts the process exit code from a cmd.Wait error.
func exitCodeOf(err error) int {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}

	return -1
}
